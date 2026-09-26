package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/workspace"
)

func setupTestWorkspaceProfile(t *testing.T, profName, projName string) string {
	t.Helper()
	tmpHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tmpHome)

	profDir := filepath.Join(tmpHome, profName)
	projDir := filepath.Join(profDir, ".gemini", "config", "projects")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}

	userDataDir := config.GetUserDataDir(profDir)
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		t.Fatal(err)
	}

	appStorage := map[string]interface{}{
		"new-convo-last-selected-project": "uuid-test-proj",
	}
	stBytes, _ := json.Marshal(appStorage)
	_ = os.WriteFile(filepath.Join(userDataDir, "app_storage.json"), stBytes, 0644)

	raw := map[string]interface{}{
		"id":   "uuid-test-proj",
		"name": projName,
		"projectResources": map[string]interface{}{
			"resources": []map[string]interface{}{
				{
					"gitFolder": map[string]interface{}{
						"folderUri":     "file:///mock/repo/" + projName,
						"defaultBranch": "main",
					},
				},
			},
		},
		"settings": map[string]interface{}{
			"sandboxMode": false,
		},
		"isWorkspaceOnly": false,
	}
	rBytes, _ := json.Marshal(raw)
	_ = os.WriteFile(filepath.Join(projDir, "uuid-test-proj.json"), rBytes, 0644)

	return tmpHome
}

func TestWorkspaceListCmd(t *testing.T) {
	prof := "cli-test-prof"
	_ = setupTestWorkspaceProfile(t, prof, "alpha-project")

	restoreHooks := workspace.SetTestHooks(
		func(name string) bool { return name == prof },
		func(name string) ([]int, error) { return []int{9999}, nil },
		func() ([]string, error) { return []string{prof}, nil },
	)
	defer restoreHooks()

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"workspace", "list"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "alpha-project") {
		t.Errorf("expected alpha-project in list, got: %s", out)
	}
	if !strings.Contains(out, "yes") {
		t.Errorf("expected active column 'yes', got: %s", out)
	}

	// Test --json flag
	buf.Reset()
	rootCmd.SetArgs([]string{"workspace", "list", "--json"})
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error on --json: %v", err)
	}

	var jsonList []workspace.Workspace
	if err := json.Unmarshal(buf.Bytes(), &jsonList); err != nil {
		t.Fatalf("invalid JSON output: %v, raw: %s", err, buf.String())
	}
	if len(jsonList) != 1 || jsonList[0].Name != "alpha-project" {
		t.Errorf("unexpected json result: %+v", jsonList)
	}
}

func TestWorkspaceActiveCmd(t *testing.T) {
	prof := "active-prof"
	_ = setupTestWorkspaceProfile(t, prof, "active-project")

	restoreHooks := workspace.SetTestHooks(
		func(name string) bool { return true },
		func(name string) ([]int, error) { return []int{9999}, nil },
		func() ([]string, error) { return []string{prof}, nil },
	)
	defer restoreHooks()

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"workspace", "active", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var jsonList []workspace.Workspace
	if err := json.Unmarshal(buf.Bytes(), &jsonList); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(jsonList) != 1 || !jsonList[0].IsActive {
		t.Errorf("expected 1 active workspace, got %+v", jsonList)
	}
}

func TestWorkspaceShowCmd(t *testing.T) {
	prof := "show-prof"
	_ = setupTestWorkspaceProfile(t, prof, "show-project")

	restoreHooks := workspace.SetTestHooks(
		func(name string) bool { return false },
		nil,
		func() ([]string, error) { return []string{prof}, nil },
	)
	defer restoreHooks()

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"workspace", "show", prof, "show-project", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ws workspace.Workspace
	if err := json.Unmarshal(buf.Bytes(), &ws); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if ws.Name != "show-project" || ws.ID != "uuid-test-proj" {
		t.Errorf("unexpected workspace info: %+v", ws)
	}
}
