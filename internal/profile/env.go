package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// BuildEnrichedPATH constructs an enriched PATH string containing host user binary directories.
func BuildEnrichedPATH(hostHome string) string {
	return BuildEnrichedPATHWithBase(os.Getenv("PATH"), hostHome)
}

// BuildEnrichedPATHWithBase constructs an enriched PATH string given a base PATH and host home directory.
// On Unix/WSL2, host tools and native Linux directories precede Windows mounts (/mnt/c/...).
func BuildEnrichedPATHWithBase(basePath, hostHome string) string {
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
			filepath.Join(hostHome, "AppData", "Local", "Programs", "Python"),
			filepath.Join(hostHome, "AppData", "Local", "Microsoft", "WinGet", "Links"),
			filepath.Join(hostHome, "AppData", "Local", "pnpm"),
			filepath.Join(hostHome, ".volta", "bin"),
			filepath.Join(hostHome, ".fnm", "current", "bin"),
		}
	} else {
		hostBinDirs = []string{
			filepath.Join(hostHome, ".local", "bin"),
			filepath.Join(hostHome, ".cargo", "bin"),
			filepath.Join(hostHome, ".bun", "bin"),
			filepath.Join(hostHome, "go", "bin"),
			filepath.Join(hostHome, ".local", "share", "pnpm"),
			filepath.Join(hostHome, ".fnm", "current", "bin"),
			filepath.Join(hostHome, ".volta", "bin"),
		}

		// NVM node versions discovery: ~/.nvm/versions/node/*/bin
		nvmPattern := filepath.Join(hostHome, ".nvm", "versions", "node", "*", "bin")
		if matches, err := filepath.Glob(nvmPattern); err == nil && len(matches) > 0 {
			sort.Slice(matches, func(i, j int) bool {
				return matches[i] > matches[j]
			})
			hostBinDirs = append(hostBinDirs, matches...)
		}
	}

	pathSep := string(os.PathListSeparator)
	seen := make(map[string]bool)
	var candidateParts []string

	for _, d := range hostBinDirs {
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
