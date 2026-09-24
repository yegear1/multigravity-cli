//go:build windows

package chat

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func getOpenConversationsOS(convDir string) (map[string]bool, error) {
	result := make(map[string]bool)
	entries, err := os.ReadDir(convDir)
	if err != nil {
		return result, nil
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}

		// If -wal or -shm exists with size > 0, SQLite has an active session
		if strings.HasSuffix(name, ".db-wal") || strings.HasSuffix(name, ".db-shm") {
			if fi, err := entry.Info(); err == nil && fi.Size() > 0 {
				uuid := extractUUIDFromDBName(name)
				if uuid != "" {
					result[uuid] = true
				}
			}
		}

		if strings.HasSuffix(name, ".db") {
			uuid := strings.TrimSuffix(name, ".db")
			dbPath := filepath.Join(convDir, name)
			// Try to open with write access without sharing - Windows will return ERROR_SHARING_VIOLATION if in use
			f, err := os.OpenFile(dbPath, os.O_RDWR, 0)
			if err != nil {
				if pathErr, ok := err.(*os.PathError); ok {
					if errno, ok := pathErr.Err.(syscall.Errno); ok {
						// 32 = ERROR_SHARING_VIOLATION, 33 = ERROR_LOCK_VIOLATION
						if errno == 32 || errno == 33 {
							result[uuid] = true
						}
					}
				}
			} else {
				f.Close()
			}
		}
	}
	return result, nil
}
