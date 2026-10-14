package dispatch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/agent"
	"github.com/ye-dev/multigravity-cli/internal/worktree"
)

func setupTestGitRepo(t *testing.T) string {
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

func setupTestProfile(t *testing.T, name string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)

	profDir := filepath.Join(home, name)
	if err := os.MkdirAll(profDir, 0755); err != nil {
		t.Fatal(err)
	}
	return profDir
}

func TestTaskIDValidation(t *testing.T) {
	valid := []string{"task-1", "task_abc", "task.123", "feature/task-1"}
	for _, id := range valid {
		if err := ValidateTaskID(id); err != nil {
			t.Errorf("expected valid ID %q, got error: %v", id, err)
		}
	}

	invalid := []string{"", " ", "../task", "task..1", "-task", "!invalid"}
	for _, id := range invalid {
		if err := ValidateTaskID(id); err == nil {
			t.Errorf("expected error for invalid ID %q, got nil", id)
		}
	}
}

func TestResolveAgentCommand(t *testing.T) {
	// Claude
	cmd, args := ResolveAgentCommand("claude", "", nil, "fix tests")
	if cmd != "claude" || len(args) != 2 || args[0] != "-p" || args[1] != "fix tests" {
		t.Errorf("unexpected claude command: %s %v", cmd, args)
	}

	// Aider
	cmd, args = ResolveAgentCommand("aider", "", nil, "write docs")
	if cmd != "aider" || len(args) != 2 || args[0] != "--message" || args[1] != "write docs" {
		t.Errorf("unexpected aider command: %s %v", cmd, args)
	}

	// Custom command overrides
	cmd, args = ResolveAgentCommand("", "python3", []string{"script.py"}, "")
	if cmd != "python3" || len(args) != 1 || args[0] != "script.py" {
		t.Errorf("unexpected custom command: %s %v", cmd, args)
	}

	// agy with prompt and empty args
	cmd, args = ResolveAgentCommand("", "agy", nil, "fix issue")
	if cmd != "agy" || len(args) != 2 || args[0] != "-p" || args[1] != "fix issue" {
		t.Errorf("unexpected agy command with prompt: %s %v", cmd, args)
	}

	// agy with explicit args does not duplicate -p
	cmd, args = ResolveAgentCommand("", "agy", []string{"-p", "custom prompt", "--other"}, "fix issue")
	if cmd != "agy" || len(args) != 3 || args[0] != "-p" || args[1] != "custom prompt" || args[2] != "--other" {
		t.Errorf("unexpected agy command with explicit args: %s %v", cmd, args)
	}

	// command with spaces is not sliced into argv
	cmd, args = ResolveAgentCommand("", "echo hello world", nil, "")
	if cmd != "echo hello world" || len(args) != 0 {
		t.Errorf("unexpected command with spaces: %s %v", cmd, args)
	}

	// codex with prompt (without cwd)
	cmd, args = ResolveAgentCommand("codex", "", nil, "fix tests")
	if cmd != "codex" || len(args) != 4 || args[0] != "exec" || args[1] != "--sandbox" || args[2] != "workspace-write" || args[3] != "fix tests" {
		t.Errorf("unexpected codex command without cwd: %s %v", cmd, args)
	}

	// codex with cwd
	cmd, args = ResolveAgentCommandWithCwd("codex", "", nil, "fix tests", "/my/repo")
	if cmd != "codex" || len(args) != 6 || args[0] != "exec" || args[1] != "-C" || args[2] != "/my/repo" || args[3] != "--sandbox" || args[4] != "workspace-write" || args[5] != "fix tests" {
		t.Errorf("unexpected codex command with cwd: %s %v", cmd, args)
	}

	// codex inferred from command with cwd
	cmd, args = ResolveAgentCommandWithCwd("", "codex", nil, "fix tests", "/my/repo")
	if cmd != "codex" || len(args) != 6 || args[0] != "exec" || args[1] != "-C" || args[2] != "/my/repo" || args[3] != "--sandbox" || args[4] != "workspace-write" || args[5] != "fix tests" {
		t.Errorf("unexpected codex command inferred from command: %s %v", cmd, args)
	}

	// codex with explicit args does not duplicate -C or -s/--sandbox
	cmd, args = ResolveAgentCommandWithCwd("codex", "", []string{"exec", "-C", "/custom", "-s", "read-only"}, "fix tests", "/my/repo")
	if cmd != "codex" || len(args) != 6 || args[0] != "exec" || args[1] != "-C" || args[2] != "/custom" || args[3] != "-s" || args[4] != "read-only" || args[5] != "fix tests" {
		t.Errorf("unexpected codex command with explicit args: %s %v", cmd, args)
	}

	// codex with prompt already in args does not duplicate prompt
	cmd, args = ResolveAgentCommandWithCwd("codex", "", []string{"exec", "-C", "/custom", "fix tests"}, "fix tests", "/my/repo")
	if cmd != "codex" || len(args) != 6 || args[0] != "exec" || args[1] != "--sandbox" || args[2] != "workspace-write" || args[3] != "-C" || args[4] != "/custom" || args[5] != "fix tests" {
		t.Errorf("unexpected codex command with existing prompt in args: %s %v", cmd, args)
	}
}

