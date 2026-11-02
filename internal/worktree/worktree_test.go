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
	if !strings.Contains(diff, "new_feature.go") {
		t.Errorf("expected diff to contain untracked 'new_feature.go', got:\n%s", diff)
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

func TestGitDiffUntrackedAndIgnored(t *testing.T) {
	repoDir := initTestRepo(t)

	// Create an untracked file: main.py
	mainPy := filepath.Join(repoDir, "main.py")
	if err := os.WriteFile(mainPy, []byte("print('hello world')\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a modified tracked file: README.md
	readme := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test Repo Modified\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an ignored file: config.local (in .gitignore from initTestRepo)
	ignoredFile := filepath.Join(repoDir, "config.local")
	if err := os.WriteFile(ignoredFile, []byte("SECRET=123\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Full unified diff should contain main.py and README.md, but NOT config.local
	diff, err := GitDiff(repoDir, "", false, false)
	if err != nil {
		t.Fatalf("GitDiff failed: %v", err)
	}
	if !strings.Contains(diff, "main.py") {
		t.Errorf("expected diff to contain untracked 'main.py', got:\n%s", diff)
	}
	if !strings.Contains(diff, "print('hello world')") {
		t.Errorf("expected diff to contain additions of 'main.py', got:\n%s", diff)
	}
	if !strings.Contains(diff, "README.md") {
		t.Errorf("expected diff to contain modified 'README.md', got:\n%s", diff)
	}
	if strings.Contains(diff, "config.local") {
		t.Errorf("diff should NOT contain ignored 'config.local', got:\n%s", diff)
	}

	// 2. Stat-only diff should list main.py and README.md, but NOT config.local
	statDiff, err := GitDiff(repoDir, "", true, false)
	if err != nil {
		t.Fatalf("GitDiff --stat failed: %v", err)
	}
	if !strings.Contains(statDiff, "main.py") {
		t.Errorf("expected statDiff to contain 'main.py', got:\n%s", statDiff)
	}
	if !strings.Contains(statDiff, "README.md") {
		t.Errorf("expected statDiff to contain 'README.md', got:\n%s", statDiff)
	}
	if strings.Contains(statDiff, "config.local") {
		t.Errorf("statDiff should NOT contain ignored 'config.local', got:\n%s", statDiff)
	}

	// 3. Cached diff should NOT contain untracked main.py
	cachedDiff, err := GitDiff(repoDir, "", false, true)
	if err != nil {
		t.Fatalf("GitDiff --cached failed: %v", err)
	}
	if strings.Contains(cachedDiff, "main.py") {
		t.Errorf("cachedDiff should NOT contain untracked 'main.py', got:\n%s", cachedDiff)
	}
}

func TestWorktreeAdvancingMainDoesNotPolluteDiff(t *testing.T) {
	repoDir := initTestRepo(t)

	// Create a worktree passing symbolic ref "main" as BaseCommit
	wt, err := CreateWorktree(CreateOptions{
		ID:         "wt-adv-main",
		RepoPath:   repoDir,
		BaseCommit: "main",
	})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// BaseCommit should have been resolved to an immutable SHA, not "main"
	if wt.BaseCommit == "main" || len(wt.BaseCommit) < 40 {
		t.Errorf("expected BaseCommit to be resolved immutable SHA, got %q", wt.BaseCommit)
	}

	// Make a modification in the worktree
	readmeInWt := filepath.Join(wt.Path, "README.md")
	if err := os.WriteFile(readmeInWt, []byte("# Worktree Changes\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Advance main in the primary repository
	mainFile := filepath.Join(repoDir, "main_advance.txt")
	if err := os.WriteFile(mainFile, []byte("main advanced content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	runGitCmd := func(dir string, args ...string) {
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

	runGitCmd(repoDir, "add", "main_advance.txt")
	runGitCmd(repoDir, "commit", "-m", "advance main branch")

	// Get worktree diff; it should NOT include main_advance.txt
	diff, err := GetWorktreeDiff(repoDir, "wt-adv-main", DiffOptions{})
	if err != nil {
		t.Fatalf("GetWorktreeDiff failed: %v", err)
	}

	if !strings.Contains(diff, "Worktree Changes") {
		t.Errorf("expected diff to contain worktree modifications, got:\n%s", diff)
	}
	if strings.Contains(diff, "main_advance.txt") || strings.Contains(diff, "main advanced content") {
		t.Errorf("diff should NOT be polluted by advances to main branch, got:\n%s", diff)
	}
}

func TestWorktreeDiffOptionsPathsFiltering(t *testing.T) {
	repoDir := initTestRepo(t)

	// Add an extra tracked file in main repo before creating worktree
	extraFile := filepath.Join(repoDir, "tracked_extra.txt")
	if err := os.WriteFile(extraFile, []byte("tracked extra original\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "add", "tracked_extra.txt")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v: %s", err, string(out))
	}
	cmd = exec.Command("git", "commit", "-m", "add tracked_extra")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v: %s", err, string(out))
	}

	wt, err := CreateWorktree(CreateOptions{
		ID:       "wt-paths-filter",
		RepoPath: repoDir,
	})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// Modify tracked README.md
	if err := os.WriteFile(filepath.Join(wt.Path, "README.md"), []byte("# Modified README\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Modify tracked tracked_extra.txt
	if err := os.WriteFile(filepath.Join(wt.Path, "tracked_extra.txt"), []byte("tracked extra modified\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create untracked file 1
	if err := os.WriteFile(filepath.Join(wt.Path, "untracked1.txt"), []byte("untracked 1 content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create untracked file 2
	if err := os.WriteFile(filepath.Join(wt.Path, "untracked2.txt"), []byte("untracked 2 content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Filter by README.md and untracked1.txt
	diff, err := GetWorktreeDiff(repoDir, "wt-paths-filter", DiffOptions{
		Paths: []string{"README.md", "untracked1.txt"},
	})
	if err != nil {
		t.Fatalf("GetWorktreeDiff with paths failed: %v", err)
	}

	if !strings.Contains(diff, "Modified README") {
		t.Errorf("expected diff to contain 'Modified README', got:\n%s", diff)
	}
	if !strings.Contains(diff, "untracked 1 content") {
		t.Errorf("expected diff to contain 'untracked 1 content', got:\n%s", diff)
	}
	if strings.Contains(diff, "tracked extra modified") {
		t.Errorf("diff should NOT contain unrelated tracked file 'tracked_extra.txt', got:\n%s", diff)
	}
	if strings.Contains(diff, "untracked 2 content") {
		t.Errorf("diff should NOT contain unrelated untracked file 'untracked2.txt', got:\n%s", diff)
	}

	// Filter only untracked1.txt
	untrackedOnlyDiff, err := GetWorktreeDiff(repoDir, "wt-paths-filter", DiffOptions{
		Paths: []string{"untracked1.txt"},
	})
	if err != nil {
		t.Fatalf("GetWorktreeDiff untracked only failed: %v", err)
	}
	if !strings.Contains(untrackedOnlyDiff, "untracked 1 content") {
		t.Errorf("expected diff to contain 'untracked 1 content', got:\n%s", untrackedOnlyDiff)
	}
	if strings.Contains(untrackedOnlyDiff, "Modified README") || strings.Contains(untrackedOnlyDiff, "tracked extra modified") || strings.Contains(untrackedOnlyDiff, "untracked 2 content") {
		t.Errorf("diff should ONLY contain untracked1.txt, got:\n%s", untrackedOnlyDiff)
	}
}

func TestWorktreeAutoLink(t *testing.T) {
	repoDir := initTestRepo(t)

	deps := []string{"node_modules", "target", ".venv", ".cargo"}
	for _, dep := range deps {
		depDir := filepath.Join(repoDir, dep)
		if err := os.MkdirAll(depDir, 0755); err != nil {
			t.Fatal(err)
		}
		dummyFile := filepath.Join(depDir, "indicator.txt")
		if err := os.WriteFile(dummyFile, []byte(dep), 0644); err != nil {
			t.Fatal(err)
		}
	}

	wt, err := CreateWorktree(CreateOptions{
		ID:       "wt-autolink-test",
		RepoPath: repoDir,
	})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	for _, dep := range deps {
		dst := filepath.Join(wt.Path, dep)
		fi, err := os.Lstat(dst)
		if err != nil {
			t.Fatalf("expected auto-symlinked path %s to exist: %v", dep, err)
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("expected %s to be a symlink, got mode %v", dep, fi.Mode())
		}
		target, err := os.Readlink(dst)
		if err != nil {
			t.Fatalf("failed to read symlink %s: %v", dep, err)
		}
		expectedTarget := filepath.Join(repoDir, dep)
		if filepath.Clean(target) != filepath.Clean(expectedTarget) {
			t.Errorf("expected symlink target %s, got %s", expectedTarget, target)
		}

		content, err := os.ReadFile(filepath.Join(dst, "indicator.txt"))
		if err != nil {
			t.Errorf("failed to read file through symlink %s: %v", dep, err)
		} else if string(content) != dep {
			t.Errorf("expected file content %s, got %s", dep, string(content))
		}
	}

	missingDep := filepath.Join(wt.Path, "nonexistent_dep")
	if _, err := os.Lstat(missingDep); !os.IsNotExist(err) {
		t.Errorf("expected nonexistent dependency not to exist in worktree")
	}

	wt2, err := CreateWorktree(CreateOptions{
		ID:        "wt-autolink-overlap",
		RepoPath:  repoDir,
		LinkPaths: []string{"node_modules"},
	})
	if err != nil {
		t.Fatalf("CreateWorktree with overlapping LinkPaths failed: %v", err)
	}
	nm := filepath.Join(wt2.Path, "node_modules")
	if fi, err := os.Lstat(nm); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected node_modules to be symlinked without conflict")
	}
}

func TestPruneWorktreeAndPruneAllDeletesBranch(t *testing.T) {
	repoDir := initTestRepo(t)

	// 1. Create worktree with ephemeral multigravity/ branch
	wt1, err := CreateWorktree(CreateOptions{
		ID:       "ephemeral-wt-1",
		RepoPath: repoDir,
		Branch:   "multigravity/ephemeral-wt-1",
	})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	if !BranchExists(repoDir, "multigravity/ephemeral-wt-1") {
		t.Fatalf("expected branch multigravity/ephemeral-wt-1 to exist")
	}

	// 2. PruneWorktree should remove worktree and delete the multigravity/ branch
	if err := PruneWorktree(repoDir, wt1.ID); err != nil {
		t.Fatalf("PruneWorktree failed: %v", err)
	}

	if BranchExists(repoDir, "multigravity/ephemeral-wt-1") {
		t.Errorf("expected branch multigravity/ephemeral-wt-1 to be deleted after PruneWorktree")
	}

	// 3. Create another worktree and simulate missing directory
	wt2, err := CreateWorktree(CreateOptions{
		ID:       "ephemeral-wt-2",
		RepoPath: repoDir,
		Branch:   "multigravity/ephemeral-wt-2",
	})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	if !BranchExists(repoDir, "multigravity/ephemeral-wt-2") {
		t.Fatalf("expected branch multigravity/ephemeral-wt-2 to exist")
	}

	// Remove worktree directory on disk directly
	_ = os.RemoveAll(wt2.Path)

	// PruneAll should prune stale worktrees and delete orphaned multigravity/ branches
	if err := PruneAll(repoDir); err != nil {
		t.Fatalf("PruneAll failed: %v", err)
	}

	if BranchExists(repoDir, "multigravity/ephemeral-wt-2") {
		t.Errorf("expected branch multigravity/ephemeral-wt-2 to be deleted after PruneAll")
	}

	branches, err := GitListBranches(repoDir, "multigravity/")
	if err != nil {
		t.Fatalf("GitListBranches failed: %v", err)
	}
	if len(branches) != 0 {
		t.Errorf("expected 0 multigravity branches remaining, got %v", branches)
	}
}
