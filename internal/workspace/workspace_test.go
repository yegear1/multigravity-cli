package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

func TestFileURIToPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"/var/log", "/var/log"},
		{"file:///home/user/project", "/home/user/project"},
		{"file:///home/user/my%20project", "/home/user/my project"},
	}

	for _, tt := range tests {
		got := FileURIToPath(tt.input)
		if tt.input == "" {
			if got != "" {
				t.Errorf("FileURIToPath(\"\") = %q; want \"\"", got)
			}
			continue
		}
		// On windows normalize separators
		if runtime.GOOS == "windows" && tt.expected != "" && tt.expected[0] == '/' {
			continue // skip POSIX absolute on Windows
		}
		if got != filepath.Clean(tt.expected) {
			t.Errorf("FileURIToPath(%q) = %q; want %q", tt.input, got, filepath.Clean(tt.expected))
		}
	}
}

func TestDetectGitRepoInfo(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Not a git repo
	restoreGit := SetGitRunnerFn(func(dir string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "rev-parse" && args[1] == "--is-inside-work-tree" {
			return "false", fmt.Errorf("not a git repo")
		}
		return "", nil
	})
	info := DetectGitRepoInfo(tmpDir)
	if info == nil || info.IsGitRepo {
		t.Fatalf("expected IsGitRepo false, got %+v", info)
	}
	restoreGit()

	// 2. Clean git repo
	restoreGit = SetGitRunnerFn(func(dir string, args ...string) (string, error) {
		subCmd := args[0]
		switch subCmd {
		case "rev-parse":
			if args[1] == "--is-inside-work-tree" {
				return "true", nil
			}
			if args[1] == "--abbrev-ref" {
				return "feature/epic-08", nil
			}
		case "config":
			return "https://github.com/example/repo.git", nil
		case "log":
			return "a1b2c3d|initial commit", nil
		case "status":
			return "", nil
		}
		return "", nil
	})
	info = DetectGitRepoInfo(tmpDir)
	if info == nil || !info.IsGitRepo {
		t.Fatalf("expected IsGitRepo true, got %+v", info)
	}
	if info.Branch != "feature/epic-08" {
		t.Errorf("expected branch feature/epic-08, got %s", info.Branch)
	}
	if info.RemoteURL != "https://github.com/example/repo.git" {
		t.Errorf("expected remote url, got %s", info.RemoteURL)
	}
	if info.CommitHash != "a1b2c3d" || info.CommitMessage != "initial commit" {
		t.Errorf("commit info mismatch: %+v", info)
	}
	if !info.IsClean || info.ModifiedFiles != 0 || info.UntrackedFiles != 0 {
		t.Errorf("expected clean repo, got %+v", info)
	}
	restoreGit()

	// 3. Dirty git repo
	restoreGit = SetGitRunnerFn(func(dir string, args ...string) (string, error) {
		if args[0] == "rev-parse" && args[1] == "--is-inside-work-tree" {
			return "true", nil
		}
		if args[0] == "status" {
			return " M file1.go\n M file2.go\n?? untracked.txt", nil
		}
		return "", nil
	})
	info = DetectGitRepoInfo(tmpDir)
	if info == nil || info.IsClean {
		t.Fatalf("expected dirty repo, got %+v", info)
	}
	if info.ModifiedFiles != 2 || info.UntrackedFiles != 1 {
		t.Errorf("expected 2 modified and 1 untracked, got %d and %d", info.ModifiedFiles, info.UntrackedFiles)
	}
	restoreGit()
}

func setupMockProfileWithProjects(t *testing.T, profileName string, isRunning bool, lastSelectedProject string) string {
	t.Helper()
	tmpHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tmpHome)

	profDir := filepath.Join(tmpHome, profileName)
	projDir := filepath.Join(profDir, ".gemini", "config", "projects")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}

	userDataDir := config.GetUserDataDir(profDir)
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		t.Fatal(err)
	}

	if lastSelectedProject != "" {
		appStorage := map[string]interface{}{
			"new-convo-last-selected-project": lastSelectedProject,
		}
		storageBytes, _ := json.Marshal(appStorage)
		_ = os.WriteFile(filepath.Join(userDataDir, "app_storage.json"), storageBytes, 0644)
	}

	// Project 1
	p1 := rawProject{
		ID:   "proj-1-uuid",
		Name: "project-one",
		ProjectResources: rawProjectResources{
			Resources: []rawResource{
				{
					GitFolder: &rawGitFolder{
						FolderURI:     "file:///mock/repos/project-one",
						DefaultBranch: "main",
					},
				},
			},
		},
		Settings: rawProjectSettings{
			SandboxMode:         false,
			FileAccessPolicy:    "AGENT_SETTING_POLICY_ALLOW",
			AutoExecutionPolicy: "CASCADE_COMMANDS_AUTO_EXECUTION_EAGER",
		},
		IsWorkspaceOnly: false,
	}
	p1Bytes, _ := json.Marshal(p1)
	_ = os.WriteFile(filepath.Join(projDir, "proj-1-uuid.json"), p1Bytes, 0644)

	// Project 2
	p2 := rawProject{
		ID:   "proj-2-uuid",
		Name: "project-two",
		ProjectResources: rawProjectResources{
			Resources: []rawResource{
				{
					FolderURI: "file:///mock/repos/project-two",
				},
			},
		},
		Settings: rawProjectSettings{
			SandboxMode: true,
		},
		IsWorkspaceOnly: true,
	}
	p2Bytes, _ := json.Marshal(p2)
	_ = os.WriteFile(filepath.Join(projDir, "proj-2-uuid.json"), p2Bytes, 0644)

	return tmpHome
}

