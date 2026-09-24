package shortcut

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateAndRemoveShortcutLinux(t *testing.T) {
	tmpDir := t.TempDir()
	customLinuxLauncherDir = filepath.Join(tmpDir, "launchers")
	customLinuxDesktopDir = filepath.Join(tmpDir, "applications")
	customSelfExecutable = "/usr/local/bin/multigravity"
	defer func() {
		customLinuxLauncherDir = ""
		customLinuxDesktopDir = ""
		customSelfExecutable = ""
	}()

	profile := "dev-work"
	if err := createShortcutLinux(profile); err != nil {
		t.Fatalf("failed to create linux shortcut: %v", err)
	}

	launcherPath := filepath.Join(customLinuxLauncherDir, profile+".sh")
	desktopPath := filepath.Join(customLinuxDesktopDir, "multigravity-"+profile+".desktop")

	launcherStat, err := os.Stat(launcherPath)
	if err != nil {
		t.Fatalf("launcher script not found: %v", err)
	}
	if launcherStat.Mode()&0111 == 0 {
		t.Errorf("launcher script is not executable: %v", launcherStat.Mode())
	}

	content, err := os.ReadFile(launcherPath)
	if err != nil {
		t.Fatalf("failed to read launcher script: %v", err)
	}
	if !strings.Contains(string(content), "/usr/local/bin/multigravity") {
		t.Errorf("expected launcher to reference binary, got:\n%s", string(content))
	}
	if !strings.Contains(string(content), `"dev-work"`) {
		t.Errorf("expected launcher to reference profile name, got:\n%s", string(content))
	}

	desktopContent, err := os.ReadFile(desktopPath)
	if err != nil {
		t.Fatalf("desktop file not found: %v", err)
	}
	if !strings.Contains(string(desktopContent), "[Desktop Entry]") {
		t.Errorf("invalid desktop file content:\n%s", string(desktopContent))
	}
	if !strings.Contains(string(desktopContent), "Multigravity dev-work") {
		t.Errorf("expected desktop name Multigravity dev-work, got:\n%s", string(desktopContent))
	}

	// Remove shortcut
	if err := removeShortcutLinux(profile); err != nil {
		t.Fatalf("failed to remove linux shortcut: %v", err)
	}

	if _, err := os.Stat(launcherPath); !os.IsNotExist(err) {
		t.Errorf("launcher script should be deleted")
	}
	if _, err := os.Stat(desktopPath); !os.IsNotExist(err) {
		t.Errorf("desktop file should be deleted")
	}
}

func TestCreateAndRemoveShortcutDarwin(t *testing.T) {
	tmpDir := t.TempDir()
	customMacAppDir = filepath.Join(tmpDir, "Applications")
	customSelfExecutable = "/usr/local/bin/multigravity"
	defer func() {
		customMacAppDir = ""
		customSelfExecutable = ""
	}()

	profile := "mac-dev"
	if err := createShortcutDarwin(profile); err != nil {
		t.Fatalf("failed to create darwin shortcut: %v", err)
	}

	appDir := filepath.Join(customMacAppDir, "Multigravity mac-dev.app")
	runScript := filepath.Join(appDir, "Contents", "MacOS", "run")
	infoPlist := filepath.Join(appDir, "Contents", "Info.plist")

	if _, err := os.Stat(runScript); err != nil {
		t.Fatalf("run script not found: %v", err)
	}
	if _, err := os.Stat(infoPlist); err != nil {
		t.Fatalf("Info.plist not found: %v", err)
	}

	// Remove shortcut
	if err := removeShortcutDarwin(profile); err != nil {
		t.Fatalf("failed to remove darwin shortcut: %v", err)
	}

	if _, err := os.Stat(appDir); !os.IsNotExist(err) {
		t.Errorf("app bundle should be deleted")
	}
}

func TestCreateShortcutInvalidName(t *testing.T) {
	if err := CreateShortcut("invalid name with spaces"); err == nil {
		t.Errorf("expected error for invalid profile name")
	}
}
