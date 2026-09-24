package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

func TestResolveColor(t *testing.T) {
	// Recognized names
	p, s, err := ResolveColor("blue")
	if err != nil {
		t.Fatalf("expected no error for blue, got: %v", err)
	}
	if p != "#1e40af" || s != "#172554" {
		t.Errorf("unexpected blue colors: %s, %s", p, s)
	}

	p, s, err = ResolveColor("Emerald")
	if err != nil {
		t.Fatalf("expected no error for Emerald, got: %v", err)
	}
	if p != "#065f46" || s != "#064e3b" {
		t.Errorf("unexpected emerald colors: %s, %s", p, s)
	}

	// Hex with hash
	p, s, err = ResolveColor("#AABBCC")
	if err != nil {
		t.Fatalf("expected no error for hex, got: %v", err)
	}
	if p != "#aabbcc" || s != "#aabbcc" {
		t.Errorf("unexpected hex colors: %s, %s", p, s)
	}

	// Hex without hash
	p, s, err = ResolveColor("112233")
	if err != nil {
		t.Fatalf("expected no error for hex without hash, got: %v", err)
	}
	if p != "#112233" || s != "#112233" {
		t.Errorf("unexpected hex colors: %s, %s", p, s)
	}

	// Invalid
	_, _, err = ResolveColor("not-a-color")
	if err == nil {
		t.Fatalf("expected error for invalid color")
	}
	if !strings.Contains(err.Error(), "invalid color") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestApplyAndGetProfileColor(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tmpDir)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", t.TempDir())

	profileName := "test-color-profile"
	err := CreateProfile(CreateOptions{Name: profileName})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	// Initially no color
	c, err := GetProfileColor(profileName)
	if err != nil {
		t.Fatalf("failed to get color: %v", err)
	}
	if c != "" {
		t.Errorf("expected empty color, got: %s", c)
	}

	// Apply valid color
	err = ApplyProfileColor(profileName, "red", false)
	if err != nil {
		t.Fatalf("failed to apply color: %v", err)
	}

	c, err = GetProfileColor(profileName)
	if err != nil {
		t.Fatalf("failed to get color: %v", err)
	}
	if c != "#991b1b" {
		t.Errorf("expected #991b1b, got: %s", c)
	}

	// Verify settings.json structure
	profileDir := config.GetProfileDir(profileName)
	settingsFile := filepath.Join(config.GetUserDataDir(profileDir), "User", "settings.json")
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal settings.json: %v", err)
	}
	colors, ok := parsed["workbench.colorCustomizations"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected workbench.colorCustomizations map in settings.json")
	}
	if colors["titleBar.activeBackground"] != "#991b1b" {
		t.Errorf("unexpected titleBar.activeBackground: %v", colors["titleBar.activeBackground"])
	}
	if colors["titleBar.activeForeground"] != "#ffffff" {
		t.Errorf("unexpected titleBar.activeForeground: %v", colors["titleBar.activeForeground"])
	}

	// Clear color
	err = ApplyProfileColor(profileName, "", true)
	if err != nil {
		t.Fatalf("failed to clear color: %v", err)
	}

	c, err = GetProfileColor(profileName)
	if err != nil {
		t.Fatalf("failed to get color after clear: %v", err)
	}
	if c != "" {
		t.Errorf("expected empty color after clear, got: %s", c)
	}

	// Check workbench.colorCustomizations was deleted
	data, err = os.ReadFile(settingsFile)
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}
	var clearedParsed map[string]interface{}
	if err := json.Unmarshal(data, &clearedParsed); err != nil {
		t.Fatalf("failed to unmarshal settings.json: %v", err)
	}
	if clearedParsed["workbench.colorCustomizations"] != nil {
		t.Errorf("expected workbench.colorCustomizations to be nil after clear, got: %v", clearedParsed["workbench.colorCustomizations"])
	}
}

func TestApplyProfileColorUncouplesSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tmpDir)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", t.TempDir())

	profileName := "symlink-color-profile"
	err := CreateProfile(CreateOptions{Name: profileName})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	profileDir := config.GetProfileDir(profileName)
	userDir := filepath.Join(config.GetUserDataDir(profileDir), "User")
	settingsFile := filepath.Join(userDir, "settings.json")

	// Create a shared global settings file
	globalSettings := filepath.Join(tmpDir, "global_settings.json")
	err = os.WriteFile(globalSettings, []byte(`{"editor.fontSize": 14}`), 0644)
	if err != nil {
		t.Fatalf("failed to create global settings: %v", err)
	}

	// Symlink profile settings to global
	_ = os.MkdirAll(userDir, 0755)
	_ = os.Remove(settingsFile)
	err = os.Symlink(globalSettings, settingsFile)
	if err != nil {
		t.Fatalf("failed to symlink settings: %v", err)
	}

	// Apply color to profile
	err = ApplyProfileColor(profileName, "blue", false)
	if err != nil {
		t.Fatalf("failed to apply color: %v", err)
	}

	// Verify profile settings is no longer a symlink
	fi, err := os.Lstat(settingsFile)
	if err != nil {
		t.Fatalf("failed to lstat settings file: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Errorf("expected settings file to not be a symlink")
	}

	// Verify global settings remains untouched
	globalData, err := os.ReadFile(globalSettings)
	if err != nil {
		t.Fatalf("failed to read global settings: %v", err)
	}
	if strings.Contains(string(globalData), "workbench.colorCustomizations") {
		t.Errorf("global settings was modified!")
	}

	// Verify profile settings preserved fontSize AND added colors
	profileData, err := os.ReadFile(settingsFile)
	if err != nil {
		t.Fatalf("failed to read profile settings: %v", err)
	}
	if !strings.Contains(string(profileData), `"editor.fontSize": 14`) {
		t.Errorf("profile settings did not preserve fontSize: %s", string(profileData))
	}
	if !strings.Contains(string(profileData), `"titleBar.activeBackground": "#1e40af"`) {
		t.Errorf("profile settings did not set titleBar.activeBackground: %s", string(profileData))
	}
}
