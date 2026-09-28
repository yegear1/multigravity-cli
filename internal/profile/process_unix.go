//go:build !windows

package profile

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

func parsePIDsFromPSOutput(out string, profileDir, dataDir string, currentPID, parentPID int) []int {
	var pids []int
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "grep") {
			continue
		}

		if lineMatchesPath(line, dataDir) || lineMatchesPath(line, profileDir) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				pid, err1 := strconv.Atoi(parts[0])
				ppid, err2 := strconv.Atoi(parts[1])
				if err1 == nil && err2 == nil {
					// Invariant: ignore self and parent process to prevent killing our own shell/process
					if pid != currentPID && pid != parentPID && ppid != currentPID {
						pids = append(pids, pid)
					}
				}
			}
		}
	}
	return pids
}

func getProfilePIDsOS(name string) ([]int, error) {
	profileDir := config.GetProfileDir(name)
	dataDir := config.GetUserDataDir(profileDir)

	cmd := exec.Command("ps", "-eo", "pid,ppid,args")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parsePIDsFromPSOutput(string(out), profileDir, dataDir, os.Getpid(), os.Getppid()), nil
}

func terminateProcessOS(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGTERM)
}

func killProcessOS(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGKILL)
}
