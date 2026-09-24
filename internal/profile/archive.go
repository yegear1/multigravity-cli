package profile

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/shortcut"
)

var cacheDirNames = []string{
	"Cache",
	"Code Cache",
	"GPUCache",
	"DawnGraphiteCache",
	"DawnWebGPUCache",
	"Crashpad",
	"logs",
	"CachedData",
	"CachedExtensions",
	"CachedExtensionVSIXs",
	"webrtc_event_logs",
	"_cacache",
}

func isCacheDir(relPath string) bool {
	// Normalize path separators to "/"
	cleanRel := filepath.ToSlash(filepath.Clean(relPath))
	parts := strings.Split(cleanRel, "/")

	for _, part := range parts {
		if part == ".cache" || part == "crashes" || part == "CacheStorage" || part == "ScriptCache" {
			return true
		}
		for _, name := range cacheDirNames {
			if part == name {
				return true
			}
		}
	}

	if cleanRel == "Library/Caches" || strings.HasPrefix(cleanRel, "Library/Caches/") {
		return true
	}
	if cleanRel == "AppData/Local/Temp" || strings.HasPrefix(cleanRel, "AppData/Local/Temp/") {
		return true
	}

	return false
}

// ExportProfile exports a profile to a compressed archive (.tar.gz or .zip)
func ExportProfile(name, outPath string, includeCache bool) (string, error) {
	if err := config.ValidateProfileName(name); err != nil {
		return "", fmt.Errorf("invalid profile name: %w", err)
	}

	profileDir := config.GetProfileDir(name)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return "", fmt.Errorf("profile %q does not exist", name)
	}

	if outPath == "" {
		if runtime.GOOS == "windows" {
			outPath = fmt.Sprintf("./%s.zip", name)
		} else {
			outPath = fmt.Sprintf("./%s.tar.gz", name)
		}
	}

	// Ensure destination directory exists
	outDir := filepath.Dir(outPath)
	if outDir != "" && outDir != "." {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	isZip := strings.HasSuffix(strings.ToLower(outPath), ".zip")
	if isZip {
		if err := exportZip(profileDir, name, outPath, includeCache); err != nil {
			return "", err
		}
	} else {
		if err := exportTarGz(profileDir, name, outPath, includeCache); err != nil {
			return "", err
		}
	}

	return outPath, nil
}

func exportTarGz(profileDir, profileName, outPath string, includeCache bool) error {
	outFile, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create output file %q: %w", outPath, err)
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	err = filepath.WalkDir(profileDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(profileDir, path)
		if err != nil {
			return err
		}

		if rel == "." {
			return nil
		}

		if !includeCache && isCacheDir(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		archiveName := filepath.ToSlash(filepath.Join(profileName, rel))

		var linkTarget string
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err = os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to readlink %q: %w", path, err)
			}
		}

		header, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return fmt.Errorf("failed to create tar header for %q: %w", path, err)
		}

		header.Name = archiveName
		if d.IsDir() {
			header.Name += "/"
		}

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for %q: %w", archiveName, err)
		}

		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file %q: %w", path, err)
			}
			defer f.Close()

			if _, err := io.Copy(tw, f); err != nil {
				return fmt.Errorf("failed to copy file %q to archive: %w", path, err)
			}
		}

		return nil
	})

	return err
}

func exportZip(profileDir, profileName, outPath string, includeCache bool) error {
	outFile, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create output file %q: %w", outPath, err)
	}
	defer outFile.Close()

	zw := zip.NewWriter(outFile)
	defer zw.Close()

	err = filepath.WalkDir(profileDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(profileDir, path)
		if err != nil {
			return err
		}

		if rel == "." {
			return nil
		}

		if !includeCache && isCacheDir(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		archiveName := filepath.ToSlash(filepath.Join(profileName, rel))

		if d.IsDir() {
			archiveName += "/"
			_, err := zw.Create(archiveName)
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = archiveName
		header.Method = zip.Deflate

		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_, err = w.Write([]byte(linkTarget))
			return err
		}

		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(w, f)
			return err
		}

		return nil
	})

	return err
}

