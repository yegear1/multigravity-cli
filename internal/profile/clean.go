package profile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

// GetDirSizeStr returns a human-readable size of a directory (e.g., 42M, 1.2G)
func GetDirSizeStr(dir string) string {
	if runtime.GOOS != "windows" {
		cmd := exec.Command("du", "-sh", dir)
		if out, err := cmd.Output(); err == nil {
			fields := strings.Fields(string(out))
			if len(fields) > 0 {
				return fields[0]
			}
		}
	}

	var totalSize int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	return formatBytes(totalSize)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%c", float64(b)/float64(div), "KMGTPE"[exp])
}

// CleanSingleProfile cleans the caches of a single profile without touching user data or settings
func CleanSingleProfile(name string) (beforeSize, afterSize string, err error) {
	if err := config.ValidateProfileName(name); err != nil {
		return "", "", err
	}

	profileDir := config.GetProfileDir(name)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return "", "", fmt.Errorf("profile %q does not exist", name)
	}

	beforeSize = GetDirSizeStr(profileDir)
	userDataDir := config.GetUserDataDir(profileDir)

	cachePaths := []string{
		filepath.Join(userDataDir, "Cache"),
		filepath.Join(userDataDir, "Code Cache"),
		filepath.Join(userDataDir, "GPUCache"),
		filepath.Join(userDataDir, "DawnGraphiteCache"),
		filepath.Join(userDataDir, "DawnWebGPUCache"),
		filepath.Join(userDataDir, "Crashpad"),
		filepath.Join(userDataDir, "logs"),
		filepath.Join(userDataDir, "CachedData"),
		filepath.Join(userDataDir, "CachedExtensions"),
		filepath.Join(userDataDir, "CachedExtensionVSIXs"),
		filepath.Join(userDataDir, "Service Worker", "CacheStorage"),
		filepath.Join(userDataDir, "Service Worker", "ScriptCache"),
		filepath.Join(userDataDir, "webrtc_event_logs"),
		filepath.Join(profileDir, ".cache"),
		filepath.Join(profileDir, ".gemini", "antigravity", "crashes"),
		filepath.Join(profileDir, ".npm", "_cacache"),
		filepath.Join(profileDir, "Library", "Caches"),
		// Windows specific AppData paths
		filepath.Join(profileDir, "AppData", "Local", "Antigravity", "Cache"),
		filepath.Join(profileDir, "AppData", "Local", "Antigravity", "Code Cache"),
		filepath.Join(profileDir, "AppData", "Local", "Antigravity", "GPUCache"),
		filepath.Join(profileDir, "AppData", "Local", "Antigravity", "DawnGraphiteCache"),
		filepath.Join(profileDir, "AppData", "Local", "Antigravity", "DawnWebGPUCache"),
		filepath.Join(profileDir, "AppData", "Local", "Antigravity", "Crashpad"),
		filepath.Join(profileDir, "AppData", "Local", "Temp"),
	}

	for _, p := range cachePaths {
		_ = os.RemoveAll(p)
	}

	// Recreate empty volatile directories
	_ = os.MkdirAll(filepath.Join(profileDir, ".cache"), 0755)
	if runtime.GOOS == "windows" {
		_ = os.MkdirAll(filepath.Join(profileDir, "AppData", "Local", "Temp"), 0755)
	}

	afterSize = GetDirSizeStr(profileDir)
	return beforeSize, afterSize, nil
}

// CleanProfile cleans one profile or all profiles
func CleanProfile(target string) error {
	if target == "" {
		return fmt.Errorf("usage: multigravity clean <profile|--all>")
	}

	if target == "--all" {
		base := config.GetMultigravityHome()
		if _, err := os.Stat(base); os.IsNotExist(err) {
			fmt.Println("No profiles found.")
			return nil
		}

		names, err := ListProfiles()
		if err != nil {
			return err
		}

		if len(names) == 0 {
			fmt.Println("No profiles found.")
			return nil
		}

		count := 0
		for _, name := range names {
			if IsProfileRunning(name) {
				fmt.Printf("Skipping %q: profile is currently running\n", name)
				continue
			}

			before, after, err := CleanSingleProfile(name)
			if err != nil {
				fmt.Printf("Error cleaning profile %q: %v\n", name, err)
				continue
			}
			fmt.Printf("Cleaned cache for %q (%s -> %s)\n", name, before, after)
			count++
		}
		fmt.Printf("Cleaned %d profile(s).\n", count)
		return nil
	}

	if err := config.ValidateProfileName(target); err != nil {
		return err
	}

	profileDir := config.GetProfileDir(target)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", target)
	}

	if IsProfileRunning(target) {
		return fmt.Errorf("profile %q is currently running — stop it first (multigravity stop %s)", target, target)
	}

	before, after, err := CleanSingleProfile(target)
	if err != nil {
		return err
	}
	fmt.Printf("Cleaned cache for %q (%s -> %s)\n", target, before, after)
	return nil
}