func TestDispatchTaskLifecycle(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	eventFired := make(chan string, 10)
	taskMgr.SetEventListener(func(event string, task *Task) {
		eventFired <- event
	})

	// Dispatch task with new ephemeral worktree
	task, err := taskMgr.Dispatch(DispatchOptions{
		ID:          "task-test-lifecycle",
		Profile:     "dev",
		RepoPath:    repoDir,
		NewWorktree: true,
		Command:     "sh",
		Args:        []string{"-c", "echo 'hello from agent task' && echo 'second line' > created_file.txt"},
	})
	if err != nil {
		t.Fatalf("failed to dispatch task: %v", err)
	}

	if task.ID != "task-test-lifecycle" {
		t.Errorf("expected task ID 'task-test-lifecycle', got %s", task.ID)
	}
	if task.Status != StatusRunning && task.Status != StatusCompleted {
		t.Errorf("unexpected initial status: %s", task.Status)
	}
	if task.WorktreeID == "" || task.WorktreePath == "" {
		t.Errorf("expected worktree to be configured on task: %+v", task)
	}

	// Wait for task completion
	deadline := time.Now().Add(5 * time.Second)
	var completedTask *Task
	for time.Now().Before(deadline) {
		tCur, err := taskMgr.GetTask(repoDir, "task-test-lifecycle")
		if err == nil && (tCur.Status == StatusCompleted || tCur.Status == StatusFailed) {
			completedTask = tCur
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if completedTask == nil {
		t.Fatal("timed out waiting for task to complete")
	}

	if completedTask.Status != StatusCompleted {
		t.Fatalf("task did not complete successfully: status=%s, error=%s", completedTask.Status, completedTask.Error)
	}
	if completedTask.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", completedTask.ExitCode)
	}

	// Verify log capture
	logs, err := taskMgr.GetTaskLogs(repoDir, "task-test-lifecycle", 0)
	if err != nil {
		t.Fatalf("failed to get task logs: %v", err)
	}
	if !strings.Contains(string(logs), "hello from agent task") {
		t.Errorf("expected logs to contain 'hello from agent task', got:\n%s", string(logs))
	}

	// Verify diff inside worktree (includes untracked created_file.txt)
	diff, err := taskMgr.GetTaskDiff(repoDir, "task-test-lifecycle", false)
	if err != nil {
		t.Fatalf("failed to get task diff: %v", err)
	}
	if !strings.Contains(diff, "created_file.txt") {
		t.Errorf("expected diff to contain untracked created_file.txt, got:\n%s", diff)
	}

	// Verify structured diff and files
	sd, err := taskMgr.GetTaskStructuredDiff(repoDir, "task-test-lifecycle")
	if err != nil {
		t.Fatalf("failed to get structured diff: %v", err)
	}
	if sd == nil {
		t.Fatal("expected non-nil structured diff")
	}
	if sd.Summary.FilesChanged != 1 || sd.Summary.Additions != 1 {
		t.Errorf("expected 1 file changed and 1 addition, got %+v", sd.Summary)
	}
	if len(sd.Files) != 1 || sd.Files[0].NewPath != "created_file.txt" {
		t.Errorf("expected created_file.txt in sd.Files, got %+v", sd.Files)
	}
	if sd.Files[0].Status != DiffFileAdded {
		t.Errorf("expected DiffFileAdded status for created_file.txt, got %s", sd.Files[0].Status)
	}

	files, err := taskMgr.GetTaskFiles(repoDir, "task-test-lifecycle")
	if err != nil {
		t.Fatalf("failed to get task files: %v", err)
	}
	if len(files) != 1 || files[0].NewPath != "created_file.txt" {
		t.Errorf("expected created_file.txt in files, got %+v", files)
	}

	// Verify Dashboard Summary
	dashboard, err := taskMgr.GetDashboardSummary(repoDir)
	if err != nil {
		t.Fatalf("failed to get dashboard summary: %v", err)
	}
	if dashboard.Total < 1 {
		t.Errorf("expected dashboard total >= 1, got %d", dashboard.Total)
	}

	// Verify ListTasks
	tasks, err := taskMgr.ListTasks(repoDir, TaskFilter{Profile: "dev"})
	if err != nil {
		t.Fatalf("failed to list tasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatalf("expected at least 1 task listed, got 0")
	}
	if tasks[0].ID != "task-test-lifecycle" {
		t.Errorf("expected listed task ID 'task-test-lifecycle', got %s", tasks[0].ID)
	}

	// Verify worktree exists
	wt, err := worktree.GetWorktree(repoDir, "task-test-lifecycle")
	if err != nil {
		t.Fatalf("expected worktree to exist: %v", err)
	}
	if wt.ID != "task-test-lifecycle" {
		t.Errorf("expected worktree ID 'task-test-lifecycle', got %s", wt.ID)
	}

	// Delete task with removeWorktree=true
	if err := taskMgr.DeleteTask(repoDir, "task-test-lifecycle", true); err != nil {
		t.Fatalf("failed to delete task: %v", err)
	}

	// Ensure task is removed from list
	tasksAfter, _ := taskMgr.ListTasks(repoDir, TaskFilter{})
	for _, tsk := range tasksAfter {
		if tsk.ID == "task-test-lifecycle" {
			t.Errorf("expected task to be deleted from list, but still present")
		}
	}

	// Ensure worktree is removed
	_, err = worktree.GetWorktree(repoDir, "task-test-lifecycle")
	if err == nil {
		t.Errorf("expected worktree to be removed, but still found")
	}
}

func TestCancelTask(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	task, err := taskMgr.Dispatch(DispatchOptions{
		ID:       "task-to-cancel",
		Profile:  "dev",
		RepoPath: repoDir,
		Command:  "sh",
		Args:     []string{"-c", "sleep 10"},
	})
	if err != nil {
		t.Fatalf("failed to dispatch task: %v", err)
	}
	_ = task

	time.Sleep(100 * time.Millisecond)

	if err := taskMgr.CancelTask(repoDir, "task-to-cancel", true); err != nil {
		t.Fatalf("failed to cancel task: %v", err)
	}

	tCur, err := taskMgr.GetTask(repoDir, "task-to-cancel")
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	if tCur.Status != StatusCancelled {
		t.Errorf("expected status 'cancelled', got %s", tCur.Status)
	}

	// Allow background log streamer and watcher to finish closing files
	time.Sleep(150 * time.Millisecond)
}

func TestPruneTasks(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	task, err := taskMgr.Dispatch(DispatchOptions{
		ID:       "task-to-prune",
		Profile:  "dev",
		RepoPath: repoDir,
		Command:  "sh",
		Args:     []string{"-c", "echo 'done'"},
	})
	if err != nil {
		t.Fatalf("failed to dispatch task: %v", err)
	}
	_ = task

	time.Sleep(200 * time.Millisecond)

	// Prune with maxAge=1ns to prune immediately
	pruned, err := taskMgr.PruneTasks(repoDir, 1*time.Nanosecond)
	if err != nil {
		t.Fatalf("failed to prune tasks: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 task pruned, got %d", pruned)
	}

	_, err = taskMgr.GetTask(repoDir, task.ID)
	if err == nil {
		t.Errorf("expected task %s to be pruned, but found it", task.ID)
	}
}

func TestDispatchCodexWithWorktree(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev")

	var capturedOpts agent.CreateSessionOptions
	cleanup := agent.SetTestHooks(func(opts agent.CreateSessionOptions) *exec.Cmd {
		capturedOpts = opts
		return exec.Command("sh", "-c", "echo 'codex mocked output'")
	})
	defer cleanup()

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	task, err := taskMgr.Dispatch(DispatchOptions{
		ID:          "task-codex-test",
		Profile:     "dev",
		RepoPath:    repoDir,
		AgentType:   "codex",
		Prompt:      "implement feature X",
		NewWorktree: true,
	})
	if err != nil {
		t.Fatalf("failed to dispatch codex task: %v", err)
	}

	if task.Command != "codex" {
		t.Errorf("expected command 'codex', got %q", task.Command)
	}
	if task.AgentType != "codex" {
		t.Errorf("expected agent_type 'codex', got %q", task.AgentType)
	}
	if task.WorktreeID == "" || task.WorktreePath == "" {
		t.Fatalf("expected worktree to be configured on task: %+v", task)
	}

	// Verify args contain exec, -C <worktreePath>, --sandbox workspace-write, prompt
	expectedArgs := []string{"exec", "-C", task.WorktreePath, "--sandbox", "workspace-write", "implement feature X"}
	if len(task.Args) != len(expectedArgs) {
		t.Fatalf("expected %d args, got %d: %v", len(expectedArgs), len(task.Args), task.Args)
	}
	for i, arg := range expectedArgs {
		if task.Args[i] != arg {
			t.Errorf("arg[%d] = %q; want %q", i, task.Args[i], arg)
		}
	}

	if capturedOpts.Cwd != task.WorktreePath {
		t.Errorf("expected captured Cwd to be %q, got %q", task.WorktreePath, capturedOpts.Cwd)
	}
	if capturedOpts.Command != "codex" {
		t.Errorf("expected captured Command to be 'codex', got %q", capturedOpts.Command)
	}

	// Wait for completion
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		tCur, err := taskMgr.GetTask(repoDir, "task-codex-test")
		if err == nil && (tCur.Status == StatusCompleted || tCur.Status == StatusFailed) {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
}

func TestTaskManager_ApplyTask_Success(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	task, err := taskMgr.Dispatch(DispatchOptions{
		ID:          "task-apply-test",
		Profile:     "dev",
		RepoPath:    repoDir,
		Command:     "echo",
		Args:        []string{"agent task done"},
		NewWorktree: true,
	})
	if err != nil {
		t.Fatalf("failed to dispatch task: %v", err)
	}

	// Wait for task to complete
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		tCur, err := taskMgr.GetTask(repoDir, task.ID)
		if err == nil && tCur.Status == StatusCompleted {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}

	// Simulate agent modifications in the worktree:
	// 1. Modify README.md
	readmePath := filepath.Join(task.WorktreePath, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test Repo\n\nAdded feature description\n"), 0644); err != nil {
		t.Fatalf("failed to modify README in worktree: %v", err)
	}

	// 2. Create new file
	featPath := filepath.Join(task.WorktreePath, "feature.txt")
	if err := os.WriteFile(featPath, []byte("new feature content\n"), 0644); err != nil {
		t.Fatalf("failed to create feature.txt in worktree: %v", err)
	}

	// Test DryRun
	dryRes, err := taskMgr.ApplyTask(repoDir, task.ID, ApplyOptions{DryRun: true})
	if err != nil {
		t.Fatalf("expected dry-run apply to succeed, got error: %v", err)
	}
	if !dryRes.DryRun {
		t.Errorf("expected DryRun true, got false")
	}
	if dryRes.FilesApplied != 2 {
		t.Errorf("expected 2 files applied, got %d", dryRes.FilesApplied)
	}

	// Ensure base repo files are UNCHANGED during dry run
	baseReadme, _ := os.ReadFile(filepath.Join(repoDir, "README.md"))
	if strings.Contains(string(baseReadme), "Added feature description") {
		t.Errorf("base README was modified during dry-run")
	}
	if _, err := os.Stat(filepath.Join(repoDir, "feature.txt")); !os.IsNotExist(err) {
		t.Errorf("base feature.txt exists after dry-run")
	}

	// Now execute real ApplyTask
	applyRes, err := taskMgr.ApplyTask(repoDir, task.ID, ApplyOptions{DryRun: false})
	if err != nil {
		t.Fatalf("ApplyTask failed: %v", err)
	}
	if applyRes.DryRun {
		t.Errorf("expected DryRun false, got true")
	}
	if !applyRes.Pruned {
		t.Errorf("expected Pruned true, got false")
	}
	if applyRes.FilesApplied != 2 {
		t.Errorf("expected 2 files applied, got %d", applyRes.FilesApplied)
	}

	// Verify base repo now has the applied changes
	baseReadmeAfter, _ := os.ReadFile(filepath.Join(repoDir, "README.md"))
	if !strings.Contains(string(baseReadmeAfter), "Added feature description") {
		t.Errorf("base README was not updated after apply")
	}
	baseFeatAfter, err := os.ReadFile(filepath.Join(repoDir, "feature.txt"))
	if err != nil || string(baseFeatAfter) != "new feature content\n" {
		t.Errorf("base feature.txt content incorrect: %v, content: %q", err, string(baseFeatAfter))
	}

	// Verify worktree directory is removed from disk
	if _, err := os.Stat(task.WorktreePath); !os.IsNotExist(err) {
		t.Errorf("worktree path %s still exists after apply prune", task.WorktreePath)
	}

	// Verify repeated apply fails cleanly
	_, err = taskMgr.ApplyTask(repoDir, task.ID, ApplyOptions{})
	if err == nil {
		t.Errorf("expected error when applying an already applied task, got nil")
	}
}

func TestTaskManager_ApplyTask_ValidationErrors(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	// 1. Non-existent task
	_, err := taskMgr.ApplyTask(repoDir, "task-does-not-exist", ApplyOptions{})
	if err == nil {
		t.Errorf("expected error for non-existent task, got nil")
	}

	// 2. Task without worktree
	taskNoWT, err := taskMgr.Dispatch(DispatchOptions{
		ID:       "task-no-wt",
		Profile:  "dev",
		RepoPath: repoDir,
		Command:  "echo",
		Args:     []string{"hello"},
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	_, err = taskMgr.ApplyTask(repoDir, taskNoWT.ID, ApplyOptions{})
	if err == nil || !strings.Contains(err.Error(), "does not have an ephemeral worktree") {
		t.Errorf("expected worktree missing error, got: %v", err)
	}

	// 3. Task with empty diff (no changes made in worktree)
	taskEmptyDiff, err := taskMgr.Dispatch(DispatchOptions{
		ID:          "task-empty-diff",
		Profile:     "dev",
		RepoPath:    repoDir,
		Command:     "echo",
		Args:        []string{"noop"},
		NewWorktree: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)

	_, err = taskMgr.ApplyTask(repoDir, taskEmptyDiff.ID, ApplyOptions{})
	if err == nil || !strings.Contains(err.Error(), "no changes to apply") {
		t.Errorf("expected 'no changes to apply' error, got: %v", err)
	}
}

func TestDispatchSandboxNonNative(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	var capturedOpts agent.CreateSessionOptions
	cleanup := agent.SetTestHooks(func(opts agent.CreateSessionOptions) *exec.Cmd {
		capturedOpts = opts
		return exec.Command("sh", "-c", "echo 'claude sandboxed'")
	})
	defer cleanup()

	task, err := taskMgr.Dispatch(DispatchOptions{
		ID:        "task-sandbox-claude",
		Profile:   "dev",
		RepoPath:  repoDir,
		AgentType: "claude",
		Prompt:    "fix syntax error",
		Sandbox:   true,
	})
	if err != nil {
		t.Fatalf("failed to dispatch sandboxed task: %v", err)
	}

	if task.Command != "agent-guard" {
		t.Errorf("expected command 'agent-guard', got %q", task.Command)
	}
	if !task.Sandbox {
		t.Errorf("expected task.Sandbox to be true")
	}

	// Verify args match agent-guard wrap --role developer --repo <repoDir> -- claude -p "fix syntax error"
	expectedPrefix := []string{"wrap", "--role", "developer", "--repo", repoDir, "--", "claude", "-p", "fix syntax error"}
	if len(task.Args) != len(expectedPrefix) {
		t.Fatalf("expected %d args, got %d: %v", len(expectedPrefix), len(task.Args), task.Args)
	}
	for i, arg := range expectedPrefix {
		if task.Args[i] != arg {
			t.Errorf("task.Args[%d] = %q; want %q", i, task.Args[i], arg)
		}
	}

	if capturedOpts.Command != "agent-guard" {
		t.Errorf("expected captured Command to be 'agent-guard', got %q", capturedOpts.Command)
	}
}

func TestDispatchSandboxNativeCodex(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	var capturedOpts agent.CreateSessionOptions
	cleanup := agent.SetTestHooks(func(opts agent.CreateSessionOptions) *exec.Cmd {
		capturedOpts = opts
		return exec.Command("sh", "-c", "echo 'codex native sandboxed'")
	})
	defer cleanup()

	task, err := taskMgr.Dispatch(DispatchOptions{
		ID:        "task-sandbox-codex",
		Profile:   "dev",
		RepoPath:  repoDir,
		AgentType: "codex",
		Prompt:    "run build",
		Sandbox:   true,
	})
	if err != nil {
		t.Fatalf("failed to dispatch sandboxed codex task: %v", err)
	}

	// Codex has native sandbox flags; should NOT be wrapped with agent-guard
	if task.Command != "codex" {
		t.Errorf("expected command 'codex', got %q", task.Command)
	}

	// Verify native sandbox args: exec -C <repoDir> --sandbox workspace-write "run build"
	expectedArgs := []string{"exec", "-C", repoDir, "--sandbox", "workspace-write", "run build"}
	if len(task.Args) != len(expectedArgs) {
		t.Fatalf("expected %d args, got %d: %v", len(expectedArgs), len(task.Args), task.Args)
	}
	for i, arg := range expectedArgs {
		if task.Args[i] != arg {
			t.Errorf("task.Args[%d] = %q; want %q", i, task.Args[i], arg)
		}
	}

	if capturedOpts.Command != "codex" {
		t.Errorf("expected captured Command to be 'codex', got %q", capturedOpts.Command)
	}
}

func TestDispatchSandboxSubshellMutationIntercept(t *testing.T) {
	repoDir := setupTestGitRepo(t)

	// Valid in-repo command
	if err := agent.InterceptSubshellMutation(repoDir, "echo 'hello' > test.txt"); err != nil {
		t.Errorf("expected valid in-repo command to pass, got: %v", err)
	}

	// Subshell mutation attempting write outside target repo root
	outsideFile := filepath.Join(filepath.Dir(repoDir), "escape.txt")
	cmdOutside := fmt.Sprintf("touch %s", outsideFile)
	err := agent.InterceptSubshellMutation(repoDir, cmdOutside)
	if err == nil {
		t.Errorf("expected subshell mutation outside repo root to be intercepted, but got nil")
	}
}

func TestDispatchVerifySuccess(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	task, err := taskMgr.Dispatch(DispatchOptions{
		ID:          "task-verify-success",
		Profile:     "dev",
		RepoPath:    repoDir,
		NewWorktree: true,
		Command:     "sh",
		Args:        []string{"-c", "echo 'feature code' > feature.go"},
		Verify:      "cat feature.go",
	})
	if err != nil {
		t.Fatalf("failed to dispatch task with verify: %v", err)
	}
	if task.ID != "task-verify-success" {
		t.Errorf("expected ID task-verify-success, got %s", task.ID)
	}

	deadline := time.Now().Add(5 * time.Second)
	var completedTask *Task
	for time.Now().Before(deadline) {
		tCur, err := taskMgr.GetTask(repoDir, "task-verify-success")
		if err == nil && (tCur.Status == StatusCompleted || tCur.Status == StatusFailed || tCur.Status == StatusAuditFailed) {
			completedTask = tCur
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if completedTask == nil {
		t.Fatal("timed out waiting for task to complete")
	}

	if completedTask.Status != StatusCompleted {
		t.Fatalf("expected StatusCompleted, got %s (error: %s, output: %s)", completedTask.Status, completedTask.Error, completedTask.VerificationOutput)
	}
	if completedTask.VerificationExitCode != 0 {
		t.Errorf("expected VerificationExitCode 0, got %d", completedTask.VerificationExitCode)
	}
	if !strings.Contains(completedTask.VerificationOutput, "feature code") {
		t.Errorf("expected VerificationOutput to contain 'feature code', got %q", completedTask.VerificationOutput)
	}
	if completedTask.Verify != "cat feature.go" {
		t.Errorf("expected Verify 'cat feature.go', got %q", completedTask.Verify)
	}

	// Verify log contains verification output
	logs, err := taskMgr.GetTaskLogs(repoDir, "task-verify-success", 0)
	if err != nil {
		t.Fatalf("failed to get task logs: %v", err)
	}
	if !strings.Contains(string(logs), "=== Verification (cat feature.go) ===") {
		t.Errorf("expected logs to contain verification header, got: %s", string(logs))
	}
}

func TestDispatchVerifyFailure(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	task, err := taskMgr.Dispatch(DispatchOptions{
		ID:          "task-verify-failure",
		Profile:     "dev",
		RepoPath:    repoDir,
		NewWorktree: true,
		Command:     "sh",
		Args:        []string{"-c", "echo 'broken code' > broken.go"},
		Verify:      "sh -c 'echo \"syntax error in broken.go\" >&2 && exit 42'",
	})
	if err != nil {
		t.Fatalf("failed to dispatch task with failing verify: %v", err)
	}
	if task.ID != "task-verify-failure" {
		t.Errorf("expected ID task-verify-failure, got %s", task.ID)
	}

	deadline := time.Now().Add(5 * time.Second)
	var completedTask *Task
	for time.Now().Before(deadline) {
		tCur, err := taskMgr.GetTask(repoDir, "task-verify-failure")
		if err == nil && (tCur.Status == StatusCompleted || tCur.Status == StatusFailed || tCur.Status == StatusAuditFailed) {
			completedTask = tCur
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if completedTask == nil {
		t.Fatal("timed out waiting for task to complete")
	}

	if completedTask.Status != StatusAuditFailed {
		t.Fatalf("expected StatusAuditFailed, got %s", completedTask.Status)
	}
	if completedTask.VerificationExitCode != 42 {
		t.Errorf("expected VerificationExitCode 42, got %d", completedTask.VerificationExitCode)
	}
	if !strings.Contains(completedTask.VerificationOutput, "syntax error in broken.go") {
		t.Errorf("expected VerificationOutput to capture stderr error, got %q", completedTask.VerificationOutput)
	}
	if !strings.Contains(completedTask.Error, "42") {
		t.Errorf("expected Error to contain exit code 42, got %q", completedTask.Error)
	}

	// ApplyTask MUST reject audit_failed tasks
	_, applyErr := taskMgr.ApplyTask(repoDir, "task-verify-failure", ApplyOptions{})
	if applyErr == nil {
		t.Fatalf("expected ApplyTask to reject audit_failed task, but it succeeded")
	}
	if !strings.Contains(applyErr.Error(), "audit_failed") {
		t.Errorf("expected ApplyTask error to mention 'audit_failed', got %q", applyErr.Error())
	}
}

func TestDispatchGovernanceInjection(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev")

	// Create governance files in repo
	if err := os.WriteFile(filepath.Join(repoDir, "AGENTS.md"), []byte("# Rules\nFollow guidelines."), 0644); err != nil {
		t.Fatalf("failed to write AGENTS.md: %v", err)
	}
	agentDir := filepath.Join(repoDir, ".agent")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("failed to create .agent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "TASK.md"), []byte("## Active Task\n### 📌 Task [03.6]: Test active\n## Log\n"), 0644); err != nil {
		t.Fatalf("failed to write TASK.md: %v", err)
	}

	var capturedOpts agent.CreateSessionOptions
	cleanup := agent.SetTestHooks(func(opts agent.CreateSessionOptions) *exec.Cmd {
		capturedOpts = opts
		return exec.Command("sh", "-c", "echo 'agent done'")
	})
	defer cleanup()

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	task, err := taskMgr.Dispatch(DispatchOptions{
		ID:        "task-gov-inject",
		Profile:   "dev",
		RepoPath:  repoDir,
		AgentType: "claude",
		Prompt:    "implement governance check",
	})
	if err != nil {
		t.Fatalf("failed to dispatch task: %v", err)
	}

	if !task.Governance {
		t.Errorf("expected task.Governance to be true")
	}
	if task.Metadata["governance"] != "injected" {
		t.Errorf("expected task.Metadata['governance'] = 'injected', got %q", task.Metadata["governance"])
	}
	// task.Prompt preserves original prompt
	if task.Prompt != "implement governance check" {
		t.Errorf("expected task.Prompt to preserve original prompt, got %q", task.Prompt)
	}

	// Args and captured session options receive synthesized prompt
	if len(task.Args) != 2 || task.Args[0] != "-p" {
		t.Fatalf("unexpected task.Args: %+v", task.Args)
	}
	if !strings.Contains(task.Args[1], "[PROJECT GOVERNANCE CONTEXT]") {
		t.Errorf("expected prompt arg to contain governance heading, got %q", task.Args[1])
	}
	if !strings.Contains(task.Args[1], "Task [03.6]: Test active") {
		t.Errorf("expected prompt arg to contain active task excerpt, got %q", task.Args[1])
	}
	if !strings.Contains(task.Args[1], "[TASK INSTRUCTION]\nimplement governance check") {
		t.Errorf("expected prompt arg to contain base prompt, got %q", task.Args[1])
	}

	// Environment variable injection
	if capturedOpts.Env["MULTIGRAVITY_GOVERNANCE"] != "true" {
		t.Errorf("expected MULTIGRAVITY_GOVERNANCE=true, got %q", capturedOpts.Env["MULTIGRAVITY_GOVERNANCE"])
	}
	if capturedOpts.Env["MULTIGRAVITY_GOVERNANCE_AGENTS"] == "" {
		t.Errorf("expected MULTIGRAVITY_GOVERNANCE_AGENTS to be set")
	}
	if capturedOpts.Env["MULTIGRAVITY_GOVERNANCE_TASK"] == "" {
		t.Errorf("expected MULTIGRAVITY_GOVERNANCE_TASK to be set")
	}
}

func TestDispatchGovernanceOptOut(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev")

	// Create governance files in repo
	if err := os.WriteFile(filepath.Join(repoDir, "AGENTS.md"), []byte("# Rules\n"), 0644); err != nil {
		t.Fatalf("failed to write AGENTS.md: %v", err)
	}

	var capturedOpts agent.CreateSessionOptions
	cleanup := agent.SetTestHooks(func(opts agent.CreateSessionOptions) *exec.Cmd {
		capturedOpts = opts
		return exec.Command("sh", "-c", "echo 'agent done'")
	})
	defer cleanup()

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	task, err := taskMgr.Dispatch(DispatchOptions{
		ID:           "task-gov-optout",
		Profile:      "dev",
		RepoPath:     repoDir,
		AgentType:    "claude",
		Prompt:       "bypass governance",
		NoGovernance: true,
	})
	if err != nil {
		t.Fatalf("failed to dispatch task: %v", err)
	}

	if task.Governance {
		t.Errorf("expected task.Governance to be false when NoGovernance=true")
	}
	if task.Metadata["governance"] != "disabled" {
		t.Errorf("expected task.Metadata['governance'] = 'disabled', got %q", task.Metadata["governance"])
	}
	if len(task.Args) != 2 || task.Args[1] != "bypass governance" {
		t.Errorf("expected prompt argument to remain untouched: %+v", task.Args)
	}
	if capturedOpts.Env["MULTIGRAVITY_GOVERNANCE"] != "" {
		t.Errorf("expected MULTIGRAVITY_GOVERNANCE to be unset, got %q", capturedOpts.Env["MULTIGRAVITY_GOVERNANCE"])
	}
}
