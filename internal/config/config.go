package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
)

const (
	Version = "2.0.0"

	SentinelIsolatedMCP      = ".isolated_mcp"
	SentinelIsolatedSkills   = ".isolated_skills"
	SentinelIsolatedConfig   = ".isolated_config"
	SentinelIsolatedGH       = ".isolated_gh"
	SentinelIsolatedDotfiles = ".isolated_dotfiles"
)

var profileNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*$`)

// ValidateProfileName asserts that the name complies with the invariant: ^[a-zA-Z0-9][a-zA-Z0-9-]*$
func ValidateProfileName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	if !profileNameRegex.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: must match regex ^[a-zA-Z0-9][a-zA-Z0-9-]*$", name)
	}
	return nil
}

// GetMultigravityHome returns the base profiles directory (~/AntigravityProfiles by default)
func GetMultigravityHome() string {
	if env := os.Getenv("MULTIGRAVITY_HOME"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, "AntigravityProfiles")
}

// GetProfileDir returns the absolute path to a profile's root directory
func GetProfileDir(name string) string {
	return filepath.Join(GetMultigravityHome(), name)
}

// GetUserDataDir returns the platform-specific user-data-dir for Antigravity
func GetUserDataDir(profileDir string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(profileDir, "Library", "Application Support", "Antigravity")
	case "windows":
		return filepath.Join(profileDir, "AppData", "Roaming", "Antigravity")
	default: // linux, freebsd, etc.
		return filepath.Join(profileDir, ".config", "Antigravity")
	}
}

// GetExtensionsDir returns the extensions directory for a profile
func GetExtensionsDir(profileDir string) string {
	return filepath.Join(profileDir, ".antigravity", "extensions")
}

// GetTemplatesDir returns the directory where profile templates are stored (~/AntigravityProfiles/.templates)
func GetTemplatesDir() string {
	return filepath.Join(GetMultigravityHome(), ".templates")
}

