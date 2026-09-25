package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

func TestAuthOnlyProfileLayout(t *testing.T) {
	tempHome := setupTestHome(t)
	fakeHost := t.TempDir()
	t.Setenv("REAL_HOME", fakeHost)

	// Setup fake host assets
	hostExtDir := config.GetExtensionsDir(fakeHost)
	if err := os.MkdirAll(filepath.Join(hostExtDir, "test-extension"), 0755); err != nil {
		t.Fatalf("failed to create fake host extensions: %v", err)
	}
	_ = os.WriteFile(filepath.Join(hostExtDir, "test-extension", "package.json"), []byte(`{"name":"test-extension"}`), 0644)

	hostUserDir := filepath.Join(config.GetUserDataDir(fakeHost), "User")
	if err := os.MkdirAll(filepath.Join(hostUserDir, "snippets"), 0755); err != nil {
		t.Fatalf("failed to create fake host user dir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(hostUserDir, "settings.json"), []byte(`{"editor.fontSize":14}`), 0644)
	_ = os.WriteFile(filepath.Join(hostUserDir, "keybindings.json"), []byte(`[{"key":"ctrl+k"}]`), 0644)
	_ = os.WriteFile(filepath.Join(hostUserDir, "snippets", "go.json"), []byte(`{"print":{"body":"fmt.Println()"}}`), 0644)

	opts := CreateOptions{
		Name:     "auth-prof",
		AuthOnly: true,
	}

	if err := CreateProfile(opts); err != nil {
		t.Fatalf("failed to create auth-only profile: %v", err)
	}

	profileDir := filepath.Join(tempHome, "auth-prof")

	// 1. Sentinels
	if !hasSentinel(profileDir, config.SentinelAuthOnly) {
		t.Errorf("expected %s sentinel to exist", config.SentinelAuthOnly)
	}
	if !hasSentinel(profileDir, config.SentinelShared) {
		t.Errorf("expected %s sentinel to exist", config.SentinelShared)
	}

	// 2. Extensions symlink
	targetExt := config.GetExtensionsDir(profileDir)
	fi, err := os.Lstat(targetExt)
	if err != nil {
		t.Fatalf("failed to lstat target extensions: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected target extensions %q to be a symlink", targetExt)
	}

	// Check content accessible through symlink
	pkgData, err := os.ReadFile(filepath.Join(targetExt, "test-extension", "package.json"))
	if err != nil || string(pkgData) != `{"name":"test-extension"}` {
		t.Errorf("failed to read extension package.json through symlink: %v", err)
	}

	// 3. User settings symlinks
	targetUserDir := filepath.Join(config.GetUserDataDir(profileDir), "User")
	for _, item := range []string{"settings.json", "keybindings.json", "snippets"} {
		p := filepath.Join(targetUserDir, item)
		lfi, err := os.Lstat(p)
		if err != nil {
			t.Fatalf("failed to lstat %s: %v", p, err)
		}
		if lfi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("expected %s to be a symlink", p)
		}
	}

	// 4. GetProfile type
	info, err := GetProfile("auth-prof")
	if err != nil {
		t.Fatalf("failed to get profile info: %v", err)
	}
	if info.Type != "auth-only" {
		t.Errorf("expected profile type 'auth-only', got %q", info.Type)
	}
}

func TestSharedProfileAlias(t *testing.T) {
	tempHome := setupTestHome(t)
	fakeHost := t.TempDir()
	t.Setenv("REAL_HOME", fakeHost)

	opts := CreateOptions{
		Name:   "shared-prof",
		Shared: true,
	}

	if err := CreateProfile(opts); err != nil {
		t.Fatalf("failed to create shared profile: %v", err)
	}

	info, err := GetProfile("shared-prof")
	if err != nil {
		t.Fatalf("failed to get profile info: %v", err)
	}
	if info.Type != "auth-only" {
		t.Errorf("expected type 'auth-only', got %q", info.Type)
	}

	profileDir := filepath.Join(tempHome, "shared-prof")
	if !hasSentinel(profileDir, config.SentinelShared) {
		t.Errorf("expected %s sentinel to exist", config.SentinelShared)
	}
	if !hasSentinel(profileDir, config.SentinelAuthOnly) {
		t.Errorf("expected %s sentinel to exist", config.SentinelAuthOnly)
	}
}

func TestAuthOnlyColorUncoupling(t *testing.T) {
	setupTestHome(t)
	fakeHost := t.TempDir()
	t.Setenv("REAL_HOME", fakeHost)

	hostUserDir := filepath.Join(config.GetUserDataDir(fakeHost), "User")
	_ = os.MkdirAll(hostUserDir, 0755)
	_ = os.WriteFile(filepath.Join(hostUserDir, "settings.json"), []byte(`{"editor.fontSize":16}`), 0644)
	_ = os.WriteFile(filepath.Join(hostUserDir, "keybindings.json"), []byte(`[{"key":"ctrl+shift+p"}]`), 0644)

	opts := CreateOptions{
		Name:     "color-auth-prof",
		AuthOnly: true,
		Color:    "emerald",
	}

	if err := CreateProfile(opts); err != nil {
		t.Fatalf("failed to create profile with color: %v", err)
	}

	profileDir := config.GetProfileDir("color-auth-prof")
	settingsPath := filepath.Join(config.GetUserDataDir(profileDir), "User", "settings.json")
	keybindingsPath := filepath.Join(config.GetUserDataDir(profileDir), "User", "keybindings.json")

	// settings.json must be uncoupled (regular file, NOT symlink)
	fi, err := os.Lstat(settingsPath)
	if err != nil {
		t.Fatalf("failed to lstat settings.json: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Errorf("expected settings.json to be uncoupled from symlink into regular file")
	}

	// Host settings must be unmodified (zero colorCustomizations)
	hostSettingsBytes, _ := os.ReadFile(filepath.Join(hostUserDir, "settings.json"))
	if string(hostSettingsBytes) != `{"editor.fontSize":16}` {
		t.Errorf("host settings was unexpectedly modified: %s", string(hostSettingsBytes))
	}

	// Profile settings must have retained editor.fontSize AND added colorCustomizations
	profileSettingsBytes, _ := os.ReadFile(settingsPath)
	var parsed map[string]interface{}
	if err := json.Unmarshal(profileSettingsBytes, &parsed); err != nil {
		t.Fatalf("failed to parse profile settings: %v", err)
	}
	if fontSize, ok := parsed["editor.fontSize"].(float64); !ok || fontSize != 16 {
		t.Errorf("expected editor.fontSize 16, got %v", parsed["editor.fontSize"])
	}
	if _, ok := parsed["workbench.colorCustomizations"]; !ok {
		t.Errorf("expected workbench.colorCustomizations to be present in profile settings")
	}

	// keybindings.json must remain a symlink
	kfi, err := os.Lstat(keybindingsPath)
	if err != nil {
		t.Fatalf("failed to lstat keybindings: %v", err)
	}
	if kfi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected keybindings.json to remain a symlink")
	}
}

func TestAuthOnlyFallbackWhenHostMissing(t *testing.T) {
	setupTestHome(t)
	fakeEmptyHost := t.TempDir()
	t.Setenv("REAL_HOME", fakeEmptyHost)

	opts := CreateOptions{
		Name:     "fallback-prof",
		AuthOnly: true,
	}

	if err := CreateProfile(opts); err != nil {
		t.Fatalf("failed to create profile when host assets missing: %v", err)
	}

	profileDir := config.GetProfileDir("fallback-prof")
	extDir := config.GetExtensionsDir(profileDir)
	fi, err := os.Stat(extDir)
	if err != nil {
		t.Fatalf("extensions dir should exist as directory: %v", err)
	}
	if !fi.IsDir() {
		t.Errorf("expected extensions dir to be a directory")
	}
}
