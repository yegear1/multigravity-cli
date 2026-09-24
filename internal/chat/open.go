package chat

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

// GetOpenConversations returns a map of conversation UUIDs that currently have open file handles in the OS.
func GetOpenConversations(profileName string) (map[string]bool, error) {
	if err := config.ValidateProfileName(profileName); err != nil {
		return nil, err
	}
	if !profile.ProfileExists(profileName) {
		return nil, fmt.Errorf("profile '%s' does not exist", profileName)
	}

	pDir := config.GetProfileDir(profileName)
	convDir := filepath.Join(pDir, ".gemini", "antigravity", "conversations")
	if !dirExists(convDir) {
		return make(map[string]bool), nil
	}

	return getOpenConversationsOS(convDir)
}

func extractUUIDFromDBName(base string) string {
	if strings.HasSuffix(base, ".db-wal") {
		return strings.TrimSuffix(base, ".db-wal")
	}
	if strings.HasSuffix(base, ".db-shm") {
		return strings.TrimSuffix(base, ".db-shm")
	}
	if strings.HasSuffix(base, ".db") {
		return strings.TrimSuffix(base, ".db")
	}
	return ""
}
