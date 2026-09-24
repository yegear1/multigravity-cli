package chat

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var titleRegex = regexp.MustCompile(`title:\s*"([^"]*)"`)

// ListConversations prints a table of AI conversations in a profile
func ListConversations(profileName string) error {
	return ListConversationsWriter(os.Stdout, profileName)
}

// ListConversationsWriter prints a table of AI conversations to the provided writer
func ListConversationsWriter(w io.Writer, profileName string) error {
	if err := config.ValidateProfileName(profileName); err != nil {
		return err
	}
	if !profile.ProfileExists(profileName) {
		return fmt.Errorf("profile '%s' does not exist", profileName)
	}

	pDir := config.GetProfileDir(profileName)
	geminiDir := filepath.Join(pDir, ".gemini", "antigravity")
	convDir := filepath.Join(geminiDir, "conversations")
	brainDir := filepath.Join(geminiDir, "brain")

	convExists := dirExists(convDir)
	brainExists := dirExists(brainDir)

	if !convExists && !brainExists {
		fmt.Fprintf(w, "Profile '%s' has no saved AI chats.\n", profileName)
		return nil
	}

	entries, err := os.ReadDir(convDir)
	if err != nil || len(entries) == 0 {
		fmt.Fprintf(w, "Profile '%s' has no saved AI chats.\n", profileName)
		return nil
	}

	fmt.Fprintf(w, "AI Conversations in profile '%s':\n", profileName)
	fmt.Fprintf(w, "%-38s %-32s %s\n", "CONVERSATION ID", "TITLE", "ARTIFACTS")
	fmt.Fprintf(w, "%-38s %-32s %s\n", "---------------", "-----", "---------")

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		uuid := strings.TrimSuffix(entry.Name(), ".db")

		title := "(untitled conversation)"
		annotFile := filepath.Join(geminiDir, "annotations", uuid+".pbtxt")
		if data, err := os.ReadFile(annotFile); err == nil {
			m := titleRegex.FindStringSubmatch(string(data))
			if len(m) >= 2 && strings.TrimSpace(m[1]) != "" {
				title = strings.TrimSpace(m[1])
			}
		}

		if len(title) > 30 {
			title = title[:27] + "..."
		}

		artCount := "none"
		uBrainDir := filepath.Join(brainDir, uuid)
		if bEntries, err := os.ReadDir(uBrainDir); err == nil {
			mdCount := 0
			for _, be := range bEntries {
				if !be.IsDir() && strings.HasSuffix(be.Name(), ".md") {
					mdCount++
				}
			}
			if mdCount > 0 {
				artCount = fmt.Sprintf("%d file(s)", mdCount)
			}
		}

		fmt.Fprintf(w, "%-38s %-32s %s\n", uuid, title, artCount)
		count++
	}

	if count == 0 {
		fmt.Fprintln(w, "No conversations found.")
	} else {
		fmt.Fprintf(w, "\nTotal conversations: %d\n", count)
	}
	return nil
}

func isCredentialFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "token") ||
		strings.Contains(lower, "oauth") ||
		strings.Contains(lower, "auth") ||
		strings.Contains(lower, "credential") ||
		strings.Contains(lower, "installation_id")
}

