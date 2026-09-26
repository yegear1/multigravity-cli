package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ErrAppNotFound is returned when neither Antigravity nor agy can be located
var ErrAppNotFound = errors.New("antigravity executable not found")

// FindApp detects the path to the Antigravity or Agy executable
func FindApp() (string, error) {
	// 1. Check environment variable overrides
	override := os.Getenv("MULTIGRAVITY_APP")
	if override == "" {
		override = os.Getenv("AGY_APP")
	}
	if override != "" {
		if isExecutableOrApp(override) {
			return override, nil
		}
		if path, err := exec.LookPath(override); err == nil {
			return path, nil
		}
		return "", fmt.Errorf("configured app executable not found: %s", override)
	}

	userHome, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		candidates := []string{
			"/Applications/Antigravity.app",
			filepath.Join(userHome, "Applications", "Antigravity.app"),
			"/Applications/Agy.app",
			filepath.Join(userHome, "Applications", "Agy.app"),
			"antigravity",
			"agy",
			"/usr/local/bin/agy",
			filepath.Join(userHome, ".local", "bin", "agy"),
		}
		for _, c := range candidates {
			if isExecutableOrApp(c) {
				return c, nil
			}
			if path, err := exec.LookPath(c); err == nil {
				return path, nil
			}
		}

	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		progFiles := os.Getenv("PROGRAMFILES")
		progFilesX86 := os.Getenv("ProgramFiles(x86)")
		userProfile := os.Getenv("USERPROFILE")
		if userProfile == "" {
			userProfile = userHome
		}

		candidates := []string{
			filepath.Join(localAppData, "Programs", "Antigravity", "Antigravity.exe"),
			filepath.Join(localAppData, "Programs", "antigravity", "antigravity.exe"),
			filepath.Join(progFiles, "Antigravity", "Antigravity.exe"),
			filepath.Join(progFilesX86, "Antigravity", "Antigravity.exe"),
			filepath.Join(localAppData, "Programs", "agy", "agy.exe"),
			filepath.Join(progFiles, "agy", "agy.exe"),
			filepath.Join(userProfile, "scoop", "apps", "antigravity", "current", "antigravity.exe"),
			filepath.Join(userProfile, "scoop", "apps", "agy", "current", "agy.exe"),
		}
		for _, c := range candidates {
			if isExecutableOrApp(c) {
				return c, nil
			}
		}
		if path, err := exec.LookPath("antigravity.exe"); err == nil {
			return path, nil
		}
		if path, err := exec.LookPath("agy.exe"); err == nil {
			return path, nil
		}

	default: // linux, freebsd, etc.
		// Check PATH first
		if path, err := exec.LookPath("antigravity"); err == nil {
			return path, nil
		}
		if path, err := exec.LookPath("agy"); err == nil {
			return path, nil
		}

		candidates := []string{
			filepath.Join(userHome, "apps", "antigravity", "antigravity"),
			filepath.Join(userHome, "apps", "antigravity", "bin", "antigravity"),
			filepath.Join(userHome, "apps", "agy", "agy"),
			filepath.Join(userHome, "apps", "agy", "bin", "agy"),
			filepath.Join(userHome, ".local", "share", "antigravity", "antigravity"),
			"/opt/antigravity/antigravity",
			"/opt/antigravity/bin/antigravity",
			"/opt/agy/agy",
			"/opt/agy/bin/agy",
			"/usr/share/antigravity/antigravity",
			"/usr/bin/antigravity",
			"/usr/local/bin/antigravity",
			filepath.Join(userHome, ".local", "bin", "antigravity"),
			"/usr/bin/agy",
			"/usr/local/bin/agy",
			filepath.Join(userHome, ".local", "bin", "agy"),
			filepath.Join(userHome, "Applications", "Antigravity.AppImage"),
		}
		for _, c := range candidates {
			if isExecutableOrApp(c) {
				return c, nil
			}
		}
	}

	return "", ErrAppNotFound
}

// RequireApp returns the app path or an explanatory error
func RequireApp() (string, error) {
	app, err := FindApp()
	if err != nil {
		switch runtime.GOOS {
		case "darwin":
			return "", fmt.Errorf("Antigravity.app (or 'agy') was not found. Install Antigravity or set MULTIGRAVITY_APP to the app bundle path")
		case "windows":
			return "", fmt.Errorf("Antigravity.exe (or agy.exe) not found. Install Antigravity or set MULTIGRAVITY_APP")
		default:
			return "", fmt.Errorf("Antigravity (or 'agy') was not found. Install the Linux app so 'antigravity' or 'agy' is on PATH, or set MULTIGRAVITY_APP to the executable path")
		}
	}
	return app, nil
}

