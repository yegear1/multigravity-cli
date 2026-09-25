package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v: %s", strings.Join(args, " "), err, string(out))
		}
	}

	run("init", "-b", "main")
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "Initial commit")

	return dir
}

func TestWorktreeCommands(t *testing.T) {
	repoDir := initTestGitRepo(t)

	// 1. List worktrees initially empty
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"worktree", "list", "--repo", repoDir, "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("worktree list failed: %v", err)
	}

	var initialList []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &initialList); err != nil {
		t.Fatalf("failed to parse json list: %v (output: %s)", err, buf.String())
	}
	if len(initialList) != 0 {
		t.Errorf("expected 0 worktrees, got %d", len(initialList))
	}

	// 2. Create worktree via CLI
	buf.Reset()
	rootCmd.SetArgs([]string{
		"worktree", "create", "cli-task",
		"--repo", repoDir,
		"--profile", "test-profile",
		"--task", "TASK-42",
		"--json",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("worktree create failed: %v", err)
	}

	var createdWT map[string]any
	if err := json.Unmarshal(buf.Bytes(), &createdWT); err != nil {
		t.Fatalf("failed to parse created worktree: %v (output: %s)", err, buf.String())
	}
	if createdWT["id"] != "cli-task" {
		t.Errorf("expected id cli-task, got %v", createdWT["id"])
	}
	if createdWT["profile"] != "test-profile" {
		t.Errorf("expected profile test-profile, got %v", createdWT["profile"])
	}

	// 3. Status via CLI
	buf.Reset()
	rootCmd.SetArgs([]string{"worktree", "status", "cli-task", "--repo", repoDir, "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("worktree status failed: %v", err)
	}
	var statusMap map[string]any
	if err := json.Unmarshal(buf.Bytes(), &statusMap); err != nil {
		t.Fatalf("failed to parse worktree status: %v", err)
	}
	if statusMap["is_clean"] != true {
		t.Errorf("expected is_clean=true, got %v", statusMap["is_clean"])
	}

	// 4. Modify file in worktree and check diff
	wtPath := createdWT["path"].(string)
	_ = os.WriteFile(filepath.Join(wtPath, "README.md"), []byte("# Modified Content\n"), 0644)

	buf.Reset()
	rootCmd.SetArgs([]string{"worktree", "diff", "cli-task", "--repo", repoDir})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("worktree diff failed: %v", err)
	}
	if !strings.Contains(buf.String(), "Modified Content") {
		t.Errorf("expected diff to show Modified Content, got:\n%s", buf.String())
	}

	// 5. Remove with force and delete branch
	buf.Reset()
	rootCmd.SetArgs([]string{"worktree", "remove", "cli-task", "--repo", repoDir, "--force", "--delete-branch", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("worktree remove failed: %v", err)
	}
	var removeResp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &removeResp); err != nil {
		t.Fatalf("failed to parse remove response: %v", err)
	}
	if removeResp["removed"] != true {
		t.Errorf("expected removed=true, got %v", removeResp["removed"])
	}

	// 6. Prune via CLI
	buf.Reset()
	rootCmd.SetArgs([]string{"worktree", "prune", "--repo", repoDir, "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("worktree prune failed: %v", err)
	}
}