// ExportConversations exports AI chats to an archive, sanitized of credentials
func ExportConversations(profileName string, outPath string) error {
	if err := config.ValidateProfileName(profileName); err != nil {
		return err
	}
	if !profile.ProfileExists(profileName) {
		return fmt.Errorf("profile '%s' does not exist", profileName)
	}

	if profile.IsProfileRunning(profileName) {
		return fmt.Errorf("profile '%s' is currently running — stop it first to ensure SQLite database flush (multigravity stop %s)", profileName, profileName)
	}

	pDir := config.GetProfileDir(profileName)
	geminiDir := filepath.Join(pDir, ".gemini", "antigravity")
	if !dirExists(geminiDir) {
		return fmt.Errorf("profile '%s' has no AI data (.gemini/antigravity does not exist)", profileName)
	}

	if outPath == "" {
		outPath = fmt.Sprintf("./%s-ai-chats.tar.gz", profileName)
	}

	items := []string{"conversations", "brain", "annotations", "knowledge", "antigravity_state.pbtxt", "agyhub_summaries_proto.pb"}
	var foundItems []string
	for _, item := range items {
		if _, err := os.Stat(filepath.Join(geminiDir, item)); err == nil {
			foundItems = append(foundItems, item)
		}
	}

	if len(foundItems) == 0 {
		return fmt.Errorf("no AI conversation or brain data found in profile '%s'", profileName)
	}

	fmt.Printf("Exporting AI chats from '%s' to %s ...\n", profileName, outPath)

	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create archive %s: %w", outPath, err)
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, item := range foundItems {
		itemPath := filepath.Join(geminiDir, item)
		_ = filepath.Walk(itemPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if isCredentialFile(info.Name()) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			rel, err := filepath.Rel(geminiDir, path)
			if err != nil {
				return nil
			}

			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return nil
			}
			header.Name = filepath.ToSlash(rel)

			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			if info.Mode().IsRegular() {
				f, err := os.Open(path)
				if err != nil {
					return nil
				}
				defer f.Close()
				_, _ = io.Copy(tw, f)
			}
			return nil
		})
	}

	convCount := countConversations(geminiDir)
	fmt.Printf("✓ Successfully exported %d conversation(s) to %s (credentials sanitized).\n", convCount, outPath)
	return nil
}

// ImportConversations imports AI conversations into a profile
func ImportConversations(archivePath string, profileName string) error {
	if _, err := os.Stat(archivePath); err != nil {
		return fmt.Errorf("file not found: %s", archivePath)
	}
	if err := config.ValidateProfileName(profileName); err != nil {
		return err
	}
	if !profile.ProfileExists(profileName) {
		return fmt.Errorf("profile '%s' does not exist", profileName)
	}

	if profile.IsProfileRunning(profileName) {
		return fmt.Errorf("profile '%s' is currently running — stop it first (multigravity stop %s)", profileName, profileName)
	}

	pDir := config.GetProfileDir(profileName)
	targetGemini := filepath.Join(pDir, ".gemini", "antigravity")
	if err := os.MkdirAll(targetGemini, 0755); err != nil {
		return err
	}

	fmt.Printf("Importing AI chats into '%s'...\n", profileName)

	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		err = unpackZip(f, targetGemini)
	} else {
		err = unpackTarGz(f, targetGemini)
	}
	if err != nil {
		return err
	}

	convCount := countConversations(targetGemini)
	fmt.Printf("✓ Successfully imported AI chats into profile '%s' (%d conversation(s) now available).\n", profileName, convCount)
	return nil
}

func unpackTarGz(r io.Reader, dest string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		cleanPath := filepath.Clean(hdr.Name)
		if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
			continue
		}
		if isCredentialFile(filepath.Base(cleanPath)) {
			continue
		}

		targetPath := filepath.Join(dest, cleanPath)
		// Skip overwriting existing summaries and state
		if (cleanPath == "agyhub_summaries_proto.pb" || cleanPath == "antigravity_state.pbtxt") && fileExists(targetPath) {
			continue
		}

		if hdr.FileInfo().IsDir() {
			_ = os.MkdirAll(targetPath, 0755)
			continue
		}

		_ = os.MkdirAll(filepath.Dir(targetPath), 0755)
		out, err := os.Create(targetPath)
		if err != nil {
			continue
		}
		_, _ = io.Copy(out, tr)
		out.Close()
	}
	return nil
}

