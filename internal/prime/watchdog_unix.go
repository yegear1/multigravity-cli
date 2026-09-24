//go:build !windows

package prime

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getMultigravityExecutable() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		return exe, nil
	}
	path, err := exec.LookPath("multigravity")
	if err == nil {
		return path, nil
	}
	return "", fmt.Errorf("could not determine multigravity executable path")
}

func InstallCron(profile string, include5h bool) error {
	script, err := getMultigravityExecutable()
	if err != nil {
		return err
	}

	userHome, _ := os.UserHomeDir()
	if realHome := os.Getenv("REAL_HOME"); realHome != "" {
		userHome = realHome
	}

	logDir := filepath.Join(userHome, ".local", "share", "multigravity")
	_ = os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, "prime.log")

	cronTag := fmt.Sprintf("# multigravity-prime:%s", profile)
	extraFlags := ""
	if include5h {
		extraFlags = "--5h "
	}
	cronLine := fmt.Sprintf("15 * * * * \"%s\" prime \"%s\" %s--quiet >> \"%s\" 2>&1", script, profile, extraFlags, logFile)

	curOut, _ := exec.Command("crontab", "-l").Output()
	curCron := string(curOut)

	var filtered []string
	for _, l := range strings.Split(curCron, "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.Contains(l, cronTag) || strings.Contains(l, fmt.Sprintf("prime \"%s\"", profile)) || strings.Contains(l, fmt.Sprintf("prime %s", profile)) {
			continue
		}
		filtered = append(filtered, l)
	}

	filtered = append(filtered, cronTag, cronLine, "")
	newCron := strings.Join(filtered, "\n")

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCron)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("crontab error: %s: %w", string(out), err)
	}

	fmt.Printf("✓ Installed hourly cron job for profile '%s'.\n", profile)
	fmt.Printf("  Schedule: Every hour at minute 15%s\n", func() string {
		if include5h {
			return ", including 5-hour limits"
		}
		return ""
	}())
	fmt.Printf("  Log file: %s\n", logFile)
	return nil
}

func UninstallCron(profile string) error {
	cronTag := fmt.Sprintf("# multigravity-prime:%s", profile)
	curOut, _ := exec.Command("crontab", "-l").Output()
	curCron := string(curOut)
	if curCron == "" || !strings.Contains(curCron, cronTag) {
		fmt.Printf("No cron job found for profile '%s'.\n", profile)
		return nil
	}

	var filtered []string
	for _, l := range strings.Split(curCron, "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.Contains(l, cronTag) || strings.Contains(l, fmt.Sprintf("prime \"%s\"", profile)) || strings.Contains(l, fmt.Sprintf("prime %s", profile)) {
			continue
		}
		filtered = append(filtered, l)
	}

	if len(filtered) == 0 {
		_ = exec.Command("crontab", "-r").Run()
	} else {
		newCron := strings.Join(filtered, "\n") + "\n"
		cmd := exec.Command("crontab", "-")
		cmd.Stdin = strings.NewReader(newCron)
		_ = cmd.Run()
	}

	fmt.Printf("✓ Removed cron job for profile '%s'.\n", profile)
	return nil
}

