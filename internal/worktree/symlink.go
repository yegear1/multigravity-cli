package worktree

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultAutoLinkPaths lists standard dependency and cache directories safe to share
// via symlink from the repository root into ephemeral worktrees if they exist.
//
// Notice: Build output directories like "target" are strictly excluded to prevent
// build lockfile contention (.cargo-lock) during parallel wave runs.
var DefaultAutoLinkPaths = []string{
	"node_modules",
	".venv",
	".cargo",
	".cache",
	".sccache",
}

// IsolatedBuildPaths lists build output directories that MUST NOT be shared across
// concurrent worktrees to prevent build lockfile contention and artifact corruption.
var IsolatedBuildPaths = []string{
	"target",
	".pytest_cache",
	".mypy_cache",
	".turbo",
}

func linkPath(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.Symlink(src, dst)
}

// IsIsolatedBuildPath reports whether the given directory name matches any build output path.
func IsIsolatedBuildPath(name string) bool {
	base := filepath.Base(name)
	for _, isolated := range IsolatedBuildPaths {
		if strings.EqualFold(base, isolated) {
			return true
		}
	}
	return false
}

// AutoLinkDependencies symlinks standard safe dependencies from repoRoot into targetDir
// if they exist in repoRoot and do not already exist in targetDir.
// Isolated build directories (e.g. "target") are skipped to prevent lockfile contention.
func AutoLinkDependencies(repoRoot, targetDir string) error {
	for _, dep := range DefaultAutoLinkPaths {
		if IsIsolatedBuildPath(dep) {
			continue
		}
		src := filepath.Join(repoRoot, dep)
		dst := filepath.Join(targetDir, dep)
		if _, err := os.Lstat(src); err == nil {
			if _, err := os.Lstat(dst); os.IsNotExist(err) {
				if err := linkPath(src, dst); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// IsolateWorktreeBuildArtifacts ensures worktree-specific directories exist for
// build artifacts when corresponding projects exist in repoRoot.
// For example, if repoRoot has a Cargo "target" or "Cargo.toml", it ensures targetDir/target
// is a dedicated local directory, NOT a symlink to repoRoot/target.
func IsolateWorktreeBuildArtifacts(repoRoot, targetDir string) error {
	// If repoRoot has target/ or Cargo.toml, ensure targetDir/target exists as a regular directory
	hasCargo := false
	if _, err := os.Stat(filepath.Join(repoRoot, "Cargo.toml")); err == nil {
		hasCargo = true
	} else if _, err := os.Stat(filepath.Join(repoRoot, "target")); err == nil {
		hasCargo = true
	}

	if hasCargo {
		wtTarget := filepath.Join(targetDir, "target")
		if fi, err := os.Lstat(wtTarget); os.IsNotExist(err) {
			if err := os.MkdirAll(wtTarget, 0755); err != nil {
				return err
			}
		} else if err == nil && fi.Mode()&os.ModeSymlink != 0 {
			// If target was somehow created as a symlink, remove it and replace with real dir
			_ = os.Remove(wtTarget)
			_ = os.MkdirAll(wtTarget, 0755)
		}
	}

	return nil
}

// LinkGlobalCompilerCache symlinks shared compiler cache directories (.cache, .sccache)
// from repoRoot or the host environment into targetDir if not already present.
func LinkGlobalCompilerCache(repoRoot, targetDir string) error {
	// 1. Check repo-level caches
	for _, cacheDirName := range []string{".cache", ".sccache"} {
		src := filepath.Join(repoRoot, cacheDirName)
		dst := filepath.Join(targetDir, cacheDirName)
		if fi, err := os.Stat(src); err == nil && fi.IsDir() {
			if _, err := os.Lstat(dst); os.IsNotExist(err) {
				_ = linkPath(src, dst)
			}
		}
	}

	// 2. Host-level sccache directory linking if sccache is used
	if hostHome, err := os.UserHomeDir(); err == nil && hostHome != "" {
		hostSccache := filepath.Join(hostHome, ".cache", "sccache")
		if fi, err := os.Stat(hostSccache); err == nil && fi.IsDir() {
			targetCache := filepath.Join(targetDir, ".cache")
			_ = os.MkdirAll(targetCache, 0755)
			targetSccache := filepath.Join(targetCache, "sccache")
			if _, err := os.Lstat(targetSccache); os.IsNotExist(err) {
				_ = linkPath(hostSccache, targetSccache)
			}
		}
	}

	return nil
}
