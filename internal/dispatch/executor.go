package dispatch

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/profile"
)

// ConfigureWorktreeBuildEnv enriches the given environment map with worktree-specific
// build isolation (e.g. CARGO_TARGET_DIR) and shared compiler cache (sccache) configuration.
func ConfigureWorktreeBuildEnv(envMap map[string]string, repoRoot, worktreePath string) {
	if envMap == nil || strings.TrimSpace(worktreePath) == "" {
		return
	}

	cleanWT := filepath.Clean(worktreePath)

	// 1. Isolate CARGO_TARGET_DIR per worktree so concurrent Cargo builds never contend on .cargo-lock
	if envMap["CARGO_TARGET_DIR"] == "" {
		envMap["CARGO_TARGET_DIR"] = filepath.Join(cleanWT, "target")
	}

	// 2. Configure global compiler cache (sccache) if available on PATH or enriched PATH
	ConfigureCompilerCache(envMap)

	// 3. Preserve Go module and build cache
	if hostHome, err := os.UserHomeDir(); err == nil && hostHome != "" {
		if envMap["GOCACHE"] == "" {
			envMap["GOCACHE"] = filepath.Join(hostHome, ".cache", "go-build")
		}
		if envMap["GOMODCACHE"] == "" {
			envMap["GOMODCACHE"] = filepath.Join(hostHome, "go", "pkg", "mod")
		}
	}
}

// ConfigureCompilerCache probes for sccache and configures RUSTC_WRAPPER and SCCACHE_DIR.
func ConfigureCompilerCache(envMap map[string]string) {
	profile.ConfigureCompilerCache(envMap)
}

// FindSccacheBinary searches for sccache in PATH, host user cargo bin, and enriched toolchains.
func FindSccacheBinary() string {
	return profile.FindSccacheBinary()
}

// GetGlobalCompilerCacheDir returns the canonical host directory for sccache object storage.
func GetGlobalCompilerCacheDir() string {
	return profile.GetGlobalCompilerCacheDir()
}

// ExecuteVerifyCommandWithEnv runs verification command cmdStr inside dir with the provided
// environment variables and build isolation.
func ExecuteVerifyCommandWithEnv(dir, cmdStr string, env map[string]string) ([]byte, int, error) {
	if verifyCommandHook != nil {
		return verifyCommandHook(dir, cmdStr)
	}
	return defaultExecuteVerifyCommandWithEnv(dir, cmdStr, env)
}

func defaultExecuteVerifyCommandWithEnv(dir, cmdStr string, customEnv map[string]string) ([]byte, int, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/c", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}
	if dir != "" {
		cmd.Dir = dir
	}

	// Prepare execution environment
	envMap := make(map[string]string)
	for _, envEntry := range os.Environ() {
		parts := strings.SplitN(envEntry, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Enrich with worktree build isolation if dir is a worktree
	if dir != "" {
		ConfigureWorktreeBuildEnv(envMap, "", dir)
	}

	// Apply custom caller overrides
	for k, v := range customEnv {
		envMap[k] = v
	}

	var finalEnv []string
	for k, v := range envMap {
		finalEnv = append(finalEnv, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = finalEnv

	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	if exitCode == 0 && strings.Contains(string(out), "testing: warning: no tests to run") {
		return out, 1, fmt.Errorf("verification matched zero tests (\"testing: warning: no tests to run\")")
	}
	return out, exitCode, err
}
