package profile

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

var hexColorRegex = regexp.MustCompile(`^#?([0-9a-fA-F]{6})$`)

var colorPalette = map[string][2]string{
	"blue":    {"#1e40af", "#172554"},
	"navy":    {"#1e3a8a", "#0f172a"},
	"green":   {"#166534", "#14532d"},
	"emerald": {"#065f46", "#064e3b"},
	"teal":    {"#115e59", "#134e4a"},
	"cyan":    {"#155e75", "#164e63"},
	"red":     {"#991b1b", "#7f1d1d"},
	"rose":    {"#9f1239", "#881337"},
	"purple":  {"#6b21a8", "#581c87"},
	"violet":  {"#5b21b6", "#4c1d95"},
	"indigo":  {"#3730a3", "#312e81"},
	"pink":    {"#9d174d", "#831843"},
	"orange":  {"#9a3412", "#7c2d12"},
	"amber":   {"#854d0e", "#713f12"},
	"yellow":  {"#854d0e", "#713f12"},
	"slate":   {"#334155", "#1e293b"},
	"gray":    {"#334155", "#1e293b"},
	"grey":    {"#334155", "#1e293b"},
}

// ResolveColor resolves a friendly color name or hex code into primary and secondary colors
func ResolveColor(colorArg string) (primary, secondary string, err error) {
	clean := strings.ToLower(strings.TrimSpace(colorArg))
	if pair, ok := colorPalette[clean]; ok {
		return pair[0], pair[1], nil
	}

	matches := hexColorRegex.FindStringSubmatch(clean)
	if len(matches) == 2 {
		hex := "#" + strings.ToLower(matches[1])
		return hex, hex, nil
	}

	return "", "", fmt.Errorf("invalid color %q. Use a recognized name (blue, green, red, purple, orange, cyan, pink, emerald, indigo, slate) or hex '#RRGGBB'.", colorArg)
}

// GetProfileColor returns the current titleBar.activeBackground color configured for a profile, if any
func GetProfileColor(name string) (string, error) {
	profileDir := config.GetProfileDir(name)
	settingsFile := filepath.Join(config.GetUserDataDir(profileDir), "User", "settings.json")

	data, err := os.ReadFile(settingsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", nil // corrupted or non-standard json, ignore gracefully
	}

	customizations, ok := parsed["workbench.colorCustomizations"].(map[string]interface{})
	if !ok {
		return "", nil
	}

	val, ok := customizations["titleBar.activeBackground"].(string)
	if !ok {
		return "", nil
	}

	return val, nil
}

// ApplyProfileColor applies or clears workbench color customizations on the profile's settings.json
func ApplyProfileColor(name string, colorArg string, clear bool) error {
	profileDir := config.GetProfileDir(name)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", name)
	}

	var primary, secondary string
	if !clear {
		p, s, err := ResolveColor(colorArg)
		if err != nil {
			return err
		}
		primary, secondary = p, s
	}

	userDir := filepath.Join(config.GetUserDataDir(profileDir), "User")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return fmt.Errorf("failed to create user config dir: %w", err)
	}

	settingsFile := filepath.Join(userDir, "settings.json")

	// If settings.json is a symlink, uncouple it safely
	if fi, err := os.Lstat(settingsFile); err == nil && (fi.Mode()&os.ModeSymlink != 0) {
		content, readErr := os.ReadFile(settingsFile)
		_ = os.Remove(settingsFile)
		if readErr == nil {
			_ = os.WriteFile(settingsFile, content, 0644)
		}
	}

	parsed := make(map[string]interface{})
	if data, err := os.ReadFile(settingsFile); err == nil {
		_ = json.Unmarshal(data, &parsed)
	}

	colorKeys := []string{
		"titleBar.activeBackground", "titleBar.activeForeground",
		"titleBar.inactiveBackground", "titleBar.inactiveForeground",
		"activityBar.background", "activityBar.foreground",
		"activityBar.inactiveForeground", "statusBar.background",
		"statusBar.foreground",
	}

	if clear {
		if customizations, ok := parsed["workbench.colorCustomizations"].(map[string]interface{}); ok {
			for _, k := range colorKeys {
				delete(customizations, k)
			}
			if len(customizations) == 0 {
				delete(parsed, "workbench.colorCustomizations")
			}
		}
	} else {
		customizations, ok := parsed["workbench.colorCustomizations"].(map[string]interface{})
		if !ok {
			customizations = make(map[string]interface{})
			parsed["workbench.colorCustomizations"] = customizations
		}

		customizations["titleBar.activeBackground"] = primary
		customizations["titleBar.activeForeground"] = "#ffffff"
		customizations["titleBar.inactiveBackground"] = secondary
		customizations["titleBar.inactiveForeground"] = "#d1d5db"
		customizations["activityBar.background"] = secondary
		customizations["activityBar.foreground"] = "#ffffff"
		customizations["activityBar.inactiveForeground"] = "#9ca3af"
		customizations["statusBar.background"] = primary
		customizations["statusBar.foreground"] = "#ffffff"
	}

	f, err := os.OpenFile(settingsFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open settings.json: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(parsed); err != nil {
		return fmt.Errorf("failed to write settings.json: %w", err)
	}

	return nil
}

// CopyFile copies a regular file from src to dst
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// CopyDir recursively copies a directory tree from src to dst
func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
