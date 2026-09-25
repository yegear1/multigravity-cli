package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

// TemplateInfo holds metadata about a saved profile template
type TemplateInfo struct {
	Name string
	Path string
	Size string
}

// SaveTemplate saves an existing profile as a reusable template
func SaveTemplate(profileName, templateName string) error {
	if err := config.ValidateProfileName(profileName); err != nil {
		return fmt.Errorf("invalid profile name: %w", err)
	}
	if err := config.ValidateProfileName(templateName); err != nil {
		return fmt.Errorf("invalid template name: %w", err)
	}

	srcDir := config.GetProfileDir(profileName)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", profileName)
	}

	tplDir := config.GetTemplatesDir()
	tplPath := filepath.Join(tplDir, templateName)

	if _, err := os.Stat(tplPath); err == nil {
		return fmt.Errorf("template %q already exists", templateName)
	}

	if err := os.MkdirAll(tplDir, 0755); err != nil {
		return fmt.Errorf("failed to create templates directory: %w", err)
	}

	if err := CopyDir(srcDir, tplPath); err != nil {
		return fmt.Errorf("failed to copy profile to template: %w", err)
	}

	// Templates should not retain the .shared or .auth_only marker
	_ = os.Remove(filepath.Join(tplPath, config.SentinelShared))
	_ = os.Remove(filepath.Join(tplPath, config.SentinelAuthOnly))

	return nil
}

// ListTemplates returns all available templates sorted alphabetically
func ListTemplates() ([]TemplateInfo, error) {
	tplDir := config.GetTemplatesDir()
	entries, err := os.ReadDir(tplDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read templates directory: %w", err)
	}

	var templates []TemplateInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(tplDir, name)
		size := GetDirSizeStr(path)

		templates = append(templates, TemplateInfo{
			Name: name,
			Path: path,
			Size: size,
		})
	}

	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})

	return templates, nil
}

// DeleteTemplate deletes a saved template by name
func DeleteTemplate(templateName string) error {
	if err := config.ValidateProfileName(templateName); err != nil {
		return fmt.Errorf("invalid template name: %w", err)
	}

	tplPath := filepath.Join(config.GetTemplatesDir(), templateName)
	if _, err := os.Stat(tplPath); os.IsNotExist(err) {
		return fmt.Errorf("template %q does not exist", templateName)
	}

	if err := os.RemoveAll(tplPath); err != nil {
		return fmt.Errorf("failed to remove template %q: %w", templateName, err)
	}

	return nil
}
