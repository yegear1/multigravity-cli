package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSeedDefaultPermissionsNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.json")

	changed, err := SeedDefaultPermissions(cfgFile)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !changed {
		t.Errorf("expected changed to be true for new file")
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal config json: %v", err)
	}

	userSettings, ok := parsed["userSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected userSettings map")
	}
	gpg, ok := userSettings["globalPermissionGrants"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected globalPermissionGrants map")
	}
	allowList, ok := gpg["allow"].([]interface{})
	if !ok {
		t.Fatalf("expected allow slice")
	}

	// Each command generates 2 grants (command and unsandboxed)
	expectedCount := len(DefaultReadOnlyCommands) * 2
	if len(allowList) != expectedCount {
		t.Errorf("expected %d grants, got %d", expectedCount, len(allowList))
	}

	// Re-running without changes should return changed = false
	changedAgain, err := SeedDefaultPermissions(cfgFile)
	if err != nil {
		t.Fatalf("expected no error on second run, got: %v", err)
	}
	if changedAgain {
		t.Errorf("expected changed to be false on second run")
	}
}

func TestSeedDefaultPermissionsAdditive(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.json")

	initialJSON := `{
  "userSettings": {
    "themeMode": "dark",
    "globalPermissionGrants": {
      "allow": [
        "command(custom-tool)"
      ]
    }
  }
}`
	if err := os.WriteFile(cfgFile, []byte(initialJSON), 0644); err != nil {
		t.Fatalf("failed to write initial json: %v", err)
	}

	changed, err := SeedDefaultPermissions(cfgFile)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !changed {
		t.Errorf("expected changed to be true")
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal config json: %v", err)
	}

	userSettings := parsed["userSettings"].(map[string]interface{})
	if userSettings["themeMode"] != "dark" {
		t.Errorf("expected themeMode to be dark, got: %v", userSettings["themeMode"])
	}

	gpg := userSettings["globalPermissionGrants"].(map[string]interface{})
	allowList := gpg["allow"].([]interface{})

	// Pre-existing custom-tool must be preserved
	var hasCustom bool
	for _, a := range allowList {
		if a == "command(custom-tool)" {
			hasCustom = true
			break
		}
	}
	if !hasCustom {
		t.Errorf("pre-existing custom-tool grant was lost!")
	}
	if len(allowList) != len(DefaultReadOnlyCommands)*2+1 {
		t.Errorf("expected %d grants, got %d", len(DefaultReadOnlyCommands)*2+1, len(allowList))
	}
}