func TestGetProfileWorkspacesAndSummary(t *testing.T) {
	profileName := "test-prof"
	_ = setupMockProfileWithProjects(t, profileName, true, "proj-1-uuid")

	restoreHooks := SetTestHooks(
		func(name string) bool { return name == profileName },
		func(name string) ([]int, error) { return []int{1234}, nil },
		func() ([]string, error) { return []string{profileName}, nil },
	)
	defer restoreHooks()

	workspaces, err := GetProfileWorkspaces(profileName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(workspaces))
	}

	// proj-1 should be active because lastSelectedProject == "proj-1-uuid"
	var p1, p2 *Workspace
	for i := range workspaces {
		if workspaces[i].ID == "proj-1-uuid" {
			p1 = &workspaces[i]
		}
		if workspaces[i].ID == "proj-2-uuid" {
			p2 = &workspaces[i]
		}
	}

	if p1 == nil || !p1.IsActive {
		t.Fatalf("expected project-one to be active, got %+v", p1)
	}
	if p1.DefaultBranch != "main" {
		t.Errorf("expected default branch main, got %s", p1.DefaultBranch)
	}
	if p1.Settings.SandboxMode {
		t.Errorf("expected sandbox mode false for p1")
	}

	if p2 == nil || p2.IsActive {
		t.Fatalf("expected project-two to be inactive, got %+v", p2)
	}
	if !p2.IsWorkspaceOnly {
		t.Errorf("expected project-two to be workspace-only")
	}

	// Test Summary
	summary, err := GetProfileSummary(profileName)
	if err != nil {
		t.Fatalf("GetProfileSummary error: %v", err)
	}
	if !summary.IsRunning {
		t.Errorf("expected summary IsRunning true")
	}
	if summary.ActiveWorkspace == nil || summary.ActiveWorkspace.ID != "proj-1-uuid" {
		t.Errorf("expected active workspace proj-1-uuid, got %+v", summary.ActiveWorkspace)
	}
	if summary.TotalWorkspaces != 2 {
		t.Errorf("expected 2 total workspaces, got %d", summary.TotalWorkspaces)
	}
}

func TestGetAllAndActiveWorkspaces(t *testing.T) {
	profileName := "prof-running"
	_ = setupMockProfileWithProjects(t, profileName, true, "proj-1-uuid")

	restoreHooks := SetTestHooks(
		func(name string) bool { return true },
		func(name string) ([]int, error) { return []int{1234}, nil },
		func() ([]string, error) { return []string{profileName}, nil },
	)
	defer restoreHooks()

	all, err := GetAllWorkspaces()
	if err != nil {
		t.Fatalf("GetAllWorkspaces error: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(all))
	}

	active, err := GetActiveWorkspaces()
	if err != nil {
		t.Fatalf("GetActiveWorkspaces error: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active workspace, got %d", len(active))
	}
	if active[0].ID != "proj-1-uuid" {
		t.Errorf("expected active workspace proj-1-uuid, got %s", active[0].ID)
	}
}

func TestGetWorkspaceByPath(t *testing.T) {
	profileName := "prof-finder"
	_ = setupMockProfileWithProjects(t, profileName, false, "")

	restoreHooks := SetTestHooks(
		func(name string) bool { return false },
		nil,
		func() ([]string, error) { return []string{profileName}, nil },
	)
	defer restoreHooks()

	matched, err := GetWorkspaceByPath("/mock/repos/project-one")
	if err != nil {
		t.Fatalf("GetWorkspaceByPath error: %v", err)
	}
	if len(matched) != 1 {
		t.Fatalf("expected 1 match for /mock/repos/project-one, got %d", len(matched))
	}
	if matched[0].Name != "project-one" {
		t.Errorf("expected project-one, got %s", matched[0].Name)
	}
}
