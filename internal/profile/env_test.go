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

func TestPopulateWorktreeProjectEnv(t *testing.T) {
	// 1. Early returns
	// nil map should not panic
	PopulateWorktreeProjectEnv(nil, "/base", "/worktree")

	// empty base or worktree
	env := make(map[string]string)
	PopulateWorktreeProjectEnv(env, "", "/worktree")
	if len(env) != 0 {
		t.Errorf("expected empty env for empty baseRepoPath, got: %v", env)
	}

	PopulateWorktreeProjectEnv(env, "   ", "/worktree")
	if len(env) != 0 {
		t.Errorf("expected empty env for whitespace baseRepoPath, got: %v", env)
	}

	PopulateWorktreeProjectEnv(env, "/base", "")
	if len(env) != 0 {
		t.Errorf("expected empty env for empty worktreePath, got: %v", env)
	}

	// base == worktree
	PopulateWorktreeProjectEnv(env, "/base", "/base")
	if len(env) != 0 {
		t.Errorf("expected empty env when baseRepoPath == worktreePath, got: %v", env)
	}

	// 2. Full setup with .venv (with src/) and node_modules
	tempDir := t.TempDir()
	baseRepo := filepath.Join(tempDir, "base-repo")
	worktree := filepath.Join(tempDir, "base-repo", ".multigravity", "worktrees", "wt-1")
	worktreeSrc := filepath.Join(worktree, "src")

	binName := "bin"
	if runtime.GOOS == "windows" {
		binName = "Scripts"
	}
	venvBin := filepath.Join(baseRepo, ".venv", binName)
	nodeBin := filepath.Join(baseRepo, "node_modules", ".bin")

	for _, d := range []string{venvBin, nodeBin, worktreeSrc} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", d, err)
		}
	}

	envFull := map[string]string{
		"PATH": "/usr/bin",
	}
	PopulateWorktreeProjectEnv(envFull, baseRepo, worktree)

	expectedVenv := filepath.Join(baseRepo, ".venv")
	if envFull["UV_PROJECT_ENVIRONMENT"] != expectedVenv {
		t.Errorf("UV_PROJECT_ENVIRONMENT = %q; want %q", envFull["UV_PROJECT_ENVIRONMENT"], expectedVenv)
	}
	if envFull["VIRTUAL_ENV"] != expectedVenv {
		t.Errorf("VIRTUAL_ENV = %q; want %q", envFull["VIRTUAL_ENV"], expectedVenv)
	}

	expectedNodeModules := filepath.Join(baseRepo, "node_modules")
	if envFull["NODE_PATH"] != expectedNodeModules {
		t.Errorf("NODE_PATH = %q; want %q", envFull["NODE_PATH"], expectedNodeModules)
	}

	sep := string(os.PathListSeparator)
	expectedPythonPath := worktreeSrc + sep + worktree
	if envFull["PYTHONPATH"] != expectedPythonPath {
		t.Errorf("PYTHONPATH = %q; want %q", envFull["PYTHONPATH"], expectedPythonPath)
	}

	// PATH should contain nodeBin and venvBin prepended
	pathParts := strings.Split(envFull["PATH"], sep)
	if len(pathParts) < 3 {
		t.Fatalf("expected at least 3 parts in PATH, got %v", pathParts)
	}
	if filepath.Clean(pathParts[0]) != filepath.Clean(nodeBin) {
		t.Errorf("expected nodeBin first in PATH, got %q (want %q)", pathParts[0], nodeBin)
	}
	if filepath.Clean(pathParts[1]) != filepath.Clean(venvBin) {
		t.Errorf("expected venvBin second in PATH, got %q (want %q)", pathParts[1], venvBin)
	}

	// 3. Worktree without src/ directory
	worktreeNoSrc := filepath.Join(tempDir, "base-repo", ".multigravity", "worktrees", "wt-no-src")
	if err := os.MkdirAll(worktreeNoSrc, 0755); err != nil {
		t.Fatal(err)
	}
	envNoSrc := make(map[string]string)
	PopulateWorktreeProjectEnv(envNoSrc, baseRepo, worktreeNoSrc)
	if envNoSrc["PYTHONPATH"] != worktreeNoSrc {
		t.Errorf("PYTHONPATH without src = %q; want %q", envNoSrc["PYTHONPATH"], worktreeNoSrc)
	}

	// 4. Pre-existing values and idempotency
	envExisting := map[string]string{
		"UV_PROJECT_ENVIRONMENT": "/custom/uv",
		"VIRTUAL_ENV":            "/custom/venv",
		"NODE_PATH":              "/custom/node",
		"PATH":                   nodeBin + sep + venvBin + sep + "/bin",
		"PYTHONPATH":             "/custom/python",
	}
	PopulateWorktreeProjectEnv(envExisting, baseRepo, worktree)
	if envExisting["UV_PROJECT_ENVIRONMENT"] != "/custom/uv" {
		t.Errorf("expected preserved UV_PROJECT_ENVIRONMENT, got %q", envExisting["UV_PROJECT_ENVIRONMENT"])
	}
	if envExisting["VIRTUAL_ENV"] != "/custom/venv" {
		t.Errorf("expected preserved VIRTUAL_ENV, got %q", envExisting["VIRTUAL_ENV"])
	}
	if envExisting["NODE_PATH"] != "/custom/node" {
		t.Errorf("expected preserved NODE_PATH, got %q", envExisting["NODE_PATH"])
	}
	// PATH shouldn't have duplicate nodeBin or venvBin
	parts := strings.Split(envExisting["PATH"], sep)
	countNode := 0
	countVenv := 0
	for _, p := range parts {
		if filepath.Clean(p) == filepath.Clean(nodeBin) {
			countNode++
		}
		if filepath.Clean(p) == filepath.Clean(venvBin) {
			countVenv++
		}
	}
	if countNode != 1 || countVenv != 1 {
		t.Errorf("expected exactly 1 instance each in PATH, got node=%d, venv=%d in %s", countNode, countVenv, envExisting["PATH"])
	}
	// PYTHONPATH should have worktree paths prepended before existing
	expectedPrependedPy := worktreeSrc + sep + worktree + sep + "/custom/python"
	if envExisting["PYTHONPATH"] != expectedPrependedPy {
		t.Errorf("PYTHONPATH with existing = %q; want %q", envExisting["PYTHONPATH"], expectedPrependedPy)
	}

	// 5. Cargo project CARGO_TARGET_DIR isolation
	cargoRepo := filepath.Join(tempDir, "cargo-repo")
	_ = os.MkdirAll(cargoRepo, 0755)
	_ = os.WriteFile(filepath.Join(cargoRepo, "Cargo.toml"), []byte("[package]\nname = \"demo\"\n"), 0644)
	cargoWT := filepath.Join(cargoRepo, ".multigravity", "worktrees", "wt-cargo")
	_ = os.MkdirAll(cargoWT, 0755)

	envCargo := make(map[string]string)
	PopulateWorktreeProjectEnv(envCargo, cargoRepo, cargoWT)
	expectedTarget := filepath.Join(cargoWT, "target")
	if envCargo["CARGO_TARGET_DIR"] != expectedTarget {
		t.Errorf("CARGO_TARGET_DIR = %q; want %q", envCargo["CARGO_TARGET_DIR"], expectedTarget)
	}

	// 6. Preserves existing CARGO_TARGET_DIR override
	envCargoCustom := map[string]string{
		"CARGO_TARGET_DIR": "/custom/target",
	}
	PopulateWorktreeProjectEnv(envCargoCustom, cargoRepo, cargoWT)
	if envCargoCustom["CARGO_TARGET_DIR"] != "/custom/target" {
		t.Errorf("CARGO_TARGET_DIR override = %q; want /custom/target", envCargoCustom["CARGO_TARGET_DIR"])
	}
}

