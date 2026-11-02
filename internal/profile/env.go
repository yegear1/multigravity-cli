package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

// BuildEnrichedPATH constructs an enriched PATH string containing host user binary directories.
func BuildEnrichedPATH(hostHome string) string {
	return BuildEnrichedPATHForProfile(os.Getenv("PATH"), hostHome, "")
}

// BuildEnrichedPATHWithBase constructs an enriched PATH string given a base PATH and host home directory.
// On Unix/WSL2, host tools and native Linux directories precede Windows mounts (/mnt/c/...).
func BuildEnrichedPATHWithBase(basePath, hostHome string) string {
	return BuildEnrichedPATHForProfile(basePath, hostHome, "")
}

// BuildEnrichedPATHForProfile constructs an enriched PATH string given a base PATH, host home directory,
// and current profile name. It discovers host system tools and active peer profile toolchains (unless isolated).
func BuildEnrichedPATHForProfile(basePath, hostHome, currentProfile string) string {
	if hostHome == "" {
		hostHome = os.Getenv("REAL_HOME")
	}
	if hostHome == "" {
		hostHome, _ = os.UserHomeDir()
	}

	var hostBinDirs []string
	if runtime.GOOS == "windows" {
		hostBinDirs = []string{
			filepath.Join(hostHome, ".cargo", "bin"),
			filepath.Join(hostHome, ".local", "bin"),
			filepath.Join(hostHome, "go", "bin"),
			filepath.Join(hostHome, "AppData", "Local", "Programs", "Python"),
			filepath.Join(hostHome, "AppData", "Local", "Microsoft", "WinGet", "Links"),
			filepath.Join(hostHome, "AppData", "Local", "pnpm"),
			filepath.Join(hostHome, ".volta", "bin"),
			filepath.Join(hostHome, ".fnm", "current", "bin"),
			filepath.Join(hostHome, ".pyenv", "pyenv-win", "bin"),
			filepath.Join(hostHome, ".pyenv", "pyenv-win", "shims"),
		}
		if pnpmHome := os.Getenv("PNPM_HOME"); pnpmHome != "" {
			hostBinDirs = append(hostBinDirs, pnpmHome)
		}
		// Windows Python versions and Scripts:
		if matches, err := filepath.Glob(filepath.Join(hostHome, "AppData", "Local", "Programs", "Python", "Python*")); err == nil && len(matches) > 0 {
			sort.Slice(matches, func(i, j int) bool {
				return matches[i] > matches[j]
			})
			for _, m := range matches {
				hostBinDirs = append(hostBinDirs, m, filepath.Join(m, "Scripts"))
			}
		}
		appendGlobDirs(&hostBinDirs, filepath.Join(hostHome, ".rustup", "toolchains", "*", "bin"))
		appendGlobDirs(&hostBinDirs, filepath.Join(hostHome, "sdk", "go*", "bin"))
	} else {
		hostBinDirs = []string{
			filepath.Join(hostHome, ".local", "bin"),
			filepath.Join(hostHome, ".cargo", "bin"),
			filepath.Join(hostHome, ".bun", "bin"),
			filepath.Join(hostHome, "go", "bin"),
			filepath.Join(hostHome, ".local", "share", "pnpm"),
			filepath.Join(hostHome, ".pnpm"),
			filepath.Join(hostHome, ".fnm", "current", "bin"),
			filepath.Join(hostHome, ".volta", "bin"),
			filepath.Join(hostHome, ".nvm", "current", "bin"),
			filepath.Join(hostHome, ".asdf", "shims"),
			filepath.Join(hostHome, ".nodenv", "shims"),
			filepath.Join(hostHome, ".pyenv", "shims"),
			filepath.Join(hostHome, ".conda", "bin"),
			filepath.Join(hostHome, "miniconda3", "bin"),
			filepath.Join(hostHome, "anaconda3", "bin"),
			filepath.Join(hostHome, "mambaforge", "bin"),
			filepath.Join(hostHome, "miniforge3", "bin"),
			"/usr/local/go/bin",
			"/opt/go/bin",
			filepath.Join(hostHome, ".local", "go", "bin"),
			"/opt/homebrew/bin",
			"/usr/local/bin",
		}
		if pnpmHome := os.Getenv("PNPM_HOME"); pnpmHome != "" {
			hostBinDirs = append(hostBinDirs, pnpmHome)
		}

		// NVM node versions discovery: ~/.nvm/versions/node/*/bin
		appendGlobDirs(&hostBinDirs, filepath.Join(hostHome, ".nvm", "versions", "node", "*", "bin"))
		// asdf node versions: ~/.asdf/installs/nodejs/*/bin
		appendGlobDirs(&hostBinDirs, filepath.Join(hostHome, ".asdf", "installs", "nodejs", "*", "bin"))
		// nodenv node versions: ~/.nodenv/versions/*/bin
		appendGlobDirs(&hostBinDirs, filepath.Join(hostHome, ".nodenv", "versions", "*", "bin"))
		// Go SDKs: ~/sdk/go*/bin
		appendGlobDirs(&hostBinDirs, filepath.Join(hostHome, "sdk", "go*", "bin"))
		// Rustup toolchains: ~/.rustup/toolchains/*/bin
		appendGlobDirs(&hostBinDirs, filepath.Join(hostHome, ".rustup", "toolchains", "*", "bin"))
		// pyenv versions: ~/.pyenv/versions/*/bin
		appendGlobDirs(&hostBinDirs, filepath.Join(hostHome, ".pyenv", "versions", "*", "bin"))
		// asdf python versions: ~/.asdf/installs/python/*/bin
		appendGlobDirs(&hostBinDirs, filepath.Join(hostHome, ".asdf", "installs", "python", "*", "bin"))
		// uv python versions: ~/.local/share/uv/python/*/bin
		appendGlobDirs(&hostBinDirs, filepath.Join(hostHome, ".local", "share", "uv", "python", "*", "bin"))
	}

	var peerBinDirs []string
	isIsolated := false
	if currentProfile != "" {
		pDir := config.GetProfileDir(currentProfile)
		if hasSentinel(pDir, config.SentinelIsolatedToolchain) {
			isIsolated = true
		}
	}
	if !isIsolated {
		peerBinDirs = DiscoverPeerProfileToolchains(currentProfile)
	}

	pathSep := string(os.PathListSeparator)
	seen := make(map[string]bool)
	var candidateParts []string

	for _, d := range append(hostBinDirs, peerBinDirs...) {
		if d == "" {
			continue
		}
		clean := filepath.Clean(d)
		if seen[clean] {
			continue
		}
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			seen[clean] = true
			candidateParts = append(candidateParts, d)
		}
	}

	var existingParts []string
	if basePath != "" {
		for _, p := range strings.Split(basePath, pathSep) {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			clean := filepath.Clean(p)
			if !seen[clean] {
				seen[clean] = true
				existingParts = append(existingParts, p)
			}
		}
	}

	allParts := append(candidateParts, existingParts...)
	if len(allParts) == 0 {
		return ""
	}

	// On WSL2 / Linux, ensure Linux system and user paths precede any /mnt/c/ Windows paths
	if runtime.GOOS != "windows" {
		var linuxParts []string
		var windowsMountParts []string
		for _, p := range allParts {
			if isWindowsMountPath(p) {
				windowsMountParts = append(windowsMountParts, p)
			} else {
				linuxParts = append(linuxParts, p)
			}
		}
		allParts = append(linuxParts, windowsMountParts...)
	}

	return strings.Join(allParts, pathSep)
}

