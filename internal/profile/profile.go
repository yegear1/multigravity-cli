package profile

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

type ProfileInfo struct {
	Name      string
	Path      string
	IsRunning bool
	PIDs      []int
	Type      string // "full" or "shared"
	LastUsed  time.Time
	Size      string
}

// ListProfiles returns a sorted list of profile names
func ListProfiles() ([]string, error) {
	base := config.GetMultigravityHome()
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == ".templates" {
			continue
		}
		names = append(names, name)
	}

	sort.Strings(names)
	return names, nil
}

// GetProfiles returns detailed information about all profiles
func GetProfiles() ([]ProfileInfo, error) {
	names, err := ListProfiles()
	if err != nil {
		return nil, err
	}

	var list []ProfileInfo
	for _, name := range names {
		dir := config.GetProfileDir(name)
		pType := "full"
		if _, err := os.Stat(filepath.Join(dir, ".shared")); err == nil {
			pType = "shared"
		}

		pids, _ := GetProfilePIDs(name)
		isRunning := len(pids) > 0

		lastUsed := time.Time{}
		if stat, err := os.Stat(dir); err == nil {
			lastUsed = stat.ModTime()
		}

		size := getDirSizeStr(dir)

		list = append(list, ProfileInfo{
			Name:      name,
			Path:      dir,
			IsRunning: isRunning,
			PIDs:      pids,
			Type:      pType,
			LastUsed:  lastUsed,
			Size:      size,
		})
	}

	return list, nil
}

// GetProfilePIDs checks for running processes associated with the profile
func GetProfilePIDs(name string) ([]int, error) {
	profileDir := config.GetProfileDir(name)
	dataDir := filepath.Join(profileDir, "data")

	// Use ps to locate processes matching --user-data-dir
	cmd := exec.Command("ps", "-eo", "pid,args")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	currentPID := os.Getpid()
	var pids []int

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "grep") {
			continue
		}

		if strings.Contains(line, dataDir) || strings.Contains(line, profileDir) {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				pid, err := strconv.Atoi(parts[0])
				if err == nil && pid != currentPID {
					pids = append(pids, pid)
				}
			}
		}
	}

	return pids, nil
}

// IsProfileRunning returns true if the profile has active processes
func IsProfileRunning(name string) bool {
	pids, err := GetProfilePIDs(name)
	return err == nil && len(pids) > 0
}

func getDirSizeStr(dir string) string {
	cmd := exec.Command("du", "-sh", dir)
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	fields := strings.Fields(string(out))
	if len(fields) > 0 {
		return fields[0]
	}
	return "unknown"
}