func InstallSystemd(profile string, include5h bool) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemd is not available on this system")
	}

	script, err := getMultigravityExecutable()
	if err != nil {
		return err
	}

	userHome, _ := os.UserHomeDir()
	if realHome := os.Getenv("REAL_HOME"); realHome != "" {
		userHome = realHome
	}

	unitDir := filepath.Join(userHome, ".config", "systemd", "user")
	_ = os.MkdirAll(unitDir, 0755)

	svcFile := filepath.Join(unitDir, fmt.Sprintf("multigravity-prime-%s.service", profile))
	timerFile := filepath.Join(unitDir, fmt.Sprintf("multigravity-prime-%s.timer", profile))

	extraFlags := ""
	if include5h {
		extraFlags = "--5h "
	}

	svcContent := fmt.Sprintf(`[Unit]
Description=Multigravity Quota Auto-Prime for profile %s
After=network-online.target

[Service]
Type=oneshot
ExecStart="%s" prime "%s" %s--quiet
`, profile, script, profile, extraFlags)

	timerContent := fmt.Sprintf(`[Unit]
Description=Hourly timer for Multigravity Auto-Prime (%s)

[Timer]
OnCalendar=*-*-* *:15:00
Persistent=true

[Install]
WantedBy=timers.target
`, profile)

	if err := os.WriteFile(svcFile, []byte(svcContent), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(timerFile, []byte(timerContent), 0644); err != nil {
		return err
	}

	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	timerName := fmt.Sprintf("multigravity-prime-%s.timer", profile)
	cmd := exec.Command("systemctl", "--user", "enable", "--now", timerName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable systemd timer: %s: %w", string(out), err)
	}

	fmt.Printf("✓ Installed and activated systemd timer '%s'%s.\n", timerName, func() string {
		if include5h {
			return " (including 5-hour limits)"
		}
		return ""
	}())
	return nil
}

func UninstallSystemd(profile string) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemd is not available on this system")
	}

	timerName := fmt.Sprintf("multigravity-prime-%s.timer", profile)
	_ = exec.Command("systemctl", "--user", "disable", "--now", timerName).Run()

	userHome, _ := os.UserHomeDir()
	if realHome := os.Getenv("REAL_HOME"); realHome != "" {
		userHome = realHome
	}
	unitDir := filepath.Join(userHome, ".config", "systemd", "user")
	_ = os.Remove(filepath.Join(unitDir, fmt.Sprintf("multigravity-prime-%s.service", profile)))
	_ = os.Remove(filepath.Join(unitDir, timerFile(profile)))
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

	fmt.Printf("✓ Removed systemd service and timer for profile '%s'.\n", profile)
	return nil
}

func timerFile(profile string) string {
	return fmt.Sprintf("multigravity-prime-%s.timer", profile)
}

func CheckWatchdogStatus(profile string) (cronDesc string, systemdDesc string) {
	cronDesc = "Not installed"
	systemdDesc = "Not active / Not installed"

	crOut, err := exec.Command("crontab", "-l").Output()
	if err == nil {
		crStr := string(crOut)
		if strings.Contains(crStr, fmt.Sprintf("multigravity-prime:%s", profile)) ||
			strings.Contains(crStr, fmt.Sprintf("prime \"%s\"", profile)) ||
			strings.Contains(crStr, fmt.Sprintf("prime %s", profile)) {
			has5h := false
			for _, line := range strings.Split(crStr, "\n") {
				if strings.Contains(line, profile) && (strings.Contains(line, "--5h") || strings.Contains(line, "--include-5h")) {
					has5h = true
					break
				}
			}
			if has5h {
				cronDesc = "Installed (Hourly, Weekly + 5h)"
			} else {
				cronDesc = "Installed (Hourly, Weekly only)"
			}
		}
	}

	timerName := fmt.Sprintf("multigravity-prime-%s.timer", profile)
	sysOut, err := exec.Command("systemctl", "--user", "is-active", timerName).Output()
	if err == nil && strings.TrimSpace(string(sysOut)) == "active" {
		userHome, _ := os.UserHomeDir()
		if realHome := os.Getenv("REAL_HOME"); realHome != "" {
			userHome = realHome
		}
		svPath := filepath.Join(userHome, ".config", "systemd", "user", fmt.Sprintf("multigravity-prime-%s.service", profile))
		has5h := false
		if sBytes, err := os.ReadFile(svPath); err == nil && bytes.Contains(sBytes, []byte("--5h")) {
			has5h = true
		}
		if has5h {
			systemdDesc = "Active (Weekly + 5h)"
		} else {
			systemdDesc = "Active (Weekly only)"
		}
	}

	return cronDesc, systemdDesc
}
