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

