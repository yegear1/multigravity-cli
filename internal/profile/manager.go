package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/shortcut"
)

// CreateOptions defines options for creating a new profile
type CreateOptions struct {
	Name             string
	Shared           bool
	IsolatedDotfiles bool
	IsolatedMCP      bool
	IsolatedSkills   bool
	IsolatedConfig   bool
	IsolatedGH       bool
	Color            string
	FromTemplate     string
}

// CreateProfile creates a new profile directory and layout
func CreateProfile(opts CreateOptions) error {
	if err := config.ValidateProfileName(opts.Name); err != nil {
		return err
	}

	if opts.Color != "" {
		if _, _, err := ResolveColor(opts.Color); err != nil {
			return err
		}
	}

	profileDir := config.GetProfileDir(opts.Name)
	if _, err := os.Stat(profileDir); err == nil {
		return fmt.Errorf("profile %q already exists", opts.Name)
	}

	baseDir := config.GetMultigravityHome()
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base profiles directory: %w", err)
	}

	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return fmt.Errorf("failed to create profile directory: %w", err)
	}

	// Create sentinel files if isolation flags are specified
	if opts.IsolatedDotfiles {
		_ = touchFile(filepath.Join(profileDir, config.SentinelIsolatedDotfiles))
	}
	if opts.IsolatedMCP {
		_ = touchFile(filepath.Join(profileDir, config.SentinelIsolatedMCP))
	}
	if opts.IsolatedSkills {
		_ = touchFile(filepath.Join(profileDir, config.SentinelIsolatedSkills))
	}
	if opts.IsolatedConfig {
		_ = touchFile(filepath.Join(profileDir, config.SentinelIsolatedConfig))
	}
	if opts.IsolatedGH {
		_ = touchFile(filepath.Join(profileDir, config.SentinelIsolatedGH))
	}
	if opts.Shared {
		_ = touchFile(filepath.Join(profileDir, ".shared"))
	}

	if err := EnsureProfileLayout(profileDir); err != nil {
		return fmt.Errorf("failed to configure profile layout: %w", err)
	}

	if opts.Color != "" {
		_ = ApplyProfileColor(opts.Name, opts.Color, false)
	}

	_ = shortcut.CreateShortcut(opts.Name)

	return nil
}

// DeleteProfile deletes a profile and all its data
func DeleteProfile(name string, force bool) error {
	if err := config.ValidateProfileName(name); err != nil {
		return err
	}

	profileDir := config.GetProfileDir(name)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", name)
	}

	if IsProfileRunning(name) {
		if !force {
			return fmt.Errorf("cannot delete profile %q because it is currently running. Stop it first with: multigravity stop %s (or pass --force)", name, name)
		}

		// Force stop processes
		if err := StopProfile(name, true); err != nil {
			return fmt.Errorf("failed to stop running profile %q: %w", name, err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	_ = shortcut.RemoveShortcut(name)

	if err := os.RemoveAll(profileDir); err != nil {
		return fmt.Errorf("failed to remove profile directory: %w", err)
	}

	return nil
}

// RenameProfile renames an existing profile
func RenameProfile(oldName, newName string) error {
	if err := config.ValidateProfileName(oldName); err != nil {
		return fmt.Errorf("invalid source name: %w", err)
	}
	if err := config.ValidateProfileName(newName); err != nil {
		return fmt.Errorf("invalid destination name: %w", err)
	}

	oldDir := config.GetProfileDir(oldName)
	newDir := config.GetProfileDir(newName)

	if _, err := os.Stat(oldDir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", oldName)
	}
	if _, err := os.Stat(newDir); err == nil {
		return fmt.Errorf("profile %q already exists", newName)
	}

	if IsProfileRunning(oldName) {
		return fmt.Errorf("cannot rename profile %q because it is currently running. Stop it first with: multigravity stop %s", oldName, oldName)
	}

	_ = shortcut.RemoveShortcut(oldName)

	if err := os.Rename(oldDir, newDir); err != nil {
		return fmt.Errorf("failed to rename profile directory: %w", err)
	}

	_ = shortcut.CreateShortcut(newName)

	return nil
}

func touchFile(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}
