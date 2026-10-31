package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildEnrichedPATH_CandidateDiscovery(t *testing.T) {
	tempHome := t.TempDir()

	var expectedDirs []string
	if runtime.GOOS == "windows" {
		expectedDirs = []string{
			filepath.Join(tempHome, ".cargo", "bin"),
			filepath.Join(tempHome, ".local", "bin"),
			filepath.Join(tempHome, "AppData", "Local", "Programs", "Python"),
			filepath.Join(tempHome, "AppData", "Local", "Microsoft", "WinGet", "Links"),
			filepath.Join(tempHome, "AppData", "Local", "pnpm"),
			filepath.Join(tempHome, ".volta", "bin"),
			filepath.Join(tempHome, ".fnm", "current", "bin"),
		}
	} else {
		expectedDirs = []string{
			filepath.Join(tempHome, ".local", "bin"),
			filepath.Join(tempHome, ".cargo", "bin"),
			filepath.Join(tempHome, ".bun", "bin"),
			filepath.Join(tempHome, "go", "bin"),
			filepath.Join(tempHome, ".local", "share", "pnpm"),
			filepath.Join(tempHome, ".fnm", "current", "bin"),
			filepath.Join(tempHome, ".volta", "bin"),
			filepath.Join(tempHome, ".nvm", "versions", "node", "v22.0.0", "bin"),
			filepath.Join(tempHome, ".nvm", "versions", "node", "v20.10.0", "bin"),
		}
	}

	for _, d := range expectedDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create test dir %s: %v", d, err)
		}
	}

	basePath := "/usr/local/bin:/usr/bin:/bin"
	if runtime.GOOS == "windows" {
		basePath = `C:\Windows\system32;C:\Windows`
	}

	enriched := BuildEnrichedPATHWithBase(basePath, tempHome)
	sep := string(os.PathListSeparator)
	parts := strings.Split(enriched, sep)

	for _, d := range expectedDirs {
		found := false
		for _, p := range parts {
			if filepath.Clean(p) == filepath.Clean(d) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected candidate dir %s in enriched PATH, but was missing: %s", d, enriched)
		}
	}
}

func TestBuildEnrichedPATH_WSLOrdering(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("WSL ordering test is specific to non-Windows / WSL environments")
	}

	tempHome := t.TempDir()
	cargoBin := filepath.Join(tempHome, ".cargo", "bin")
	if err := os.MkdirAll(cargoBin, 0755); err != nil {
		t.Fatal(err)
	}

	// Simulated WSL PATH where Windows paths might be scattered
	basePath := "/mnt/c/Windows/system32:/usr/local/bin:/mnt/c/Users/dev/AppData/Local/bin:/usr/bin:/bin"

	enriched := BuildEnrichedPATHWithBase(basePath, tempHome)
	parts := strings.Split(enriched, ":")

	firstWindowsIdx := -1
	lastLinuxIdx := -1

	for i, p := range parts {
		if strings.HasPrefix(p, "/mnt/c/") {
			if firstWindowsIdx == -1 {
				firstWindowsIdx = i
			}
		} else {
			lastLinuxIdx = i
		}
	}

	if firstWindowsIdx != -1 && lastLinuxIdx != -1 {
		if firstWindowsIdx < lastLinuxIdx {
			t.Errorf("expected all Linux paths to precede /mnt/c/ Windows paths. First Windows at %d, last Linux at %d. Enriched: %s",
				firstWindowsIdx, lastLinuxIdx, enriched)
		}
	}

	// Verify candidate dir (cargoBin) is at the beginning
	if len(parts) == 0 || filepath.Clean(parts[0]) != filepath.Clean(cargoBin) {
		t.Errorf("expected host candidate dir to be first, got: %v", parts[0])
	}
}

func TestBuildEnrichedPATH_Deduplication(t *testing.T) {
	tempHome := t.TempDir()
	localBin := filepath.Join(tempHome, ".local", "bin")
	if err := os.MkdirAll(localBin, 0755); err != nil {
		t.Fatal(err)
	}

	sep := string(os.PathListSeparator)
	basePath := strings.Join([]string{
		"/usr/bin",
		localBin,
		"/usr/bin",
		"/bin",
		localBin,
		"",
	}, sep)

	enriched := BuildEnrichedPATHWithBase(basePath, tempHome)
	parts := strings.Split(enriched, sep)

	seen := make(map[string]int)
	for _, p := range parts {
		if p == "" {
			t.Errorf("enriched PATH should not contain empty entries: %s", enriched)
		}
		clean := filepath.Clean(p)
		seen[clean]++
		if seen[clean] > 1 {
			t.Errorf("duplicate entry detected in enriched PATH for %s: %s", clean, enriched)
		}
	}
}

func TestPopulateToolchainEnv(t *testing.T) {
	tempHome := t.TempDir()

	// Case 1: All unset
	env1 := make(map[string]string)
	PopulateToolchainEnv(env1, tempHome)

	expectedCargo := filepath.Join(tempHome, ".cargo")
	expectedRustup := filepath.Join(tempHome, ".rustup")
	expectedGo := filepath.Join(tempHome, "go")

	if env1["CARGO_HOME"] != expectedCargo {
		t.Errorf("expected CARGO_HOME=%s, got %s", expectedCargo, env1["CARGO_HOME"])
	}
	if env1["RUSTUP_HOME"] != expectedRustup {
		t.Errorf("expected RUSTUP_HOME=%s, got %s", expectedRustup, env1["RUSTUP_HOME"])
	}
	if env1["GOPATH"] != expectedGo {
		t.Errorf("expected GOPATH=%s, got %s", expectedGo, env1["GOPATH"])
	}

	// Case 2: Pre-existing values preserved
	customCargo := filepath.Join(tempHome, "custom_cargo")
	customRustup := filepath.Join(tempHome, "custom_rustup")
	customGo := filepath.Join(tempHome, "custom_go")

	env2 := map[string]string{
		"CARGO_HOME":  customCargo,
		"RUSTUP_HOME": customRustup,
		"GOPATH":      customGo,
	}
	PopulateToolchainEnv(env2, tempHome)

	if env2["CARGO_HOME"] != customCargo {
		t.Errorf("expected CARGO_HOME to remain %s, got %s", customCargo, env2["CARGO_HOME"])
	}
	if env2["RUSTUP_HOME"] != customRustup {
		t.Errorf("expected RUSTUP_HOME to remain %s, got %s", customRustup, env2["RUSTUP_HOME"])
	}
	if env2["GOPATH"] != customGo {
		t.Errorf("expected GOPATH to remain %s, got %s", customGo, env2["GOPATH"])
	}
}

func TestIsWindowsMountPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("isWindowsMountPath is active only on Unix/WSL")
	}

	tests := []struct {
		path     string
		expected bool
	}{
		{"/mnt/c/Windows/system32", true},
		{"/mnt/c", true},
		{"/mnt/c/", true},
		{"/mnt/d/Program Files", true},
		{"/mnt/z/tools", true},
		{"/mnt/data", false},
		{"/mnt/wslg", false},
		{"/mnt/drive/bin", false},
		{"/mnt/", false},
		{"/usr/bin", false},
		{"/home/user/.cargo/bin", false},
	}

	for _, tt := range tests {
		got := isWindowsMountPath(tt.path)
		if got != tt.expected {
			t.Errorf("isWindowsMountPath(%q) = %v; expected %v", tt.path, got, tt.expected)
		}
	}
}
