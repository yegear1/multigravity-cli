package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

type ProfileInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	IsRunning bool      `json:"is_running"`
	PIDs      []int     `json:"pids,omitempty"`
	Type      string    `json:"type"` // "full" or "shared"
	LastUsed  time.Time `json:"last_used"`
	Size      string    `json:"size"`
	Color     string    `json:"color,omitempty"`
}

// ProfileExists returns whether a profile exists
func ProfileExists(name string) bool {
	dir := config.GetProfileDir(name)
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
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

// GetProfile returns detailed information about a single profile
func GetProfile(name string) (*ProfileInfo, error) {
	if err := config.ValidateProfileName(name); err != nil {
		return nil, err
	}
	dir := config.GetProfileDir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", name)
	}

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

	size := GetDirSizeStr(dir)
	pColor, _ := GetProfileColor(name)

	return &ProfileInfo{
		Name:      name,
		Path:      dir,
		IsRunning: isRunning,
		PIDs:      pids,
		Type:      pType,
		LastUsed:  lastUsed,
		Size:      size,
		Color:     pColor,
	}, nil
}

// GetProfiles returns detailed information about all profiles
func GetProfiles() ([]ProfileInfo, error) {
	names, err := ListProfiles()
	if err != nil {
		return nil, err
	}

	var list []ProfileInfo
	for _, name := range names {
		info, err := GetProfile(name)
		if err != nil {
			continue
		}
		list = append(list, *info)
	}

	return list, nil
}

