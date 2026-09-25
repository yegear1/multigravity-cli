package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initTestRepo(t *testing.T) string {
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
	gitignoreFile := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gitignoreFile, []byte("config.local\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md", ".gitignore")
	run("commit", "-m", "Initial commit")

	return dir
}

func TestValidateWorktreeID(t *testing.T) {
	valid := []string{"task-1", "feature/auth", "bug_fix.1", "123-abc"}
	for _, id := range valid {
		if err := ValidateWorktreeID(id); err != nil {
			t.Errorf("expected %q to be valid, got: %v", id, err)
		}
	}

	invalid := []string{"", " ", "../escape", "task..bad", "-starts-with-dash", "invalid@char"}
	for _, id := range invalid {
		if err := ValidateWorktreeID(id); err == nil {
			t.Errorf("expected %q to be invalid, got nil", id)
		}
	}
}

func TestWorktreeLifecycle(t *testing.T) {
	repoDir := initTestRepo(t)

	// 1. Ensure git exclude works
	if err := EnsureGitExclude(repoDir, ".multigravity/"); err != nil {
		t.Fatalf("EnsureGitExclude failed: %v", err)
	}
	// Calling again should be idempotent
	if err := EnsureGitExclude(repoDir, ".multigravity/"); err != nil {
		t.Fatalf("EnsureGitExclude idempotent failed: %v", err)
	}

	// Create a dummy config file to test CopyFiles
	cfgFile := filepath.Join(repoDir, "config.local")
	_ = os.WriteFile(cfgFile, []byte("KEY=VAL\n"), 0644)

	// 2. Create worktree
	wt, err := CreateWorktree(CreateOptions{
		ID:        "agent-task-1",
		RepoPath:  repoDir,
		Profile:   "dev-profile",
		TaskID:    "T-100",
		CopyFiles: []string{"config.local"},
		Metadata: map[string]string{
			"agent": "claude-code",
		},
	})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	if wt.ID != "agent-task-1" {
		t.Errorf("expected ID agent-task-1, got %s", wt.ID)
	}
	if wt.Branch != "multigravity/agent-task-1" {
		t.Errorf("expected branch multigravity/agent-task-1, got %s", wt.Branch)
	}
	if wt.Status != "active" {
		t.Errorf("expected status active, got %s", wt.Status)
	}
	if wt.Profile != "dev-profile" {
		t.Errorf("expected profile dev-profile, got %s", wt.Profile)
	}
	if wt.Metadata["agent"] != "claude-code" {
		t.Errorf("expected metadata agent=claude-code, got %v", wt.Metadata)
	}

	// Verify copied file
	copiedCfg := filepath.Join(wt.Path, "config.local")
	if _, err := os.Stat(copiedCfg); os.IsNotExist(err) {
		t.Errorf("expected config.local to be copied to worktree")
	}

	// Verify duplicate ID fails
	_, err = CreateWorktree(CreateOptions{
		ID:       "agent-task-1",
		RepoPath: repoDir,
	})
	if err == nil {
		t.Fatalf("expected duplicate worktree creation to fail")
	}

	// 3. List worktrees
	list, err := ListWorktrees(repoDir)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 worktree in list, got %d", len(list))
	}
	if list[0].ID != "agent-task-1" {
		t.Errorf("expected worktree ID agent-task-1, got %s", list[0].ID)
	}

	// 4. Get worktree
	found, err := GetWorktree(repoDir, "agent-task-1")
	if err != nil {
		t.Fatalf("GetWorktree failed: %v", err)
	}
	if found.ID != "agent-task-1" {
		t.Errorf("expected worktree ID agent-task-1, got %s", found.ID)
	}

	// 5. Check clean status
	status, err := GetWorktreeStatus(repoDir, "agent-task-1")
	if err != nil {
		t.Fatalf("GetWorktreeStatus failed: %v", err)
	}
	if !status.IsClean {
		t.Errorf("expected initial status to be clean, got modified=%v untracked=%v",
			status.ModifiedFiles, status.UntrackedFiles)
	}

	// 6. Make modifications in worktree
	newFile := filepath.Join(wt.Path, "new_feature.go")
	if err := os.WriteFile(newFile, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	readmeInWt := filepath.Join(wt.Path, "README.md")
	if err := os.WriteFile(readmeInWt, []byte("# Test Repo Modified\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Check dirty status
	status, err = GetWorktreeStatus(repoDir, "agent-task-1")
	if err != nil {
		t.Fatalf("GetWorktreeStatus after edits failed: %v", err)
	}
	if status.IsClean {
		t.Errorf("expected status to be dirty after edits")
	}
	if len(status.ModifiedFiles) != 1 || status.ModifiedFiles[0] != "README.md" {
		t.Errorf("expected README.md in modified files, got %v", status.ModifiedFiles)
	}
	if len(status.UntrackedFiles) != 1 || status.UntrackedFiles[0] != "new_feature.go" {
		t.Errorf("expected new_feature.go in untracked files, got %v", status.UntrackedFiles)
	}

	// 7. Check diff
	diff, err := GetWorktreeDiff(repoDir, "agent-task-1", DiffOptions{})
	if err != nil {
		t.Fatalf("GetWorktreeDiff failed: %v", err)
	}
	if !strings.Contains(diff, "Test Repo Modified") {
		t.Errorf("expected diff to contain 'Test Repo Modified', got:\n%s", diff)
	}

	// 8. Attempt remove without force should fail due to dirty changes
	err = RemoveWorktree(repoDir, "agent-task-1", RemoveOptions{Force: false})
	if err == nil {
		t.Fatalf("expected remove of dirty worktree without force to fail")
	}

	// 9. Remove with force and delete branch
	err = RemoveWorktree(repoDir, "agent-task-1", RemoveOptions{Force: true, DeleteBranch: true})
	if err != nil {
		t.Fatalf("RemoveWorktree with force failed: %v", err)
	}

	// Verify worktree is gone
	list, err = ListWorktrees(repoDir)
	if err != nil {
		t.Fatalf("ListWorktrees after remove failed: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 worktrees after remove, got %d", len(list))
	}

	// Verify branch was deleted
	if BranchExists(repoDir, "multigravity/agent-task-1") {
		t.Errorf("expected branch multigravity/agent-task-1 to be deleted")
	}

	// 10. Prune
	if err := PruneWorktrees(repoDir); err != nil {
		t.Fatalf("PruneWorktrees failed: %v", err)
	}
}
