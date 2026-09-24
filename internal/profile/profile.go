package profile

import (
	"os"
	"path/filepath"
	"sort"
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
	Color     string
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

		size := GetDirSizeStr(dir)
		pColor, _ := GetProfileColor(name)

		list = append(list, ProfileInfo{
			Name:      name,
			Path:      dir,
			IsRunning: isRunning,
			PIDs:      pids,
			Type:      pType,
			LastUsed:  lastUsed,
			Size:      size,
			Color:     pColor,
		})
	}

	return list, nil
}
