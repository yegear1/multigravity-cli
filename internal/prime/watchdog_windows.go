//go:build windows

package prime

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func getMultigravityExecutable() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		return exe, nil
	}
	path, err := exec.LookPath("multigravity.exe")
	if err == nil {
		return path, nil
	}
	return "multigravity.exe", nil
}

func InstallTask(profile string, include5h bool) error {
	script, _ := getMultigravityExecutable()
	taskName := fmt.Sprintf("MultigravityPrime-%s", profile)

	extraFlags := ""
	if include5h {
		extraFlags = "--5h "
	}
	tr := fmt.Sprintf(`"%s" prime "%s" %s--quiet`, script, profile, extraFlags)

	// schtasks /create /tn $taskName /tr $tr /sc hourly /mo 1 /f
	cmd := exec.Command("schtasks.exe", "/create", "/tn", taskName, "/tr", tr, "/sc", "hourly", "/mo", "1", "/f")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create scheduled task: %s: %w", string(out), err)
	}

	fmt.Printf("✓ Installed hourly Scheduled Task '%s'%s.\n", taskName, func() string {
		if include5h {
			return " (including 5-hour limits)"
		}
		return ""
	}())
	return nil
}

func UninstallTask(profile string) error {
	taskName := fmt.Sprintf("MultigravityPrime-%s", profile)
	cmd := exec.Command("schtasks.exe", "/delete", "/tn", taskName, "/f")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete scheduled task: %s: %w", string(out), err)
	}
	fmt.Printf("✓ Removed Scheduled Task '%s'.\n", taskName)
	return nil
}

func CheckWatchdogStatus(profile string) (cronDesc string, systemdDesc string) {
	taskName := fmt.Sprintf("MultigravityPrime-%s", profile)
	cmd := exec.Command("schtasks.exe", "/query", "/tn", taskName)
	if out, err := cmd.CombinedOutput(); err == nil && strings.Contains(string(out), taskName) {
		return fmt.Sprintf("Scheduled Task '%s' is Active", taskName), "N/A (Windows)"
	}
	return "Not scheduled", "N/A (Windows)"
}

func InstallCron(profile string, include5h bool) error {
	return InstallTask(profile, include5h)
}

func UninstallCron(profile string) error {
	return UninstallTask(profile)
}

func InstallSystemd(profile string, include5h bool) error {
	return fmt.Errorf("systemd is not available on Windows")
}

func UninstallSystemd(profile string) error {
	return fmt.Errorf("systemd is not available on Windows")
}
