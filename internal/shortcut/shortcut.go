package shortcut

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

var (
	// customPaths for testing overrides
	customLinuxLauncherDir string
	customLinuxDesktopDir  string
	customMacAppDir        string
	customWinStartMenuDir  string
	customSelfExecutable   string
)

// SetCustomDirs allows tests to redirect shortcut generation to temporary directories
func SetCustomDirs(linuxLauncher, linuxDesktop, macApp, winStart string) {
	customLinuxLauncherDir = linuxLauncher
	customLinuxDesktopDir = linuxDesktop
	customMacAppDir = macApp
	customWinStartMenuDir = winStart
}

// SetCustomSelfExecutable overrides the executable path for testing
func SetCustomSelfExecutable(exe string) {
	customSelfExecutable = exe
}

// GetMultigravityBinary returns the current executable path or "multigravity"
func GetMultigravityBinary() string {
	if customSelfExecutable != "" {
		return customSelfExecutable
	}
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return "multigravity"
	}
	// If executable is a temporary test binary, fallback to "multigravity"
	if strings.Contains(exe, "/tmp/") || strings.Contains(exe, "\\Temp\\") {
		return "multigravity"
	}
	return exe
}

// CreateShortcut creates the desktop shortcut / launcher for the given profile
func CreateShortcut(profile string) error {
	if err := config.ValidateProfileName(profile); err != nil {
		return err
	}

	switch runtime.GOOS {
	case "darwin":
		return createShortcutDarwin(profile)
	case "windows":
		return createShortcutWindows(profile)
	default: // linux, freebsd, etc.
		return createShortcutLinux(profile)
	}
}

// RemoveShortcut removes the desktop shortcut / launcher for the given profile
func RemoveShortcut(profile string) error {
	switch runtime.GOOS {
	case "darwin":
		return removeShortcutDarwin(profile)
	case "windows":
		return removeShortcutWindows(profile)
	default:
		return removeShortcutLinux(profile)
	}
}

func linuxLauncherDir() string {
	if customLinuxLauncherDir != "" {
		return customLinuxLauncherDir
	}
	if env := os.Getenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR"); env != "" {
		return filepath.Join(env, "launchers")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "multigravity", "launchers")
}

func linuxDesktopDir() string {
	if customLinuxDesktopDir != "" {
		return customLinuxDesktopDir
	}
	if env := os.Getenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR"); env != "" {
		return filepath.Join(env, "applications")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "applications")
}

func macApplicationsDir() string {
	if customMacAppDir != "" {
		return customMacAppDir
	}
	if env := os.Getenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR"); env != "" {
		return filepath.Join(env, "Applications")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Applications")
}

func windowsStartMenuDir() string {
	if customWinStartMenuDir != "" {
		return customWinStartMenuDir
	}
	if env := os.Getenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR"); env != "" {
		return filepath.Join(env, "Programs")
	}
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs")
}

func createShortcutLinux(profile string) error {
	launcherRoot := linuxLauncherDir()
	desktopRoot := linuxDesktopDir()

	if err := os.MkdirAll(launcherRoot, 0755); err != nil {
		return fmt.Errorf("failed to create launcher directory: %w", err)
	}
	if err := os.MkdirAll(desktopRoot, 0755); err != nil {
		return fmt.Errorf("failed to create desktop entry directory: %w", err)
	}

	launcherPath := filepath.Join(launcherRoot, profile+".sh")
	desktopPath := filepath.Join(desktopRoot, fmt.Sprintf("multigravity-%s.desktop", profile))
	launcherBin := GetMultigravityBinary()
	baseDir := config.GetMultigravityHome()

	launcherContent := fmt.Sprintf(`#!/usr/bin/env sh
export MULTIGRAVITY_HOME=%q
exec %q %q "$@"
`, baseDir, launcherBin, profile)

	if err := os.WriteFile(launcherPath, []byte(launcherContent), 0755); err != nil {
		return fmt.Errorf("failed to create launcher script: %w", err)
	}

	desktopContent := fmt.Sprintf(`[Desktop Entry]
Version=1.0
Type=Application
Name=Multigravity %s
Comment=Launch the %s Antigravity profile
Exec=%q %%F
Icon=antigravity
Terminal=false
StartupNotify=false
StartupWMClass=Antigravity
Categories=TextEditor;Development;IDE;
MimeType=application/x-antigravity-workspace;
`, profile, profile, launcherPath)

	if err := os.WriteFile(desktopPath, []byte(desktopContent), 0644); err != nil {
		return fmt.Errorf("failed to create desktop entry: %w", err)
	}

	fmt.Printf("Shortcut created: %s\n", desktopPath)
	return nil
}

func removeShortcutLinux(profile string) error {
	launcherPath := filepath.Join(linuxLauncherDir(), profile+".sh")
	if _, err := os.Stat(launcherPath); err == nil {
		if err := os.Remove(launcherPath); err == nil {
			fmt.Printf("Removed shortcut: %s\n", launcherPath)
		}
	}

	desktopPath := filepath.Join(linuxDesktopDir(), fmt.Sprintf("multigravity-%s.desktop", profile))
	if _, err := os.Stat(desktopPath); err == nil {
		if err := os.Remove(desktopPath); err == nil {
			fmt.Printf("Removed shortcut: %s\n", desktopPath)
		}
	}
	return nil
}

func createShortcutDarwin(profile string) error {
	appDir := filepath.Join(macApplicationsDir(), fmt.Sprintf("Multigravity %s.app", profile))
	macosDir := filepath.Join(appDir, "Contents", "MacOS")
	resDir := filepath.Join(appDir, "Contents", "Resources")

	if err := os.MkdirAll(macosDir, 0755); err != nil {
		return err
	}
	_ = os.MkdirAll(resDir, 0755)

	runScript := filepath.Join(macosDir, "run")
	launcherBin := GetMultigravityBinary()
	baseDir := config.GetMultigravityHome()

	content := fmt.Sprintf(`#!/usr/bin/env bash
export MULTIGRAVITY_HOME=%q
exec %q %q "$@"
`, baseDir, launcherBin, profile)

	if err := os.WriteFile(runScript, []byte(content), 0755); err != nil {
		return err
	}

	plistPath := filepath.Join(appDir, "Contents", "Info.plist")
	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
"http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>run</string>
  <key>CFBundleIconFile</key>
  <string>icon</string>
  <key>CFBundleIdentifier</key>
  <string>com.multigravity.profile.%s</string>
  <key>CFBundleName</key>
  <string>Multigravity %s</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
</dict>
</plist>
`, profile, profile)

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return err
	}

	fmt.Printf("Shortcut created: %s\n", appDir)
	return nil
}

func removeShortcutDarwin(profile string) error {
	appDir := filepath.Join(macApplicationsDir(), fmt.Sprintf("Multigravity %s.app", profile))
	if _, err := os.Stat(appDir); err == nil {
		if err := os.RemoveAll(appDir); err == nil {
			fmt.Printf("Removed shortcut: %s\n", appDir)
		}
	}
	return nil
}
