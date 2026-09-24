//go:build !windows

package chat

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getOpenConversationsOS(convDir string) (map[string]bool, error) {
	if _, err := os.Stat("/proc"); err == nil {
		res, err := scanProcFD(convDir)
		if err == nil {
			return res, nil
		}
	}
	return scanLsof(convDir)
}

func scanProcFD(convDir string) (map[string]bool, error) {
	procDir, err := os.Open("/proc")
	if err != nil {
		return nil, err
	}
	defer procDir.Close()

	entries, err := procDir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	cleanConvDir := filepath.Clean(convDir)
	result := make(map[string]bool)

	for _, entry := range entries {
		// Only inspect numeric dirs (PIDs)
		if len(entry) == 0 || entry[0] < '0' || entry[0] > '9' {
			continue
		}

		fdDir := filepath.Join("/proc", entry, "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}

		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			cleanTarget := filepath.Clean(target)
			if strings.HasPrefix(cleanTarget, cleanConvDir+string(filepath.Separator)) {
				base := filepath.Base(cleanTarget)
				uuid := extractUUIDFromDBName(base)
				if uuid != "" {
					result[uuid] = true
				}
			}
		}
	}
	return result, nil
}

func scanLsof(convDir string) (map[string]bool, error) {
	cmd := exec.Command("lsof", "-Fn", "+D", convDir)
	out, err := cmd.Output()
	if err != nil {
		return make(map[string]bool), nil
	}

	result := make(map[string]bool)
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "n") {
			filePath := strings.TrimPrefix(line, "n")
			base := filepath.Base(filePath)
			uuid := extractUUIDFromDBName(base)
			if uuid != "" {
				result[uuid] = true
			}
		}
	}
	return result, nil
}
