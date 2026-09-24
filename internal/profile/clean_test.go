package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

func TestCleanSingleProfile(t *testing.T) {
	tempHome := setupTestHome(t)

	err := CreateProfile(CreateOptions{Name: "clean-me"})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	profileDir := filepath.Join(tempHome, "clean-me")
	userDataDir := config.GetUserDataDir(profileDir)

	// Create dummy cache dirs and files
	cacheDirs := []string{
		filepath.Join(userDataDir, "Cache"),
		filepath.Join(userDataDir, "GPUCache"),
		filepath.Join(userDataDir, "logs"),
		filepath.Join(profileDir, ".cache"),
		filepath.Join(profileDir, ".gemini", "antigravity", "crashes"),
	}

	for _, d := range cacheDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", d, err)
		}
		dummyFile := filepath.Join(d, "dummy.bin")
		if err := os.WriteFile(dummyFile, []byte("temporary cached content"), 0644); err != nil {
			t.Fatalf("failed to write dummy file: %v", err)
		}
	}

	// Create user settings file that MUST be preserved
	userDir := filepath.Join(profileDir, "User")
	_ = os.MkdirAll(userDir, 0755)
	settingsFile := filepath.Join(userDir, "settings.json")
	if err := os.WriteFile(settingsFile, []byte(`{"workbench.colorTheme": "Default Dark"}`), 0644); err != nil {
		t.Fatalf("failed to create settings file: %v", err)
	}

	before, after, err := CleanSingleProfile("clean-me")
	if err != nil {
		t.Fatalf("CleanSingleProfile failed: %v", err)
	}

	if before == "" || after == "" {
		t.Errorf("expected non-empty before and after size strings")
	}

	// Verify cache directories/files were removed
	for _, d := range cacheDirs {
		dummyFile := filepath.Join(d, "dummy.bin")
		if _, err := os.Stat(dummyFile); !os.IsNotExist(err) {
			t.Errorf("expected dummy file in %s to be removed, but it exists", d)
		}
	}

	// Invariant: .cache directory must be recreated
	dotCache := filepath.Join(profileDir, ".cache")
	if stat, err := os.Stat(dotCache); os.IsNotExist(err) || !stat.IsDir() {
		t.Errorf("expected .cache directory to exist after clean")
	}

	// Invariant: User/settings.json must be preserved
	if _, err := os.Stat(settingsFile); os.IsNotExist(err) {
		t.Errorf("CRITICAL: User/settings.json was removed during clean!")
	}
}

func TestCleanProfileAll(t *testing.T) {
	_ = setupTestHome(t)

	_ = CreateProfile(CreateOptions{Name: "prof-1"})
	_ = CreateProfile(CreateOptions{Name: "prof-2"})

	// Mock prof-2 as running
	oldPIDsFn := getProfilePIDsFn
	defer func() { getProfilePIDsFn = oldPIDsFn }()

	getProfilePIDsFn = func(name string) ([]int, error) {
		if name == "prof-2" {
			return []int{7777}, nil
		}
		return nil, nil
	}

	err := CleanProfile("--all")
	if err != nil {
		t.Fatalf("expected CleanProfile --all to succeed, got: %v", err)
	}
}

func TestCleanProfileRunningError(t *testing.T) {
	_ = setupTestHome(t)

	_ = CreateProfile(CreateOptions{Name: "running-target"})

	oldPIDsFn := getProfilePIDsFn
	defer func() { getProfilePIDsFn = oldPIDsFn }()

	getProfilePIDsFn = func(name string) ([]int, error) {
		if name == "running-target" {
			return []int{8888}, nil
		}
		return nil, nil
	}

	err := CleanProfile("running-target")
	if err == nil || !strings.Contains(err.Error(), "currently running — stop it first") {
		t.Fatalf("expected error for running profile, got: %v", err)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500B"},
		{1024, "1.0K"},
		{1048576, "1.0M"},
		{1073741824, "1.0G"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %s, want %s", tt.bytes, got, tt.want)
		}
	}
}
