package profile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyDir recursively copies a directory tree from src to dest, preserving symlinks and permissions.
func CopyDir(src, dest string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source %q: %w", src, err)
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("source %q is not a directory", src)
	}

	if err := os.MkdirAll(dest, srcInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("failed to create destination %q: %w", dest, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("failed to get info for %q: %w", srcPath, err)
		}

		switch {
		case info.Mode()&os.ModeSymlink != 0:
			linkTarget, err := os.Readlink(srcPath)
			if err != nil {
				return fmt.Errorf("failed to readlink %q: %w", srcPath, err)
			}
			// Remove existing destination file if present
			_ = os.Remove(destPath)
			if err := os.Symlink(linkTarget, destPath); err != nil {
				return fmt.Errorf("failed to create symlink at %q: %w", destPath, err)
			}
		case info.IsDir():
			if err := CopyDir(srcPath, destPath); err != nil {
				return err
			}
		default:
			if err := CopyFile(srcPath, destPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// CopyFile copies a regular file from src to dest preserving mode
func CopyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %q: %w", src, err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	info, err := in.Stat()
	mode := os.FileMode(0644)
	if err == nil {
		mode = info.Mode().Perm()
	}

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create destination file %q: %w", dest, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("failed to copy file contents from %q to %q: %w", src, dest, err)
	}

	return nil
}