// FindLanguageServer detects the path to the internal language_server binary
func FindLanguageServer() (string, error) {
	app, _ := FindApp()
	if app != "" {
		dir := filepath.Dir(app)
		binName := "language_server"
		if runtime.GOOS == "windows" {
			binName = "language_server.exe"
		}

		cands := []string{
			filepath.Join(dir, "resources", "bin", binName),
			filepath.Join(dir, "..", "Resources", "bin", binName),
		}
		for _, c := range cands {
			if isExecutableOrApp(c) {
				return c, nil
			}
		}
	}

	userHome, _ := os.UserHomeDir()
	binName := "language_server"
	if runtime.GOOS == "windows" {
		binName = "language_server.exe"
		localAppData := os.Getenv("LOCALAPPDATA")
		progFiles := os.Getenv("PROGRAMFILES")
		cands := []string{
			filepath.Join(localAppData, "Programs", "Antigravity", "resources", "bin", binName),
			filepath.Join(localAppData, "Programs", "antigravity", "resources", "bin", binName),
			filepath.Join(progFiles, "Antigravity", "resources", "bin", binName),
			filepath.Join(localAppData, "Programs", "agy", "resources", "bin", binName),
			filepath.Join(progFiles, "agy", "resources", "bin", binName),
		}
		for _, c := range cands {
			if isExecutableOrApp(c) {
				return c, nil
			}
		}
	} else {
		cands := []string{
			filepath.Join(userHome, "apps", "antigravity", "resources", "bin", binName),
			filepath.Join(userHome, "apps", "agy", "resources", "bin", binName),
			filepath.Join(userHome, ".local", "share", "antigravity", "resources", "bin", binName),
			filepath.Join("/opt", "antigravity", "resources", "bin", binName),
			filepath.Join("/opt", "agy", "resources", "bin", binName),
			filepath.Join("/usr", "share", "antigravity", "resources", "bin", binName),
		}
		for _, c := range cands {
			if isExecutableOrApp(c) {
				return c, nil
			}
		}
	}

	return "", errors.New("language_server binary not found")
}

// FindAgy detects the path to the agy CLI binary
func FindAgy() (string, error) {
	if override := os.Getenv("AGY_BIN"); override != "" {
		if isExecutableOrApp(override) {
			return override, nil
		}
		if path, err := exec.LookPath(override); err == nil {
			return path, nil
		}
	}

	userHome, _ := os.UserHomeDir()
	binName := "agy"
	if runtime.GOOS == "windows" {
		binName = "agy.exe"
	}

	if path, err := exec.LookPath(binName); err == nil {
		return path, nil
	}

	switch runtime.GOOS {
	case "darwin":
		candidates := []string{
			"/usr/local/bin/agy",
			filepath.Join(userHome, ".local", "bin", "agy"),
			"/opt/homebrew/bin/agy",
			"/Applications/Agy.app/Contents/MacOS/agy",
		}
		for _, c := range candidates {
			if isExecutableOrApp(c) {
				return c, nil
			}
		}
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		progFiles := os.Getenv("PROGRAMFILES")
		userProfile := os.Getenv("USERPROFILE")
		if userProfile == "" {
			userProfile = userHome
		}
		candidates := []string{
			filepath.Join(localAppData, "Programs", "agy", "agy.exe"),
			filepath.Join(progFiles, "agy", "agy.exe"),
			filepath.Join(userProfile, "scoop", "apps", "agy", "current", "agy.exe"),
			filepath.Join(userHome, ".local", "bin", "agy.exe"),
		}
		for _, c := range candidates {
			if isExecutableOrApp(c) {
				return c, nil
			}
		}
	default:
		candidates := []string{
			filepath.Join(userHome, ".local", "bin", "agy"),
			"/usr/local/bin/agy",
			"/usr/bin/agy",
			"/opt/agy/bin/agy",
			"/opt/agy/agy",
			filepath.Join(userHome, "apps", "agy", "bin", "agy"),
			filepath.Join(userHome, "apps", "agy", "agy"),
		}
		for _, c := range candidates {
			if isExecutableOrApp(c) {
				return c, nil
			}
		}
	}

	return "", errors.New("agy binary not found")
}

func isExecutableOrApp(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	if info.IsDir() {
		// On macOS, .app is a directory bundle
		if runtime.GOOS == "darwin" && filepath.Ext(path) == ".app" {
			return true
		}
		return false
	}

	// On Windows, executable extension check
	if runtime.GOOS == "windows" {
		ext := filepath.Ext(path)
		return ext == ".exe" || ext == ".cmd" || ext == ".bat"
	}

	// On Unix, check executable bits
	return info.Mode()&0111 != 0
}