func appendGlobDirs(list *[]string, pattern string) {
	if matches, err := filepath.Glob(pattern); err == nil && len(matches) > 0 {
		sort.Slice(matches, func(i, j int) bool {
			return matches[i] > matches[j]
		})
		*list = append(*list, matches...)
	}
}

// DiscoverPeerProfileToolchains searches peer profiles in MULTIGRAVITY_HOME for local toolchains.
// Excludes currentProfile and any profile with .isolated_toolchain.
func DiscoverPeerProfileToolchains(currentProfile string) []string {
	base := config.GetMultigravityHome()
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}

	var peerDirs []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		pName := entry.Name()
		if currentProfile != "" && pName == currentProfile {
			continue
		}
		pDir := filepath.Join(base, pName)
		if hasSentinel(pDir, config.SentinelIsolatedToolchain) {
			continue
		}

		candidates := []string{
			filepath.Join(pDir, ".cargo", "bin"),
			filepath.Join(pDir, ".local", "bin"),
			filepath.Join(pDir, "go", "bin"),
			filepath.Join(pDir, ".bun", "bin"),
			filepath.Join(pDir, ".local", "share", "pnpm"),
		}
		if runtime.GOOS == "windows" {
			candidates = append(candidates,
				filepath.Join(pDir, "AppData", "Local", "pnpm"),
				filepath.Join(pDir, "AppData", "Local", "Programs", "Python"),
			)
			if pyMatches, err := filepath.Glob(filepath.Join(pDir, "AppData", "Local", "Programs", "Python", "Python*")); err == nil {
				for _, match := range pyMatches {
					candidates = append(candidates, match, filepath.Join(match, "Scripts"))
				}
			}
		}

		for _, cand := range candidates {
			if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
				peerDirs = append(peerDirs, cand)
			}
		}
	}
	return peerDirs
}