func TestConfigureCompilerCacheAndSccache(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Test GetGlobalCompilerCacheDir defaults
	cacheDir := GetGlobalCompilerCacheDir()
	if !strings.Contains(cacheDir, "sccache") {
		t.Errorf("expected GetGlobalCompilerCacheDir to contain 'sccache', got %q", cacheDir)
	}

	// Explicit override
	t.Setenv("SCCACHE_DIR", "/custom/sccache/dir")
	if GetGlobalCompilerCacheDir() != "/custom/sccache/dir" {
		t.Errorf("expected custom SCCACHE_DIR, got %q", GetGlobalCompilerCacheDir())
	}

	// Mock fake sccache executable in ~/.cargo/bin
	cargoBin := filepath.Join(tempHome, ".cargo", "bin")
	_ = os.MkdirAll(cargoBin, 0755)
	binName := "sccache"
	if runtime.GOOS == "windows" {
		binName = "sccache.exe"
	}
	fakeSccache := filepath.Join(cargoBin, binName)
	_ = os.WriteFile(fakeSccache, []byte("#!/bin/sh\n"), 0755)

	found := FindSccacheBinary()
	if found == "" {
		t.Errorf("expected FindSccacheBinary to discover fake sccache in cargo bin")
	}

	envMap := make(map[string]string)
	ConfigureCompilerCache(envMap)
	if envMap["RUSTC_WRAPPER"] != found {
		t.Errorf("RUSTC_WRAPPER = %q, want %q", envMap["RUSTC_WRAPPER"], found)
	}
	if envMap["SCCACHE_DIR"] != "/custom/sccache/dir" {
		t.Errorf("SCCACHE_DIR = %q, want /custom/sccache/dir", envMap["SCCACHE_DIR"])
	}
}

