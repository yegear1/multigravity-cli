package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/dispatch"
)

func setupTestGitRepoForCmd(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git command %s failed: %v, output: %s", args, err, out)
		}
	}

	run("git", "init")
	run("git", "config", "user.name", "Test")
	run("git", "config", "user.email", "test@test.com")

	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "README.md")
	run("git", "commit", "-m", "initial commit")

	return dir
}

func setupTestProfileForCmd(t *testing.T, name string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)

	profDir := filepath.Join(home, name)
	if err := os.MkdirAll(profDir, 0755); err != nil {
		t.Fatal(err)
	}
	return profDir
}

func setupTestProfilesForCmd(t *testing.T, names ...string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	for _, name := range names {
		profDir := filepath.Join(home, name)
		if err := os.MkdirAll(profDir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestDispatchCLIRunAndList(t *testing.T) {
	repoDir := setupTestGitRepoForCmd(t)
	setupTestProfileForCmd(t, "dev")

	// Change to repo dir for command execution
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	cmd := newDispatchCmd()

	// 1. List when empty
	out, err := executeCommand(cmd, "list", "--json")
	if err != nil {
		t.Fatalf("failed to execute dispatch list --json: %v", err)
	}
	var emptyList []dispatch.Task
	if err := json.Unmarshal([]byte(out), &emptyList); err != nil {
		t.Fatalf("failed to parse empty list JSON: %v (output: %s)", err, out)
	}

	// 2. Dispatch run in background with --detach
	out, err = executeCommand(cmd, "run", "dev", "--detach", "--prompt", "test prompt", "--json", "--", "sh", "-c", "echo 'dispatched via cli' && sleep 0.1")
	if err != nil {
		t.Fatalf("failed to run dispatch run --detach: %v (output: %s)", err, out)
	}

	var dispatched dispatch.Task
	if err := json.Unmarshal([]byte(out), &dispatched); err != nil {
		t.Fatalf("failed to parse dispatched JSON: %v (output: %s)", err, out)
	}
	if dispatched.ID == "" {
		t.Fatalf("expected non-empty task ID")
	}
	if dispatched.Profile != "dev" {
		t.Errorf("expected profile 'dev', got %s", dispatched.Profile)
	}

	// Wait for task completion
	time.Sleep(300 * time.Millisecond)

	// 3. Status command with --json
	out, err = executeCommand(cmd, "status", dispatched.ID, "--json")
	if err != nil {
		t.Fatalf("failed to run dispatch status --json: %v", err)
	}
	var statusTask dispatch.Task
	if err := json.Unmarshal([]byte(out), &statusTask); err != nil {
		t.Fatalf("failed to parse status JSON: %v", err)
	}
	if statusTask.ID != dispatched.ID {
		t.Errorf("expected task ID %s, got %s", dispatched.ID, statusTask.ID)
	}

	// 4. Logs command with --json
	out, err = executeCommand(cmd, "logs", dispatched.ID, "--json")
	if err != nil {
		t.Fatalf("failed to run dispatch logs --json: %v", err)
	}
	var logRes map[string]interface{}
	if err := json.Unmarshal([]byte(out), &logRes); err != nil {
		t.Fatalf("failed to parse logs JSON: %v", err)
	}
	logsStr, _ := logRes["logs"].(string)
	if !strings.Contains(logsStr, "dispatched via cli") {
		t.Errorf("expected log to contain 'dispatched via cli', got: %s", logsStr)
	}

	// 5. List command (tabular text mode)
	out, err = executeCommand(cmd, "list")
	if err != nil {
		t.Fatalf("failed to run dispatch list: %v", err)
	}
	if !strings.Contains(out, dispatched.ID) {
		t.Errorf("expected list output to contain task ID %s, got:\n%s", dispatched.ID, out)
	}

	// 6. Delete command with --json
	out, err = executeCommand(cmd, "delete", dispatched.ID, "--json")
	if err != nil {
		t.Fatalf("failed to run dispatch delete: %v", err)
	}
	var delRes map[string]interface{}
	if err := json.Unmarshal([]byte(out), &delRes); err != nil {
		t.Fatalf("failed to parse delete JSON: %v", err)
	}
	if delRes["deleted"] != true {
		t.Errorf("expected deleted=true, got %v", delRes["deleted"])
	}
}

func TestDispatchCLIRunWorktreeAndDiff(t *testing.T) {
	repoDir := setupTestGitRepoForCmd(t)
	setupTestProfileForCmd(t, "dev")

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	cmd := newDispatchCmd()

	// Run with --new-worktree
	out, err := executeCommand(cmd, "run", "dev", "--new-worktree", "--detach", "--json", "--", "sh", "-c", "echo 'modified line' >> README.md && sleep 0.1")
	if err != nil {
		t.Fatalf("failed to run dispatch with worktree: %v (output: %s)", err, out)
	}

	var task dispatch.Task
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		t.Fatalf("failed to parse task JSON: %v", err)
	}
	if task.WorktreeID == "" {
		t.Fatalf("expected worktree to be allocated")
	}

	time.Sleep(300 * time.Millisecond)

	// Check diff command with --json
	out, err = executeCommand(cmd, "diff", task.ID, "--json")
	if err != nil {
		t.Fatalf("failed to run dispatch diff: %v", err)
	}
	var diffRes map[string]interface{}
	if err := json.Unmarshal([]byte(out), &diffRes); err != nil {
		t.Fatalf("failed to parse diff JSON: %v", err)
	}
	diffStr, _ := diffRes["diff"].(string)
	if !strings.Contains(diffStr, "modified line") {
		t.Errorf("expected diff to contain 'modified line', got:\n%s", diffStr)
	}

	// Clean up task and worktree
	_, err = executeCommand(cmd, "delete", task.ID, "--worktree", "--json")
	if err != nil {
		t.Fatalf("failed to delete task with worktree: %v", err)
	}
}

func TestDispatchCLICancelAndPrune(t *testing.T) {
	repoDir := setupTestGitRepoForCmd(t)
	setupTestProfileForCmd(t, "dev")

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	cmd := newDispatchCmd()

	// Launch long running task
	out, err := executeCommand(cmd, "run", "dev", "--detach", "--json", "--", "sh", "-c", "sleep 15")
	if err != nil {
		t.Fatalf("failed to launch sleep task: %v", err)
	}

	var task dispatch.Task
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		t.Fatalf("failed to parse task JSON: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Cancel task
	out, err = executeCommand(cmd, "cancel", task.ID, "--force", "--json")
	if err != nil {
		t.Fatalf("failed to cancel task: %v", err)
	}
	var cancelRes map[string]interface{}
	if err := json.Unmarshal([]byte(out), &cancelRes); err != nil {
		t.Fatalf("failed to parse cancel JSON: %v", err)
	}
	if cancelRes["status"] != "cancelled" {
		t.Errorf("expected status 'cancelled', got %v", cancelRes["status"])
	}

	time.Sleep(100 * time.Millisecond)

	// Prune task
	out, err = executeCommand(cmd, "prune", "--max-age", "1ms", "--json")
	if err != nil {
		t.Fatalf("failed to prune tasks: %v", err)
	}
	var pruneRes map[string]interface{}
	if err := json.Unmarshal([]byte(out), &pruneRes); err != nil {
		t.Fatalf("failed to parse prune JSON: %v", err)
	}
	if pruneRes["pruned"].(float64) < 1 {
		t.Errorf("expected at least 1 task pruned, got %v", pruneRes["pruned"])
	}
}

func TestDispatchCLIDiffAndDashboard(t *testing.T) {
	repoDir := setupTestGitRepoForCmd(t)
	setupTestProfileForCmd(t, "dev")

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	cmd := newDispatchCmd()

	// 1. Test Dashboard when empty
	out, err := executeCommand(cmd, "dashboard", "--json")
	if err != nil {
		t.Fatalf("failed to run dispatch dashboard --json: %v", err)
	}
	var dash dispatch.TaskDashboardSummary
	if err := json.Unmarshal([]byte(out), &dash); err != nil {
		t.Fatalf("failed to decode dashboard json: %v, out: %s", err, out)
	}
	if dash.Total != 0 {
		t.Errorf("expected 0 total tasks, got %d", dash.Total)
	}

	// 2. Dispatch a task with a worktree modifying a file
	out, err = executeCommand(cmd, "run", "dev", "--new-worktree", "--detach", "--json", "--", "sh", "-c", "echo 'new line' >> README.md")
	if err != nil {
		t.Fatalf("failed to dispatch task with worktree: %v", err)
	}
	var task dispatch.Task
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		t.Fatalf("failed to decode task json: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// 3. Test Dashboard with task present
	out, err = executeCommand(cmd, "dashboard")
	if err != nil {
		t.Fatalf("failed to run dispatch dashboard: %v", err)
	}
	if !strings.Contains(out, "Task Execution Dashboard") {
		t.Errorf("expected dashboard title in output: %s", out)
	}

	// 4. Test diff --json (includes structured breakdown)
	out, err = executeCommand(cmd, "diff", task.ID, "--json")
	if err != nil {
		t.Fatalf("failed to run dispatch diff --json: %v", err)
	}
	var diffMap map[string]interface{}
	if err := json.Unmarshal([]byte(out), &diffMap); err != nil {
		t.Fatalf("failed to decode diff json: %v", err)
	}
	if diffMap["structured"] == nil {
		t.Errorf("expected structured key in diff JSON: %+v", diffMap)
	}

	// 5. Test diff --structured
	out, err = executeCommand(cmd, "diff", task.ID, "--structured")
	if err != nil {
		t.Fatalf("failed to run dispatch diff --structured: %v", err)
	}
	if !strings.Contains(out, "Diff Summary") {
		t.Errorf("expected diff summary header, got: %s", out)
	}

	// 6. Test diff --web
	out, err = executeCommand(cmd, "diff", task.ID, "--web")
	if err != nil {
		t.Fatalf("failed to run dispatch diff --web: %v", err)
	}
	if !strings.Contains(out, "Visual Diff Viewer: http://127.0.0.1:") {
		t.Errorf("expected web diff url, got: %s", out)
	}
}

func TestDispatchCLIPlan(t *testing.T) {
	repoDir := setupTestGitRepoForCmd(t)
	setupTestProfilesForCmd(t, "dev-plan-1", "dev-plan-2")

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	planFile := filepath.Join(repoDir, "test-plan.json")
	planJSON := `{
		"plan_id": "cli-plan-test",
		"workers": 2,
		"subtasks": [
			{
				"id": "sub-1",
				"profile": "dev-plan-1",
				"command": "sh",
				"args": ["-c", "mkdir -p mod1 && echo 'module 1' > mod1/mod1.go && git add -A"]
			},
			{
				"id": "sub-2",
				"profile": "dev-plan-2",
				"command": "sh",
				"args": ["-c", "mkdir -p mod2 && echo 'module 2' > mod2/mod2.go && git add -A"]
			}
		]
	}`
	if err := os.WriteFile(planFile, []byte(planJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newDispatchCmd()

	// 1. Test dispatch plan --json
	out, err := executeCommand(cmd, "plan", planFile, "--json")
	if err != nil {
		t.Fatalf("failed to run dispatch plan --json: %v", err)
	}

	var planRes dispatch.PlanResult
	if err := json.Unmarshal([]byte(out), &planRes); err != nil {
		t.Fatalf("failed to decode plan JSON: %v, out: %s", err, out)
	}
	if planRes.PlanID != "cli-plan-test" || planRes.Succeeded != 2 {
		t.Errorf("unexpected plan result: %+v", planRes)
	}
	if !planRes.UnifiedSummary.DisjointScopesClean {
		t.Errorf("expected DisjointScopesClean to be true")
	}

	// 2. Test dispatch plan terminal output
	planFile2 := filepath.Join(repoDir, "test-plan-2.json")
	planJSON2 := strings.Replace(planJSON, "cli-plan-test", "cli-plan-test-2", 1)
	if err := os.WriteFile(planFile2, []byte(planJSON2), 0644); err != nil {
		t.Fatal(err)
	}

	cmd2 := newDispatchCmd()
	outText, err := executeCommand(cmd2, "plan", planFile2)
	if err != nil {
		t.Fatalf("failed to run dispatch plan terminal: %v", err)
	}
	if !strings.Contains(outText, "Plan Execution") || !strings.Contains(outText, "Unified Diff Summary") {
		t.Errorf("expected plan output headers, got:\n%s", outText)
	}
}

func TestDispatchCLIPlanRepoOtherThanCwd(t *testing.T) {
	cwdRepo := setupTestGitRepoForCmd(t)
	targetRepo := setupTestGitRepoForCmd(t)
	setupTestProfilesForCmd(t, "dev-other-repo")

	renameMain := exec.Command("git", "branch", "-M", "main")
	renameMain.Dir = targetRepo
	if out, err := renameMain.CombinedOutput(); err != nil {
		t.Fatalf("failed to rename target branch: %v, output: %s", err, out)
	}

	gitOut := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v in %s failed: %v, output: %s", args, dir, err, out)
		}
		return string(out)
	}

	if remotes := strings.TrimSpace(gitOut(targetRepo, "remote")); remotes != "" {
		t.Fatalf("target repo must have no remotes, got: %s", remotes)
	}
	if porcelain := gitOut(targetRepo, "status", "--porcelain"); porcelain != "" {
		t.Fatalf("target main worktree must be clean, got: %s", porcelain)
	}
	cwdHead := strings.TrimSpace(gitOut(cwdRepo, "rev-parse", "HEAD"))
	cwdPorcelain := gitOut(cwdRepo, "status", "--porcelain")
	targetHead := strings.TrimSpace(gitOut(targetRepo, "rev-parse", "HEAD"))

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(cwdRepo); err != nil {
		t.Fatal(err)
	}

	planFile := filepath.Join(t.TempDir(), "other-repo-plan.json")
	planJSON := `{
		"plan_id": "cli-plan-other-repo",
		"workers": 1,
		"subtasks": [
			{
				"id": "hello",
				"profile": "dev-other-repo",
				"command": "sh",
				"args": ["-c", "echo hello > hello.txt"]
			}
		]
	}`
	if err := os.WriteFile(planFile, []byte(planJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newDispatchCmd()
	out, err := executeCommand(cmd, "plan", planFile, "--repo", targetRepo, "--json")
	if err != nil {
		t.Fatalf("failed to run dispatch plan --repo: %v\n%s", err, out)
	}

	var planRes dispatch.PlanResult
	if err := json.Unmarshal([]byte(out), &planRes); err != nil {
		t.Fatalf("failed to decode plan JSON: %v, out: %s", err, out)
	}
	wantRoot, err := filepath.EvalSymlinks(targetRepo)
	if err != nil {
		t.Fatal(err)
	}
	gotRoot, err := filepath.EvalSymlinks(planRes.RepoPath)
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot != wantRoot {
		t.Errorf("plan repo_path = %q, want %q", planRes.RepoPath, targetRepo)
	}
	if planRes.Status != "completed" || planRes.Succeeded != 1 {
		t.Fatalf("unexpected plan result: status=%s succeeded=%d failed=%d err=%v", planRes.Status, planRes.Succeeded, planRes.Failed, planRes.Subtasks)
	}
	if len(planRes.Subtasks) != 1 || planRes.Subtasks[0].WorktreePath == "" {
		t.Fatalf("expected a worktree path, got %+v", planRes.Subtasks)
	}
	helloInWorktree := filepath.Join(planRes.Subtasks[0].WorktreePath, "hello.txt")
	if _, err := os.Stat(helloInWorktree); err != nil {
		t.Fatalf("expected hello.txt in the target worktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetRepo, "hello.txt")); !os.IsNotExist(err) {
		t.Fatalf("hello.txt must stay out of the target main checkout, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(cwdRepo, "hello.txt")); !os.IsNotExist(err) {
		t.Fatalf("hello.txt must stay out of the cwd repo, stat err=%v", err)
	}
	if got := strings.TrimSpace(gitOut(cwdRepo, "rev-parse", "HEAD")); got != cwdHead {
		t.Errorf("cwd HEAD changed from %s to %s", cwdHead, got)
	}
	if got := gitOut(cwdRepo, "status", "--porcelain"); got != cwdPorcelain {
		t.Errorf("cwd porcelain changed: %q", got)
	}
	if got := strings.TrimSpace(gitOut(targetRepo, "rev-parse", "HEAD")); got != targetHead {
		t.Errorf("target HEAD changed from %s to %s", targetHead, got)
	}
	if porcelain := gitOut(targetRepo, "status", "--porcelain"); porcelain != "" {
		t.Errorf("target main worktree dirty after plan: %s", porcelain)
	}
}

func TestDispatchCLIDeleteForce(t *testing.T) {
	repoDir := setupTestGitRepoForCmd(t)
	setupTestProfileForCmd(t, "dev")

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	cmd := newDispatchCmd()
	out, err := executeCommand(cmd, "run", "dev", "--new-worktree", "--detach", "--json", "--", "sh", "-c", "echo 'force delete test' > file.txt")
	if err != nil {
		t.Fatalf("failed to run dispatch: %v", err)
	}

	var task dispatch.Task
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		t.Fatalf("failed to parse task JSON: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// Delete with --force
	out, err = executeCommand(cmd, "delete", task.ID, "--force", "--json")
	if err != nil {
		t.Fatalf("failed to delete task with --force: %v", err)
	}

	var delRes map[string]interface{}
	if err := json.Unmarshal([]byte(out), &delRes); err != nil {
		t.Fatalf("failed to parse delete response: %v", err)
	}
	if delRes["deleted"] != true {
		t.Errorf("expected deleted=true, got %v", delRes["deleted"])
	}
}

func TestDispatchCLIRunProfileAndRepoFlags(t *testing.T) {
	repoDir := setupTestGitRepoForCmd(t)
	setupTestProfilesForCmd(t, "dev", "other")

	cmd := newDispatchCmd()

	// 1. Neither positional nor --profile set: returns usage error
	_, err := executeCommand(cmd, "run", "--detach", "--json")
	if err == nil {
		t.Fatal("expected error when neither positional profile nor --profile is provided")
	}
	if !strings.Contains(err.Error(), "profile is required") {
		t.Fatalf("expected 'profile is required' error, got: %v", err)
	}

	// 2. Both set but differ: returns error
	_, err = executeCommand(cmd, "run", "other", "--profile", "dev", "--detach", "--json")
	if err == nil {
		t.Fatal("expected error when positional profile differs from --profile")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected mismatch error, got: %v", err)
	}

	// 3. Both set and match: succeeds
	out, err := executeCommand(cmd, "run", "dev", "--profile", "dev", "--detach", "--json", "--repo", repoDir, "--", "sh", "-c", "echo 'match'")
	if err != nil {
		t.Fatalf("expected success when positional profile matches --profile, got: %v", err)
	}
	var task1 dispatch.Task
	if err := json.Unmarshal([]byte(out), &task1); err != nil {
		t.Fatalf("failed to parse task JSON: %v", err)
	}
	if task1.Profile != "dev" {
		t.Fatalf("expected profile 'dev', got %q", task1.Profile)
	}

	// 4. Only --profile set (no positional profile): succeeds
	out, err = executeCommand(cmd, "run", "--profile", "dev", "--detach", "--json", "--repo", repoDir, "--", "sh", "-c", "echo 'flag only'")
	if err != nil {
		t.Fatalf("expected success with --profile only, got: %v", err)
	}
	var task2 dispatch.Task
	if err := json.Unmarshal([]byte(out), &task2); err != nil {
		t.Fatalf("failed to parse task JSON: %v", err)
	}
	if task2.Profile != "dev" {
		t.Fatalf("expected profile 'dev', got %q", task2.Profile)
	}

	// 5. Cwd is in non-git directory, but --repo points to repoDir with -w (new worktree):
	nonGitDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nonGitDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(origWd)
	}()

	out, err = executeCommand(cmd, "run", "--profile", "dev", "--repo", repoDir, "-w", "--detach", "--json", "--", "sh", "-c", "echo 'in worktree'")
	if err != nil {
		t.Fatalf("expected success creating worktree via --repo from non-git cwd, got: %v", err)
	}
	var task3 dispatch.Task
	if err := json.Unmarshal([]byte(out), &task3); err != nil {
		t.Fatalf("failed to parse task JSON: %v", err)
	}
	if task3.WorktreeID == "" {
		t.Fatal("expected worktree to be created via --repo")
	}
	if task3.RepoPath != repoDir {
		t.Fatalf("expected task RepoPath to be %q, got %q", repoDir, task3.RepoPath)
	}

	time.Sleep(200 * time.Millisecond)
	_, _ = executeCommand(cmd, "delete", task3.ID, "--worktree", "--force", "--json")
	_ = os.Chdir(origWd)
}