func isWindowsMountPath(p string) bool {
	if runtime.GOOS == "windows" {
		return false
	}
	clean := filepath.ToSlash(p)
	if strings.HasPrefix(clean, "/mnt/") && len(clean) >= 6 {
		drive := clean[5]
		isLetter := (drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z')
		if isLetter && (len(clean) == 6 || clean[6] == '/') {
			return true
		}
	}
	return false
}

// PopulateToolchainEnv ensures host toolchain environments (CARGO_HOME, RUSTUP_HOME, GOPATH)
// are explicitly preserved pointing to hostHome in the isolated profile environment if unset.
func PopulateToolchainEnv(envMap map[string]string, hostHome string) {
	if hostHome == "" {
		hostHome = os.Getenv("REAL_HOME")
	}
	if hostHome == "" {
		hostHome, _ = os.UserHomeDir()
	}
	if hostHome == "" {
		return
	}

	if envMap["CARGO_HOME"] == "" {
		envMap["CARGO_HOME"] = filepath.Join(hostHome, ".cargo")
	}
	if envMap["RUSTUP_HOME"] == "" {
		envMap["RUSTUP_HOME"] = filepath.Join(hostHome, ".rustup")
	}
	if envMap["GOPATH"] == "" {
		envMap["GOPATH"] = filepath.Join(hostHome, "go")
	}
}

// PopulateWorktreeProjectEnv enriches the agent environment with project-level runtime
// dependencies (.venv, node_modules) from baseRepoPath and points PYTHONPATH to worktreePath.
func PopulateWorktreeProjectEnv(envMap map[string]string, baseRepoPath, worktreePath string) {
	if envMap == nil || strings.TrimSpace(baseRepoPath) == "" || strings.TrimSpace(worktreePath) == "" {
		return
	}

	baseRepoPath = filepath.Clean(baseRepoPath)
	worktreePath = filepath.Clean(worktreePath)
	if baseRepoPath == worktreePath {
		return
	}

	// 1. Python venv from base repository
	venvDir := filepath.Join(baseRepoPath, ".venv")
	if fi, err := os.Stat(venvDir); err == nil && fi.IsDir() {
		if envMap["UV_PROJECT_ENVIRONMENT"] == "" {
			envMap["UV_PROJECT_ENVIRONMENT"] = venvDir
		}
		if envMap["VIRTUAL_ENV"] == "" {
			envMap["VIRTUAL_ENV"] = venvDir
		}

		binDirName := "bin"
		if runtime.GOOS == "windows" {
			binDirName = "Scripts"
		}
		venvBinDir := filepath.Join(venvDir, binDirName)
		if fiBin, err := os.Stat(venvBinDir); err == nil && fiBin.IsDir() {
			prependPathEntry(envMap, "PATH", venvBinDir)
		}

		srcDir := filepath.Join(worktreePath, "src")
		var pyDirs []string
		if fiSrc, err := os.Stat(srcDir); err == nil && fiSrc.IsDir() {
			pyDirs = []string{srcDir, worktreePath}
		} else {
			pyDirs = []string{worktreePath}
		}
		prependPathEntries(envMap, "PYTHONPATH", pyDirs)
	}

	// 2. Node modules from base repository
	nodeModulesDir := filepath.Join(baseRepoPath, "node_modules")
	if fi, err := os.Stat(nodeModulesDir); err == nil && fi.IsDir() {
		if envMap["NODE_PATH"] == "" {
			envMap["NODE_PATH"] = nodeModulesDir
		}
		nodeBinDir := filepath.Join(nodeModulesDir, ".bin")
		if fiBin, err := os.Stat(nodeBinDir); err == nil && fiBin.IsDir() {
			prependPathEntry(envMap, "PATH", nodeBinDir)
		}
	}
}

func prependPathEntry(envMap map[string]string, key, dir string) {
	if dir == "" {
		return
	}
	cur := envMap[key]
	if cur == "" {
		envMap[key] = dir
		return
	}
	sep := string(os.PathListSeparator)
	cleanDir := filepath.Clean(dir)
	for _, p := range strings.Split(cur, sep) {
		if filepath.Clean(p) == cleanDir {
			return
		}
	}
	envMap[key] = dir + sep + cur
}

func prependPathEntries(envMap map[string]string, key string, dirs []string) {
	if len(dirs) == 0 {
		return
	}
	sep := string(os.PathListSeparator)
	cur := envMap[key]
	if cur == "" {
		envMap[key] = strings.Join(dirs, sep)
		return
	}

	var existingParts []string
	cleanExisting := make(map[string]bool)
	for _, p := range strings.Split(cur, sep) {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		cleanExisting[filepath.Clean(p)] = true
		existingParts = append(existingParts, p)
	}

	var newParts []string
	for _, d := range dirs {
		if !cleanExisting[filepath.Clean(d)] {
			newParts = append(newParts, d)
		}
	}

	if len(newParts) == 0 {
		return
	}

	all := append(newParts, existingParts...)
	envMap[key] = strings.Join(all, sep)
}