// ImportProfile restores a profile from a .tar.gz or .zip archive
func ImportProfile(archivePath, profileName string) (string, error) {
	if archivePath == "" {
		return "", fmt.Errorf("archive path cannot be empty")
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", archivePath)
	}
	defer f.Close()

	// Sniff magic bytes or file extension
	magic := make([]byte, 4)
	n, _ := f.Read(magic)
	_, _ = f.Seek(0, io.SeekStart)

	isZip := (n >= 4 && magic[0] == 'P' && magic[1] == 'K' && magic[2] == 0x03 && magic[3] == 0x04) ||
		strings.HasSuffix(strings.ToLower(archivePath), ".zip")

	// Infer profile name if not provided
	if profileName == "" {
		baseName := filepath.Base(archivePath)
		for _, ext := range []string{".tar.gz", ".tgz", ".tar", ".zip"} {
			if strings.HasSuffix(strings.ToLower(baseName), ext) {
				baseName = baseName[:len(baseName)-len(ext)]
				break
			}
		}
		profileName = baseName
	}

	if err := config.ValidateProfileName(profileName); err != nil {
		return "", fmt.Errorf("invalid profile name %q: %w", profileName, err)
	}

	dest := config.GetProfileDir(profileName)
	if _, err := os.Stat(dest); err == nil {
		return "", fmt.Errorf("profile %q already exists — choose a different name or delete it first", profileName)
	}

	baseHome := config.GetMultigravityHome()
	if err := os.MkdirAll(baseHome, 0755); err != nil {
		return "", fmt.Errorf("failed to create base profiles directory: %w", err)
	}

	// Extract into staging directory to inspect top-level layout
	stagingDir := filepath.Join(baseHome, fmt.Sprintf(".import_tmp_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create staging directory: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	if isZip {
		if err := extractZip(archivePath, stagingDir); err != nil {
			return "", err
		}
	} else {
		if err := extractTarGz(f, stagingDir); err != nil {
			return "", err
		}
	}

	// Check staging contents
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		return "", fmt.Errorf("failed to inspect extracted archive: %w", err)
	}

	if len(entries) == 1 && entries[0].IsDir() {
		// Single root directory: move its contents or rename it to dest
		topPath := filepath.Join(stagingDir, entries[0].Name())
		if err := os.Rename(topPath, dest); err != nil {
			// Fallback to CopyDir if Rename across devices fails
			if copyErr := CopyDir(topPath, dest); copyErr != nil {
				return "", fmt.Errorf("failed to move extracted profile to destination: %w", copyErr)
			}
		}
	} else {
		// Multiple entries at root of archive: staging itself is the profile root
		if err := os.Rename(stagingDir, dest); err != nil {
			if copyErr := CopyDir(stagingDir, dest); copyErr != nil {
				return "", fmt.Errorf("failed to move extracted profile to destination: %w", copyErr)
			}
		}
	}

	_ = shortcut.CreateShortcut(profileName)

	return profileName, nil
}

func extractTarGz(r io.Reader, destDir string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to read gzip archive: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return fmt.Errorf("illegal file path in archive: %q", header.Name)
		}

		targetPath := filepath.Join(destDir, cleanName)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
		case tar.TypeSymlink:
			_ = os.Remove(targetPath)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, header.FileInfo().Mode().Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	return nil
}

func extractZip(archivePath, destDir string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to read zip archive: %w", err)
	}
	defer zr.Close()

	for _, file := range zr.File {
		cleanName := filepath.Clean(file.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return fmt.Errorf("illegal file path in archive: %q", file.Name)
		}

		targetPath := filepath.Join(destDir, cleanName)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		if file.Mode()&os.ModeSymlink != 0 {
			rc, err := file.Open()
			if err != nil {
				return err
			}
			buf := new(bytes.Buffer)
			_, _ = io.Copy(buf, rc)
			rc.Close()

			_ = os.Remove(targetPath)
			if err := os.Symlink(buf.String(), targetPath); err != nil {
				return err
			}
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return err
		}

		f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode().Perm())
		if err != nil {
			rc.Close()
			return err
		}

		if _, err := io.Copy(f, rc); err != nil {
			f.Close()
			rc.Close()
			return err
		}

		f.Close()
		rc.Close()
	}

	return nil
}