func TestBuildEnrichedPATH_CrossProfileAndToolchainDiscovery(t *testing.T) {
	tempBase := t.TempDir()
	tempHome := filepath.Join(tempBase, "home")
	multigravityHome := filepath.Join(tempBase, "AntigravityProfiles")
	_ = os.MkdirAll(tempHome, 0755)
	_ = os.MkdirAll(multigravityHome, 0755)

	t.Setenv("REAL_HOME", tempHome)
	t.Setenv("MULTIGRAVITY_HOME", multigravityHome)

	// 1. Create host toolchain dirs
	hostUvPy := filepath.Join(tempHome, ".local", "share", "uv", "python", "cpython-3.12.0", "bin")
	hostGoSdk := filepath.Join(tempHome, "sdk", "go1.23.0", "bin")
	_ = os.MkdirAll(hostUvPy, 0755)
	_ = os.MkdirAll(hostGoSdk, 0755)

	// 2. Create peer profile dirs
	peer1 := filepath.Join(multigravityHome, "peer-alpha")
	peer1Cargo := filepath.Join(peer1, ".cargo", "bin")
	_ = os.MkdirAll(peer1Cargo, 0755)

	peer2 := filepath.Join(multigravityHome, "peer-beta")
	peer2Local := filepath.Join(peer2, ".local", "bin")
	_ = os.MkdirAll(peer2Local, 0755)

	// Peer 3 is isolated
	peer3 := filepath.Join(multigravityHome, "peer-isolated")
	peer3Go := filepath.Join(peer3, "go", "bin")
	_ = os.MkdirAll(peer3Go, 0755)
	_ = os.WriteFile(filepath.Join(peer3, ".isolated_toolchain"), []byte("1"), 0644)

	// Current profile
	curr := filepath.Join(multigravityHome, "profile-current")
	currBun := filepath.Join(curr, ".bun", "bin")
	_ = os.MkdirAll(currBun, 0755)

	// Test discovery from profile-current
	enriched := BuildEnrichedPATHForProfile("/usr/bin", tempHome, "profile-current")
	sep := string(os.PathListSeparator)
	parts := strings.Split(enriched, sep)

	hasDir := func(target string) bool {
		clean := filepath.Clean(target)
		for _, p := range parts {
			if filepath.Clean(p) == clean {
				return true
			}
		}
		return false
	}

	if !hasDir(hostUvPy) {
		t.Errorf("expected host uv python dir %s in enriched PATH", hostUvPy)
	}
	if !hasDir(hostGoSdk) {
		t.Errorf("expected host go sdk dir %s in enriched PATH", hostGoSdk)
	}
	if !hasDir(peer1Cargo) {
		t.Errorf("expected peer1 cargo dir %s in enriched PATH", peer1Cargo)
	}
	if !hasDir(peer2Local) {
		t.Errorf("expected peer2 local dir %s in enriched PATH", peer2Local)
	}
	if hasDir(peer3Go) {
		t.Errorf("did not expect isolated peer3 go dir %s in enriched PATH", peer3Go)
	}
	if hasDir(currBun) {
		t.Errorf("did not expect current profile's own dir %s in peer discovery", currBun)
	}

	// Now isolate profile-current
	_ = os.WriteFile(filepath.Join(curr, ".isolated_toolchain"), []byte("1"), 0644)
	enrichedIso := BuildEnrichedPATHForProfile("/usr/bin", tempHome, "profile-current")
	partsIso := strings.Split(enrichedIso, sep)

	hasIsoDir := func(target string) bool {
		clean := filepath.Clean(target)
		for _, p := range partsIso {
			if filepath.Clean(p) == clean {
				return true
			}
		}
		return false
	}

	if !hasIsoDir(hostUvPy) {
		t.Errorf("expected host uv python dir %s in isolated profile PATH", hostUvPy)
	}
	if hasIsoDir(peer1Cargo) {
		t.Errorf("did not expect peer1 cargo dir %s when current profile has .isolated_toolchain", peer1Cargo)
	}
}
