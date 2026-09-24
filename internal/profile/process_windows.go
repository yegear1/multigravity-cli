//go:build windows

package profile

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

func getProfilePIDsOS(name string) ([]int, error) {
	profileDir := config.GetProfileDir(name)
	dataDir := config.GetUserDataDir(profileDir)

	psScript := fmt.Sprintf(`Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -and ($_.CommandLine -like "*%s*" -or $_.CommandLine -like "*%s*") } | Select-Object -ExpandProperty ProcessId`, dataDir, profileDir)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	currentPID := os.Getpid()
	var pids []int
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if pid, err := strconv.Atoi(line); err == nil {
			if pid != currentPID {
				pids = append(pids, pid)
			}
		}
	}
	return pids, nil
}

func terminateProcessOS(pid int) error {
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid))
	return cmd.Run()
}

func killProcessOS(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
	return cmd.Run()
}