func unpackZip(f *os.File, dest string) error {
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(f, stat.Size())
	if err != nil {
		return err
	}

	for _, zf := range zr.File {
		cleanPath := filepath.Clean(zf.Name)
		if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
			continue
		}
		if isCredentialFile(filepath.Base(cleanPath)) {
			continue
		}

		targetPath := filepath.Join(dest, cleanPath)
		if (cleanPath == "agyhub_summaries_proto.pb" || cleanPath == "antigravity_state.pbtxt") && fileExists(targetPath) {
			continue
		}

		if zf.FileInfo().IsDir() {
			_ = os.MkdirAll(targetPath, 0755)
			continue
		}

		_ = os.MkdirAll(filepath.Dir(targetPath), 0755)
		rc, err := zf.Open()
		if err != nil {
			continue
		}
		out, err := os.Create(targetPath)
		if err == nil {
			_, _ = io.Copy(out, rc)
			out.Close()
		}
		rc.Close()
	}
	return nil
}

// SyncConversations synchronizes AI conversations from src to dest non-destructively
func SyncConversations(src string, dest string) error {
	if src == dest {
		return fmt.Errorf("source and target profiles cannot be the same")
	}
	if err := config.ValidateProfileName(src); err != nil {
		return err
	}
	if err := config.ValidateProfileName(dest); err != nil {
		return err
	}
	if !profile.ProfileExists(src) {
		return fmt.Errorf("profile '%s' does not exist", src)
	}
	if !profile.ProfileExists(dest) {
		return fmt.Errorf("profile '%s' does not exist", dest)
	}

	if profile.IsProfileRunning(src) {
		return fmt.Errorf("profile '%s' is currently running — stop it first (multigravity stop %s)", src, src)
	}
	if profile.IsProfileRunning(dest) {
		return fmt.Errorf("profile '%s' is currently running — stop it first (multigravity stop %s)", dest, dest)
	}

	srcGemini := filepath.Join(config.GetProfileDir(src), ".gemini", "antigravity")
	destGemini := filepath.Join(config.GetProfileDir(dest), ".gemini", "antigravity")

	if !dirExists(srcGemini) {
		return fmt.Errorf("profile '%s' has no AI data (.gemini/antigravity does not exist)", src)
	}

	_ = os.MkdirAll(destGemini, 0755)
	_ = os.MkdirAll(filepath.Join(destGemini, "conversations"), 0755)
	_ = os.MkdirAll(filepath.Join(destGemini, "annotations"), 0755)
	_ = os.MkdirAll(filepath.Join(destGemini, "brain"), 0755)

	synced := 0
	srcConv := filepath.Join(srcGemini, "conversations")
	if entries, err := os.ReadDir(srcConv); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".db") {
				continue
			}
			uuid := strings.TrimSuffix(e.Name(), ".db")
			destDb := filepath.Join(destGemini, "conversations", e.Name())
			if !fileExists(destDb) {
				_ = copyFile(filepath.Join(srcConv, e.Name()), destDb)
				synced++
			}

			// Sync annotation
			srcAnnot := filepath.Join(srcGemini, "annotations", uuid+".pbtxt")
			destAnnot := filepath.Join(destGemini, "annotations", uuid+".pbtxt")
			if fileExists(srcAnnot) && !fileExists(destAnnot) {
				_ = copyFile(srcAnnot, destAnnot)
			}

			// Sync brain
			srcBrain := filepath.Join(srcGemini, "brain", uuid)
			destBrain := filepath.Join(destGemini, "brain", uuid)
			if dirExists(srcBrain) && !dirExists(destBrain) {
				_ = profile.CopyDir(srcBrain, destBrain)
			}
		}
	}

	totalDest := countConversations(destGemini)
	fmt.Printf("✓ Successfully synced %d conversation(s) into '%s' (total: %d available).\n", synced, dest, totalDest)
	return nil
}

func countConversations(geminiDir string) int {
	convDir := filepath.Join(geminiDir, "conversations")
	entries, err := os.ReadDir(convDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".db") {
			count++
		}
	}
	return count
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
