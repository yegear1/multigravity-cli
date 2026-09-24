package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

func TestGetProfileStats(t *testing.T) {
	tempHome := setupTestHome(t)

	// Create profile 1
	err := CreateProfile(CreateOptions{Name: "prof-a"})
	if err != nil {
		t.Fatalf("failed to create prof-a: %v", err)
	}

	// Add extensions to prof-a
	extDir := config.GetExtensionsDir(filepath.Join(tempHome, "prof-a"))
	_ = os.MkdirAll(filepath.Join(extDir, "ext1"), 0755)
	_ = os.MkdirAll(filepath.Join(extDir, "ext2"), 0755)

	// Create profile 2
	err = CreateProfile(CreateOptions{Name: "prof-b"})
	if err != nil {
		t.Fatalf("failed to create prof-b: %v", err)
	}

	stats, total, err := GetProfileStats()
	if err != nil {
		t.Fatalf("GetProfileStats failed: %v", err)
	}

	if len(stats) != 2 {
		t.Fatalf("expected 2 profiles in stats, got %d", len(stats))
	}

	if stats[0].Name != "prof-a" || stats[0].ExtensionCount != 2 {
		t.Errorf("expected prof-a with 2 extensions, got %+v", stats[0])
	}

	if stats[1].Name != "prof-b" || stats[1].ExtensionCount != 0 {
		t.Errorf("expected prof-b with 0 extensions, got %+v", stats[1])
	}

	if total == "" {
		t.Errorf("expected non-empty total usage string")
	}
}
