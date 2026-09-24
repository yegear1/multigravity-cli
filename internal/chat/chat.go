package chat

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var (
	titleRegex      = regexp.MustCompile(`title:\s*"([^"]*)"`)
	archivedRegex   = regexp.MustCompile(`archived:\s*true`)
	archivedTsRegex = regexp.MustCompile(`archival_status_timestamp:\s*\{\s*seconds:\s*(\d+)`)
	lastViewTsRegex = regexp.MustCompile(`last_user_view_time:\s*\{\s*seconds:\s*(\d+)`)
	tagStripRegex   = regexp.MustCompile(`@\[[^\]]+\]\s*`)
)

// ConversationFilter defines filter criteria for listing conversations
type ConversationFilter string

const (
	FilterAll      ConversationFilter = "all"
	FilterActive   ConversationFilter = "active"
	FilterArchived ConversationFilter = "archived"
)

// ConversationInfo holds metadata about a single AI conversation
type ConversationInfo struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	ArtifactCount int        `json:"artifact_count"`
	Archived      bool       `json:"archived"`
	ArchivedAt    *time.Time `json:"archived_at,omitempty"`
	LastViewAt    *time.Time `json:"last_view_at,omitempty"`
	Size          string     `json:"size"`
	SizeBytes     int64      `json:"size_bytes"`
}

// ListConversations prints a table of AI conversations in a profile
func ListConversations(profileName string) error {
	return ListConversationsFilter(os.Stdout, profileName, FilterAll)
}

// GetConversations returns structured metadata of all AI conversations in a profile
func GetConversations(profileName string) ([]ConversationInfo, error) {
	return GetFilteredConversations(profileName, FilterAll)
}

// GetFilteredConversations returns structured metadata of AI conversations filtered by status
func GetFilteredConversations(profileName string, filter ConversationFilter) ([]ConversationInfo, error) {
	if err := config.ValidateProfileName(profileName); err != nil {
		return nil, err
	}
	if !profile.ProfileExists(profileName) {
		return nil, fmt.Errorf("profile '%s' does not exist", profileName)
	}

	pDir := config.GetProfileDir(profileName)
	geminiDir := filepath.Join(pDir, ".gemini", "antigravity")
	convDir := filepath.Join(geminiDir, "conversations")
	brainDir := filepath.Join(geminiDir, "brain")

	convExists := dirExists(convDir)
	brainExists := dirExists(brainDir)

	if !convExists && !brainExists {
		return []ConversationInfo{}, nil
	}

	entries, err := os.ReadDir(convDir)
	if err != nil || len(entries) == 0 {
		return []ConversationInfo{}, nil
	}

	var convs []ConversationInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		uuid := strings.TrimSuffix(entry.Name(), ".db")

		title := ""
		isArchived := false
		var archivedAt *time.Time
		var lastViewAt *time.Time
		var sizeBytes int64

		// 1. Conversation SQLite DB size
		dbPath := filepath.Join(convDir, entry.Name())
		if fi, err := os.Stat(dbPath); err == nil {
			sizeBytes += fi.Size()
		}

		// 2. Annotation pbtxt metadata & size
		annotFile := filepath.Join(geminiDir, "annotations", uuid+".pbtxt")
		if fi, err := os.Stat(annotFile); err == nil {
			sizeBytes += fi.Size()
		}
		if data, err := os.ReadFile(annotFile); err == nil {
			content := string(data)
			if archivedRegex.MatchString(content) {
				isArchived = true
			}
			if m := archivedTsRegex.FindStringSubmatch(content); len(m) >= 2 {
				if sec, err := strconv.ParseInt(m[1], 10, 64); err == nil {
					t := time.Unix(sec, 0)
					archivedAt = &t
				}
			}
			if m := lastViewTsRegex.FindStringSubmatch(content); len(m) >= 2 {
				if sec, err := strconv.ParseInt(m[1], 10, 64); err == nil {
					t := time.Unix(sec, 0)
					lastViewAt = &t
				}
			}
			if m := titleRegex.FindStringSubmatch(content); len(m) >= 2 && strings.TrimSpace(m[1]) != "" {
				title = strings.TrimSpace(m[1])
			}
		}

		// Apply filter
		if filter == FilterActive && isArchived {
			continue
		}
		if filter == FilterArchived && !isArchived {
			continue
		}

		// If title not found in annotations, try transcript
		if title == "" || title == "(untitled conversation)" {
			transcriptFile := filepath.Join(brainDir, uuid, ".system_generated", "logs", "transcript.jsonl")
			if t := extractTitleFromTranscript(transcriptFile); t != "" {
				title = t
			} else {
				title = "(untitled conversation)"
			}
		}

		// 3. Brain directory size and artifact count
		mdCount := 0
		uBrainDir := filepath.Join(brainDir, uuid)
		if dirExists(uBrainDir) {
			_ = filepath.Walk(uBrainDir, func(_ string, info os.FileInfo, err error) error {
				if err == nil && info != nil && !info.IsDir() {
					sizeBytes += info.Size()
					if strings.HasSuffix(info.Name(), ".md") {
						mdCount++
					}
				}
				return nil
			})
		}

		convs = append(convs, ConversationInfo{
			ID:            uuid,
			Title:         title,
			ArtifactCount: mdCount,
			Archived:      isArchived,
			ArchivedAt:    archivedAt,
			LastViewAt:    lastViewAt,
			Size:          formatBytes(sizeBytes),
			SizeBytes:     sizeBytes,
		})
	}

	// Sort: Active conversations first, then by last activity (most recent first)
	sort.SliceStable(convs, func(i, j int) bool {
		if convs[i].Archived != convs[j].Archived {
			return !convs[i].Archived
		}
		var ti, tj int64
		if convs[i].Archived && convs[i].ArchivedAt != nil {
			ti = convs[i].ArchivedAt.Unix()
		} else if convs[i].LastViewAt != nil {
			ti = convs[i].LastViewAt.Unix()
		}

		if convs[j].Archived && convs[j].ArchivedAt != nil {
			tj = convs[j].ArchivedAt.Unix()
		} else if convs[j].LastViewAt != nil {
			tj = convs[j].LastViewAt.Unix()
		}
		return ti > tj
	})

	return convs, nil
}

