package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

func TestTemplates(t *testing.T) {
	tempHome := setupTestHome(t)

	// Create a profile to use as template source
	err := CreateProfile(CreateOptions{
		Name:             "base-worker",
		Shared:           true,
		IsolatedDotfiles: true,
	})
	if err != nil {
		t.Fatalf("failed to create base profile: %v", err)
	}

	srcDir := filepath.Join(tempHome, "base-worker")
	testMarker := filepath.Join(srcDir, "template_marker.txt")
	_ = os.WriteFile(testMarker, []byte("template-content"), 0644)

	// 1. SaveTemplate
	err = SaveTemplate("base-worker", "dev-template")
	if err != nil {
		t.Fatalf("expected SaveTemplate to succeed, got: %v", err)
	}

	tplDir := filepath.Join(config.GetTemplatesDir(), "dev-template")
	if _, err := os.Stat(tplDir); os.IsNotExist(err) {
		t.Fatalf("expected template dir %q to exist", tplDir)
	}

	// Verify .shared was removed from template
	if _, err := os.Stat(filepath.Join(tplDir, ".shared")); err == nil {
		t.Errorf("expected .shared to be removed from template")
	}

	// Duplicate save should fail
	err = SaveTemplate("base-worker", "dev-template")
	if err == nil {
		t.Errorf("expected error saving duplicate template, got nil")
	}

	// 2. ListTemplates
	templates, err := ListTemplates()
	if err != nil {
		t.Fatalf("expected ListTemplates to succeed, got: %v", err)
	}
	if len(templates) != 1 || templates[0].Name != "dev-template" {
		t.Fatalf("expected 1 template named 'dev-template', got: %+v", templates)
	}

	// 3. Create profile from template
	err = CreateProfile(CreateOptions{
		Name:         "from-tpl-profile",
		FromTemplate: "dev-template",
	})
	if err != nil {
		t.Fatalf("failed to create profile from template: %v", err)
	}

	destDir := filepath.Join(tempHome, "from-tpl-profile")
	destMarker := filepath.Join(destDir, "template_marker.txt")
	content, err := os.ReadFile(destMarker)
	if err != nil || string(content) != "template-content" {
		t.Fatalf("file from template was not preserved in new profile: %v", err)
	}

	// 4. Create profile from non-existent template
	err = CreateProfile(CreateOptions{
		Name:         "failing-profile",
		FromTemplate: "non-existent-tpl",
	})
	if err == nil {
		t.Errorf("expected error when template does not exist, got nil")
	}

	// 5. DeleteTemplate
	err = DeleteTemplate("dev-template")
	if err != nil {
		t.Fatalf("expected DeleteTemplate to succeed, got: %v", err)
	}

	templatesAfter, err := ListTemplates()
	if err != nil {
		t.Fatalf("failed to list templates after delete: %v", err)
	}
	if len(templatesAfter) != 0 {
		t.Errorf("expected 0 templates after deletion, got %d", len(templatesAfter))
	}

	// Delete non-existent template
	err = DeleteTemplate("non-existent-tpl")
	if err == nil {
		t.Errorf("expected error deleting non-existent template, got nil")
	}
}
