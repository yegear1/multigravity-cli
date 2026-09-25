package dispatch

import (
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

	// Verify diff inside worktree
	diff, err := taskMgr.GetTaskDiff(repoDir, "task-test-lifecycle", false)
	if err != nil {
		t.Fatalf("failed to get task diff: %v", err)
	}
	// Untracked or modified file in worktree
	_ = diff

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