type transcriptPayload struct {
	Content string `json:"content"`
}

func extractTitleFromTranscript(transcriptPath string) string {
	f, err := os.Open(transcriptPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	line, err := reader.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return ""
	}

	var entry transcriptPayload
	if err := json.Unmarshal(line, &entry); err != nil {
		return ""
	}

	text := entry.Content
	if idx := strings.Index(text, "<USER_REQUEST>"); idx != -1 {
		text = text[idx+len("<USER_REQUEST>"):]
	}
	if idx := strings.Index(text, "</USER_REQUEST>"); idx != -1 {
		text = text[:idx]
	}
	text = tagStripRegex.ReplaceAllString(text, "")
	text = strings.TrimSpace(text)

	lines := strings.Split(text, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			if len(l) > 60 {
				return l[:57] + "..."
			}
			return l
		}
	}
	return ""
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%c", float64(b)/float64(div), "KMGTPE"[exp])
}

// ListConversationsWriter prints a table of AI conversations to the provided writer
func ListConversationsWriter(w io.Writer, profileName string) error {
	return ListConversationsFilter(w, profileName, FilterAll)
}

// ListConversationsFilter prints a table of AI conversations matching the filter
func ListConversationsFilter(w io.Writer, profileName string, filter ConversationFilter) error {
	convs, err := GetFilteredConversations(profileName, filter)
	if err != nil {
		return err
	}

	if len(convs) == 0 {
		switch filter {
		case FilterActive:
			fmt.Fprintf(w, "Profile '%s' has no active AI chats.\n", profileName)
		case FilterArchived:
			fmt.Fprintf(w, "Profile '%s' has no archived AI chats.\n", profileName)
		default:
			fmt.Fprintf(w, "Profile '%s' has no saved AI chats.\n", profileName)
		}
		return nil
	}

	activeCount := 0
	archivedCount := 0
	var totalSizeBytes int64
	for _, c := range convs {
		totalSizeBytes += c.SizeBytes
		if c.Archived {
			archivedCount++
		} else {
			activeCount++
		}
	}

	fmt.Fprintf(w, "AI Conversations in profile '%s':\n", profileName)
	fmt.Fprintf(w, "%-38s %-10s %-8s %-16s %-32s %s\n", "CONVERSATION ID", "STATUS", "SIZE", "LAST ACTIVITY", "TITLE / TOPIC", "ARTIFACTS")
	fmt.Fprintf(w, "%-38s %-10s %-8s %-16s %-32s %s\n", "---------------", "------", "----", "-------------", "-------------", "---------")

	for _, c := range convs {
		status := "active"
		if c.Archived {
			status = "archived"
		}

		timeStr := "N/A"
		if c.Archived && c.ArchivedAt != nil {
			timeStr = c.ArchivedAt.Local().Format("2006-01-02 15:04")
		} else if c.LastViewAt != nil {
			timeStr = c.LastViewAt.Local().Format("2006-01-02 15:04")
		}

		dispTitle := c.Title
		if len(dispTitle) > 30 {
			dispTitle = dispTitle[:27] + "..."
		}

		artCount := "none"
		if c.ArtifactCount > 0 {
			artCount = fmt.Sprintf("%d file(s)", c.ArtifactCount)
		}

		fmt.Fprintf(w, "%-38s %-10s %-8s %-16s %-32s %s\n", c.ID, status, c.Size, timeStr, dispTitle, artCount)
	}

	totalSizeStr := formatBytes(totalSizeBytes)
	if filter == FilterActive {
		fmt.Fprintf(w, "\nTotal active conversations: %d | Total size: %s\n", activeCount, totalSizeStr)
	} else if filter == FilterArchived {
		fmt.Fprintf(w, "\nTotal archived conversations: %d | Total size: %s\n", archivedCount, totalSizeStr)
	} else {
		fmt.Fprintf(w, "\nTotal conversations: %d (%d active, %d archived) | Total size: %s\n", len(convs), activeCount, archivedCount, totalSizeStr)
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
