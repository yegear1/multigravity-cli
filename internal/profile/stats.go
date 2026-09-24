package profile

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

// ProfileStat holds size and extension count stats for a profile
type ProfileStat struct {
	Name           string
	Size           string
	ExtensionCount int
}

// GetProfileStats gathers storage and extension counts for all profiles
func GetProfileStats() ([]ProfileStat, string, error) {
	base := config.GetMultigravityHome()
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil, "", nil
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, "", err
	}

	var stats []ProfileStat
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == ".templates" {
			continue
		}

		pDir := filepath.Join(base, name)
		size := GetDirSizeStr(pDir)

		extCount := 0
		extDir := config.GetExtensionsDir(pDir)
		if extEntries, err := os.ReadDir(extDir); err == nil {
			for _, ext := range extEntries {
				// Don't count hidden files or metadata
				if !ext.IsDir() && ext.Name() == ".obsolete" {
					continue
				}
				extCount++
			}
		}

		stats = append(stats, ProfileStat{
			Name:           name,
			Size:           size,
			ExtensionCount: extCount,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Name < stats[j].Name
	})

	totalSize := GetDirSizeStr(base)

	return stats, totalSize, nil
}
