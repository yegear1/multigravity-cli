//go:build windows

package shortcut

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ye-dev/multigravity-cli/internal/app"
)

func createShortcutWindows(profile string) error {
	dir := windowsStartMenuDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	lnkPath := filepath.Join(dir, fmt.Sprintf("Multigravity %s.lnk", profile))
	exe := GetMultigravityBinary()

	iconLocation := ""
	if icoPath, err := EnsureWindowsIcon(); err == nil && icoPath != "" {
		iconLocation = icoPath + ", 0"
	}
	if iconLocation == "" {
		if appPath, err := app.FindApp(); err == nil {
			iconLocation = appPath + ", 0"
		}
	}

	psScript := fmt.Sprintf(`$WshShell = New-Object -ComObject WScript.Shell
$Shortcut = $WshShell.CreateShortcut(%q)
$Shortcut.TargetPath = %q
$Shortcut.Arguments = %q
if (%q -ne "") {
    $Shortcut.IconLocation = %q
}
$Shortcut.Save()`, lnkPath, exe, profile, iconLocation, iconLocation)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create Windows shortcut: %w", err)
	}

	fmt.Printf("Shortcut created: %s\n", lnkPath)
	return nil
}

func removeShortcutWindows(profile string) error {
	dir := windowsStartMenuDir()
	lnkPath := filepath.Join(dir, fmt.Sprintf("Multigravity %s.lnk", profile))
	if _, err := os.Stat(lnkPath); err == nil {
		if err := os.Remove(lnkPath); err == nil {
			fmt.Printf("Removed shortcut: %s\n", lnkPath)
		}
	}
	return nil
}
