package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCloneProfile(t *testing.T) {
	tempHome := setupTestHome(t)

	// Create a source profile
	err := CreateProfile(CreateOptions{
		Name:             "source-profile",
		IsolatedDotfiles: true,
		IsolatedConfig:   true,
	})
	if err != nil {
		t.Fatalf("failed to create source profile: %v", err)
	}

	srcDir := filepath.Join(tempHome, "source-profile")
	// Add custom file and symlink
	sampleFile := filepath.Join(srcDir, "custom.txt")
	if err := os.WriteFile(sampleFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to write custom file: %v", err)
	}

	linkTarget := filepath.Join(srcDir, "custom.txt")
	linkPath := filepath.Join(srcDir, "custom_link.txt")
	if err := os.Symlink(linkTarget, linkPath); err != nil {
		t.Fatalf("failed to create test symlink: %v", err)
	}

	// 1. Success clone
	err = CloneProfile("source-profile", "cloned-profile")
	if err != nil {
		t.Fatalf("expected CloneProfile to succeed, got: %v", err)
	}

	destDir := filepath.Join(tempHome, "cloned-profile")
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		t.Fatalf("expected cloned profile directory %q to exist", destDir)
	}

	// Verify custom file content
	destCustom := filepath.Join(destDir, "custom.txt")
	content, err := os.ReadFile(destCustom)
	if err != nil || string(content) != "hello world" {
		t.Fatalf("cloned custom file missing or content mismatch: %v", err)
	}

	// Verify symlink was preserved as symlink
	destLink := filepath.Join(destDir, "custom_link.txt")
	destLinkInfo, err := os.Lstat(destLink)
	if err != nil {
		t.Fatalf("failed to stat dest symlink: %v", err)
	}
	if destLinkInfo.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected cloned symlink to be a symlink, but was not")
	}

	// 2. Clone from non-existent profile should fail
	err = CloneProfile("non-existent", "dest-profile")
	if err == nil {
		t.Errorf("expected error cloning non-existent profile, got nil")
	}

	// 3. Clone to existing profile should fail
	err = CloneProfile("source-profile", "cloned-profile")
	if err == nil {
		t.Errorf("expected error cloning to existing profile, got nil")
	}

	// 4. Invalid name validation
	err = CloneProfile("invalid name!", "valid-dest")
	if err == nil {
		t.Errorf("expected error with invalid src name, got nil")
	}
	err = CloneProfile("source-profile", "invalid dest!")
	if err == nil {
		t.Errorf("expected error with invalid dest name, got nil")
	}
}
