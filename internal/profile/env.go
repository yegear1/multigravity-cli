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
