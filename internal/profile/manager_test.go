package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

func setupTestHome(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempDir)
	return tempDir
}

func TestCreateProfile(t *testing.T) {
	tempHome := setupTestHome(t)

	opts := CreateOptions{
		Name:             "test-profile",
		Shared:           true,
		IsolatedDotfiles: true,
		IsolatedMCP:      true,
		IsolatedSkills:   true,
		IsolatedConfig:   true,
		IsolatedGH:       true,
	}

	err := CreateProfile(opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	profileDir := filepath.Join(tempHome, "test-profile")
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		t.Fatalf("profile directory was not created: %s", profileDir)
	}

	// Check sentinels
	sentinels := []string{
		".shared",
		config.SentinelIsolatedDotfiles,
		config.SentinelIsolatedMCP,
		config.SentinelIsolatedSkills,
		config.SentinelIsolatedConfig,
		config.SentinelIsolatedGH,
	}

	for _, s := range sentinels {
		sentinelPath := filepath.Join(profileDir, s)
		if _, err := os.Stat(sentinelPath); os.IsNotExist(err) {
			t.Errorf("expected sentinel %q to exist, but was not found", s)
		}
	}

	// Duplicate creation should fail
	err = CreateProfile(opts)
	if err == nil {
		t.Errorf("expected duplicate creation to fail, but got nil")
	}

	// Invalid name should fail
	invalidOpts := CreateOptions{Name: "invalid_name"}
	if err := CreateProfile(invalidOpts); err == nil {
		t.Errorf("expected invalid name to fail, but got nil")
	}
}

func TestDeleteProfile(t *testing.T) {
	tempHome := setupTestHome(t)

	opts := CreateOptions{Name: "to-delete"}
	if err := CreateProfile(opts); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	profileDir := filepath.Join(tempHome, "to-delete")
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		t.Fatalf("expected profile dir to exist before deletion")
	}

	if err := DeleteProfile("to-delete", false); err != nil {
		t.Fatalf("failed to delete profile: %v", err)
	}

	if _, err := os.Stat(profileDir); !os.IsNotExist(err) {
		t.Fatalf("expected profile dir to be removed after deletion")
	}

	// Deleting non-existent profile should fail
	if err := DeleteProfile("non-existent", false); err == nil {
		t.Errorf("expected error deleting non-existent profile, got nil")
	}
}

func TestRenameProfile(t *testing.T) {
	tempHome := setupTestHome(t)

	if err := CreateProfile(CreateOptions{Name: "old-name"}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	if err := RenameProfile("old-name", "new-name"); err != nil {
		t.Fatalf("failed to rename profile: %v", err)
	}

	oldDir := filepath.Join(tempHome, "old-name")
	newDir := filepath.Join(tempHome, "new-name")

	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Errorf("expected old profile dir to be gone")
	}
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		t.Errorf("expected new profile dir to exist")
	}

	// Renaming to existing name should fail
	if err := CreateProfile(CreateOptions{Name: "another-profile"}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}
	if err := RenameProfile("another-profile", "new-name"); err == nil {
		t.Errorf("expected error renaming to already existing profile, got nil")
	}
}
