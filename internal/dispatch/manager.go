package dispatch

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/agent"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/worktree"
)

var (
	validTaskIDRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`)
	taskMu           sync.RWMutex
	defaultMgr       *TaskManager
	defaultMgrOnce   sync.Once
)

// TaskManager coordinates task dispatching, worktree isolation, log capture, and status queries.
type TaskManager struct {
	agentMgr *agent.Manager
	tasks    map[string]*Task
	mu       sync.RWMutex
	onEvent  func(event string, task *Task)
}

// GetDefaultTaskManager returns the process-wide TaskManager singleton.
func GetDefaultTaskManager() *TaskManager {
	defaultMgrOnce.Do(func() {
		defaultMgr = NewTaskManager(agent.GetDefaultManager())
	})
	return defaultMgr
}

// NewTaskManager creates an isolated TaskManager instance.
func NewTaskManager(agentMgr *agent.Manager) *TaskManager {
	if agentMgr == nil {
		agentMgr = agent.GetDefaultManager()
	}
	return &TaskManager{
		agentMgr: agentMgr,
		tasks:    make(map[string]*Task),
	}
}

// SetEventListener registers a callback for lifecycle events (dispatched, completed, failed, cancelled).
func (m *TaskManager) SetEventListener(cb func(event string, task *Task)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onEvent = cb
}

func (m *TaskManager) emitEvent(event string, task *Task) {
	m.mu.RLock()
	cb := m.onEvent
	m.mu.RUnlock()
	if cb != nil {
		cb(event, task)
	}
}

// ValidateTaskID verifies that the given ID conforms to naming requirements.
func ValidateTaskID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("task ID cannot be empty")
	}
	if strings.Contains(id, "..") {
		return fmt.Errorf("task ID cannot contain '..': %s", id)
	}
	if !validTaskIDRegex.MatchString(id) {
		return fmt.Errorf("invalid task ID %q (must start with alphanumeric and contain only letters, numbers, '.', '_', '-', or '/')", id)
	}
	return nil
}

// GenerateTaskID produces a unique identifier for a dispatched task.
func GenerateTaskID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("task-%d-%s", time.Now().Unix(), hex.EncodeToString(b))
}

// ResolveAgentCommand infers the executable command and arguments given an agent type and prompt.
func ResolveAgentCommand(agentType, command string, args []string, prompt string) (string, []string) {
	cmdStr := strings.TrimSpace(command)
	resArgs := args

	if cmdStr == "" {
		switch strings.ToLower(agentType) {
		case "claude":
			cmdStr = "claude"
			if prompt != "" && len(resArgs) == 0 {
				resArgs = []string{"-p", prompt}
			}
		case "aider":
			cmdStr = "aider"
			if prompt != "" && len(resArgs) == 0 {
				resArgs = []string{"--message", prompt}
			}
		case "opencode":
			cmdStr = "opencode"
			if prompt != "" && len(resArgs) == 0 {
				resArgs = []string{"run", prompt}
			}
		case "agy":
			cmdStr = "agy"
			if prompt != "" && len(resArgs) == 0 {
				resArgs = []string{"-p", prompt, "--output-format", "json"}
			}
		default:
			cmdStr = "claude"
			if prompt != "" && len(resArgs) == 0 {
				resArgs = []string{"-p", prompt}
			}
		}
	} else if prompt != "" && len(resArgs) == 0 {
		base := strings.ToLower(filepath.Base(cmdStr))
		switch base {
		case "claude":
			resArgs = []string{"-p", prompt}
		case "aider":
			resArgs = []string{"--message", prompt}
		case "agy":
			resArgs = []string{"-p", prompt}
		case "opencode":
			resArgs = []string{"run", prompt}
		}
	}

	return cmdStr, resArgs
}

func getTasksDir(repoRoot string) string {
	if repoRoot != "" {
		return filepath.Join(repoRoot, ".multigravity", "tasks")
	}
	return filepath.Join(config.GetMultigravityHome(), ".tasks")
}

func getManifestPath(repoRoot string) string {
	return filepath.Join(getTasksDir(repoRoot), "tasks.json")
}

func (m *TaskManager) loadManifest(repoRoot string) (*TaskManifest, error) {
	path := getManifestPath(repoRoot)
	manifest := &TaskManifest{
		Version: 1,
		Tasks:   make(map[string]Task),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return manifest, nil
		}
		return nil, fmt.Errorf("failed to read task manifest: %w", err)
	}

	if err := json.Unmarshal(data, manifest); err != nil {
		return nil, fmt.Errorf("failed to parse task manifest: %w", err)
	}
	if manifest.Tasks == nil {
		manifest.Tasks = make(map[string]Task)
	}
	return manifest, nil
}

func (m *TaskManager) saveManifest(repoRoot string, manifest *TaskManifest) error {
	path := getManifestPath(repoRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize task manifest: %w", err)
	}

	return os.WriteFile(path, append(data, '\n'), 0644)
}

func saveTaskFile(taskDir string, task *Task) error {
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(taskDir, "task.json"), append(data, '\n'), 0644)
}

// Dispatch launches a new agent task with profile isolation, worktree environment, and log capture.
func (m *TaskManager) Dispatch(opts DispatchOptions) (*Task, error) {
	if opts.Profile == "" {
		return nil, errors.New("profile name is required for task dispatch")
	}
	if err := config.ValidateProfileName(opts.Profile); err != nil {
		return nil, fmt.Errorf("invalid profile name: %w", err)
	}
	if !profile.ProfileExists(opts.Profile) {
		return nil, fmt.Errorf("profile %q does not exist", opts.Profile)
	}

	taskID := strings.TrimSpace(opts.ID)
	if taskID == "" {
		taskID = GenerateTaskID()
	} else if err := ValidateTaskID(taskID); err != nil {
		return nil, err
	}

	// Resolve repo root
	repoDir := opts.RepoPath
	if repoDir == "" {
		cwd, err := os.Getwd()
		if err == nil {
			repoDir = cwd
		}
	}

	var repoRoot string
	if repoDir != "" {
		root, err := worktree.FindRepoRoot(repoDir)
		if err == nil {
			repoRoot = root
		}
	}

	var wtID, wtPath, branch, baseCommit string
	execDir := opts.Cwd

	// Worktree setup
	if opts.NewWorktree {
		if repoRoot == "" {
			return nil, errors.New("cannot create worktree: working directory is not inside a git repository")
		}
		wtBranch := opts.Branch
		if wtBranch == "" {
			wtBranch = "multigravity/" + taskID
		}

		wt, err := worktree.CreateWorktree(worktree.CreateOptions{
			ID:         taskID,
			RepoPath:   repoRoot,
			Branch:     wtBranch,
			BaseCommit: opts.BaseCommit,
			Profile:    opts.Profile,
			TaskID:     taskID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create worktree for task: %w", err)
		}

		wtID = wt.ID
		wtPath = wt.Path
		branch = wt.Branch
		baseCommit = wt.BaseCommit
		execDir = wt.Path
	} else if opts.WorktreeID != "" {
		if repoRoot == "" {
			return nil, errors.New("cannot associate worktree: not in a git repository")
		}
		wt, err := worktree.GetWorktree(repoRoot, opts.WorktreeID)
		if err != nil {
			return nil, fmt.Errorf("failed to get worktree %q: %w", opts.WorktreeID, err)
		}
		wtID = wt.ID
		wtPath = wt.Path
		branch = wt.Branch
		baseCommit = wt.BaseCommit
		if execDir == "" {
			execDir = wt.Path
		}
	} else if execDir == "" {
		if repoRoot != "" {
			execDir = repoRoot
		} else {
			cwd, _ := os.Getwd()
			execDir = cwd
		}
	}

	// Exclude .multigravity/ from git if inside a repo
	if repoRoot != "" {
		_ = worktree.EnsureGitExclude(repoRoot, ".multigravity/")
	}

	// Deduce command and arguments
	cmdStr, args := ResolveAgentCommand(opts.AgentType, opts.Command, opts.Args, opts.Prompt)
	agentType := opts.AgentType
	if agentType == "" {
		agentType = agent.DetectAgentType(cmdStr)
	}

	tasksDir := getTasksDir(repoRoot)
	taskDir := filepath.Join(tasksDir, taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create task directory: %w", err)
	}

	logPath := filepath.Join(taskDir, "run.log")
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		_ = f.Close()
	}
	now := time.Now().UTC()

	task := &Task{
		ID:           taskID,
		Profile:      opts.Profile,
		Status:       StatusRunning,
		Prompt:       opts.Prompt,
		AgentType:    agentType,
		Command:      cmdStr,
		Args:         args,
		RepoPath:     repoRoot,
		WorktreeID:   wtID,
		WorktreePath: wtPath,
		Branch:       branch,
		BaseCommit:   baseCommit,
		ExitCode:     -1,
		CreatedAt:    now,
		StartedAt:    now,
		LogPath:      logPath,
		Metadata:     opts.Metadata,
	}

	// Persist initial state
	_ = saveTaskFile(taskDir, task)

	m.mu.Lock()
	m.tasks[taskID] = task
	m.mu.Unlock()

	taskMu.Lock()
	manifest, _ := m.loadManifest(repoRoot)
	if manifest != nil {
		manifest.Tasks[taskID] = *task
		_ = m.saveManifest(repoRoot, manifest)
	}
	taskMu.Unlock()

	// Launch agent session
	sessOpts := agent.CreateSessionOptions{
		Profile:    opts.Profile,
		AgentType:  agentType,
		Command:    cmdStr,
		Args:       args,
		Cwd:        execDir,
		WorktreeID: wtID,
		Env:        opts.Env,
		GatewayURL: opts.GatewayURL,
		GatewayKey: opts.GatewayKey,
		Detached:   true,
	}

	inst, err := m.agentMgr.StartSession(sessOpts)
	if err != nil {
		task.Status = StatusFailed
		task.Error = err.Error()
		ended := time.Now().UTC()
		task.EndedAt = &ended
		_ = saveTaskFile(taskDir, task)

		taskMu.Lock()
		if manifest, mErr := m.loadManifest(repoRoot); mErr == nil {
			manifest.Tasks[taskID] = *task
			_ = m.saveManifest(repoRoot, manifest)
		}
		taskMu.Unlock()

		m.emitEvent("failed", task)
		return task, fmt.Errorf("failed to start agent session: %w", err)
	}

	task.SessionID = inst.GetInfo().ID
	task.PID = inst.GetInfo().PID
	_ = saveTaskFile(taskDir, task)

	m.emitEvent("dispatched", task)

	// Stream log chunks to disk
	go m.streamLogsToFile(inst, logPath)

	// Monitor session lifecycle
	go m.watchSession(inst, task, taskDir, repoRoot)

	return task, nil
}

func (m *TaskManager) streamLogsToFile(inst *agent.SessionInstance, logPath string) {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	ch, unsub := inst.Subscribe()
	defer unsub()

	// Write any initial output buffered before subscription was active
	if initial := inst.GetOutput(0); len(initial) > 0 {
		_, _ = f.Write(initial)
		_ = f.Sync()
	}

	for chunk := range ch {
		if chunk.Data != "" {
			_, _ = io.WriteString(f, chunk.Data)
			_ = f.Sync()
		}
	}
}

func (m *TaskManager) watchSession(inst *agent.SessionInstance, task *Task, taskDir, repoRoot string) {
	// Wait until process exits
	inst.Wait(0)

	info := inst.GetInfo()
	ended := time.Now().UTC()

	m.mu.Lock()
	task.EndedAt = &ended
	task.ExitCode = info.ExitCode
	task.DurationSeconds = ended.Sub(task.StartedAt).Seconds()

	if task.Status != StatusCancelled {
		if info.Status == agent.StatusStopped {
			task.Status = StatusCancelled
		} else if info.ExitCode == 0 {
			task.Status = StatusCompleted
		} else {
			task.Status = StatusFailed
		}
	}

	// Capture latest head commit from worktree if present
	if task.WorktreePath != "" {
		if head, err := worktree.GetHeadCommit(task.WorktreePath); err == nil {
			task.HeadCommit = head
		}
	}
	m.mu.Unlock()

	// Persist finalized task state
	_ = saveTaskFile(taskDir, task)

	taskMu.Lock()
	if manifest, err := m.loadManifest(repoRoot); err == nil {
		manifest.Tasks[task.ID] = *task
		_ = m.saveManifest(repoRoot, manifest)
	}
	taskMu.Unlock()

	m.emitEvent(string(task.Status), task)
}

// GetTask retrieves a task by ID from memory, task directory, or manifest.
func (m *TaskManager) GetTask(repoPath, id string) (*Task, error) {
	if err := ValidateTaskID(id); err != nil {
		return nil, err
	}

	m.mu.RLock()
	t, found := m.tasks[id]
	m.mu.RUnlock()
	if found {
		return t, nil
	}

	var repoRoot string
	if repoPath != "" {
		if r, err := worktree.FindRepoRoot(repoPath); err == nil {
			repoRoot = r
		}
	} else {
		if cwd, err := os.Getwd(); err == nil {
			if r, err := worktree.FindRepoRoot(cwd); err == nil {
				repoRoot = r
			}
		}
	}

	// Look in task directory
	taskFile := filepath.Join(getTasksDir(repoRoot), id, "task.json")
	if data, err := os.ReadFile(taskFile); err == nil {
		var task Task
		if err := json.Unmarshal(data, &task); err == nil {
			m.mu.Lock()
			m.tasks[id] = &task
			m.mu.Unlock()
			return &task, nil
		}
	}

	// Look in manifest
	taskMu.Lock()
	defer taskMu.Unlock()
	manifest, err := m.loadManifest(repoRoot)
	if err == nil {
		if task, exists := manifest.Tasks[id]; exists {
			m.mu.Lock()
			m.tasks[id] = &task
			m.mu.Unlock()
			return &task, nil
		}
	}

	return nil, fmt.Errorf("task %q not found", id)
}

// ListTasks returns all tasks matching the specified filter, ordered newest first.
func (m *TaskManager) ListTasks(repoPath string, filter TaskFilter) ([]Task, error) {
	var repoRoot string
	if repoPath != "" {
		if r, err := worktree.FindRepoRoot(repoPath); err == nil {
			repoRoot = r
		}
	} else {
		if cwd, err := os.Getwd(); err == nil {
			if r, err := worktree.FindRepoRoot(cwd); err == nil {
				repoRoot = r
			}
		}
	}

	tasksMap := make(map[string]Task)

	// 1. Read from persistent manifest
	taskMu.Lock()
	if manifest, err := m.loadManifest(repoRoot); err == nil {
		for id, t := range manifest.Tasks {
			tasksMap[id] = t
		}
	}
	taskMu.Unlock()

	// 2. Scan tasks directory in case any task.json was written directly
	tasksDir := getTasksDir(repoRoot)
	if entries, err := os.ReadDir(tasksDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				id := entry.Name()
				if _, ok := tasksMap[id]; !ok {
					tFile := filepath.Join(tasksDir, id, "task.json")
					if data, err := os.ReadFile(tFile); err == nil {
						var t Task
						if err := json.Unmarshal(data, &t); err == nil {
							tasksMap[id] = t
						}
					}
				}
			}
		}
	}

	// 3. Overlay in-memory live tasks
	m.mu.RLock()
	for id, t := range m.tasks {
		tasksMap[id] = *t
	}
	m.mu.RUnlock()

	results := make([]Task, 0, len(tasksMap))
	for _, t := range tasksMap {
		if filter.Profile != "" && t.Profile != filter.Profile {
			continue
		}
		if filter.Status != "" && t.Status != filter.Status {
			continue
		}
		if filter.AgentType != "" && t.AgentType != filter.AgentType {
			continue
		}
		if filter.WorktreeID != "" && t.WorktreeID != filter.WorktreeID {
			continue
		}
		results = append(results, t)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results, nil
}

// CancelTask terminates an active task's running agent session.
func (m *TaskManager) CancelTask(repoPath, id string, force bool) error {
	task, err := m.GetTask(repoPath, id)
	if err != nil {
		return err
	}

	if task.Status != StatusRunning {
		return fmt.Errorf("task %q is not running (status: %s)", id, task.Status)
	}

	if task.SessionID != "" {
		if force {
			_ = m.agentMgr.KillSession(task.SessionID)
		} else {
			_ = m.agentMgr.StopSession(task.SessionID)
		}
	}

	m.mu.Lock()
	task.Status = StatusCancelled
	ended := time.Now().UTC()
	task.EndedAt = &ended
	task.DurationSeconds = ended.Sub(task.StartedAt).Seconds()
	m.mu.Unlock()

	repoRoot := task.RepoPath
	taskDir := filepath.Join(getTasksDir(repoRoot), id)
	_ = saveTaskFile(taskDir, task)

	taskMu.Lock()
	if manifest, err := m.loadManifest(repoRoot); err == nil {
		manifest.Tasks[id] = *task
		_ = m.saveManifest(repoRoot, manifest)
	}
	taskMu.Unlock()

	m.emitEvent("cancelled", task)
	return nil
}

// GetTaskLogs retrieves buffered execution logs for a task.
func (m *TaskManager) GetTaskLogs(repoPath, id string, tailBytes int) ([]byte, error) {
	task, err := m.GetTask(repoPath, id)
	if err != nil {
		return nil, err
	}

	logPath := task.LogPath
	if logPath == "" {
		logPath = filepath.Join(getTasksDir(task.RepoPath), id, "run.log")
	}

	data, err := os.ReadFile(logPath)
	if err != nil || len(data) == 0 {
		// Fallback to active session buffer if file not yet written or empty
		if task.SessionID != "" {
			if out, oErr := m.agentMgr.GetSessionOutput(task.SessionID, tailBytes); oErr == nil && len(out) > 0 {
				return out, nil
			}
		}
		if err != nil {
			if os.IsNotExist(err) {
				return []byte{}, nil
			}
			return nil, fmt.Errorf("failed to read log file: %w", err)
		}
	}

	if tailBytes <= 0 || tailBytes >= len(data) {
		return data, nil
	}

	start := len(data) - tailBytes
	return data[start:], nil
}

// GetTaskDiff returns git diff of changes introduced in the task's worktree.
func (m *TaskManager) GetTaskDiff(repoPath, id string, statOnly bool) (string, error) {
	task, err := m.GetTask(repoPath, id)
	if err != nil {
		return "", err
	}

	if task.WorktreePath == "" {
		return "", nil
	}

	base := task.BaseCommit
	return worktree.GitDiff(task.WorktreePath, base, statOnly, false)
}

// GetTaskStructuredDiff parses the git diff of the task's worktree into a structured model.
func (m *TaskManager) GetTaskStructuredDiff(repoPath, id string) (*StructuredDiff, error) {
	raw, err := m.GetTaskDiff(repoPath, id, false)
	if err != nil {
		return nil, err
	}
	return ParseUnifiedDiff(raw), nil
}

// GetTaskFiles returns the list of modified files with their stats for the given task.
func (m *TaskManager) GetTaskFiles(repoPath, id string) ([]DiffFile, error) {
	sd, err := m.GetTaskStructuredDiff(repoPath, id)
	if err != nil {
		return nil, err
	}
	return sd.Files, nil
}

// GetDashboardSummary aggregates execution metrics, counts, and recent tasks for GUI dashboards.
func (m *TaskManager) GetDashboardSummary(repoPath string) (*TaskDashboardSummary, error) {
	tasks, err := m.ListTasks(repoPath, TaskFilter{})
	if err != nil {
		return nil, err
	}

	summary := &TaskDashboardSummary{
		Total:       len(tasks),
		RecentTasks: make([]Task, 0),
		ActiveTasks: make([]Task, 0),
		GeneratedAt: time.Now().UTC(),
	}

	for _, t := range tasks {
		switch t.Status {
		case StatusRunning:
			summary.Running++
			summary.ActiveTasks = append(summary.ActiveTasks, t)
		case StatusCompleted:
			summary.Completed++
		case StatusFailed:
			summary.Failed++
		case StatusCancelled:
			summary.Cancelled++
		}
	}

	// Limit RecentTasks to up to 15 newest
	limit := 15
	if len(tasks) < limit {
		limit = len(tasks)
	}
	summary.RecentTasks = tasks[:limit]

	return summary, nil
}

// PruneTasks removes completed/failed/cancelled tasks older than maxAge.
func (m *TaskManager) PruneTasks(repoPath string, maxAge time.Duration) (int, error) {
	var repoRoot string
	if repoPath != "" {
		if r, err := worktree.FindRepoRoot(repoPath); err == nil {
			repoRoot = r
		}
	} else {
		if cwd, err := os.Getwd(); err == nil {
			if r, err := worktree.FindRepoRoot(cwd); err == nil {
				repoRoot = r
			}
		}
	}

	now := time.Now().UTC()
	pruned := 0

	taskMu.Lock()
	defer taskMu.Unlock()

	manifest, err := m.loadManifest(repoRoot)
	if err != nil {
		return 0, err
	}

	tasksDir := getTasksDir(repoRoot)

	for id, t := range manifest.Tasks {
		if t.Status != StatusRunning && t.EndedAt != nil {
			if now.Sub(*t.EndedAt) > maxAge {
				delete(manifest.Tasks, id)
				m.mu.Lock()
				delete(m.tasks, id)
				m.mu.Unlock()

				taskDir := filepath.Join(tasksDir, id)
				_ = os.RemoveAll(taskDir)
				pruned++
			}
		}
	}

	if pruned > 0 {
		_ = m.saveManifest(repoRoot, manifest)
	}

	return pruned, nil
}

// DeleteTask removes a specific task record, logs, and optionally its worktree.
func (m *TaskManager) DeleteTask(repoPath, id string, removeWorktree bool) error {
	task, err := m.GetTask(repoPath, id)
	if err != nil {
		return err
	}

	if task.Status == StatusRunning {
		return fmt.Errorf("cannot delete running task %q; cancel it first", id)
	}

	repoRoot := task.RepoPath
	taskMu.Lock()
	manifest, err := m.loadManifest(repoRoot)
	if err == nil {
		delete(manifest.Tasks, id)
		_ = m.saveManifest(repoRoot, manifest)
	}
	taskMu.Unlock()

	m.mu.Lock()
	delete(m.tasks, id)
	m.mu.Unlock()

	taskDir := filepath.Join(getTasksDir(repoRoot), id)
	_ = os.RemoveAll(taskDir)

	if removeWorktree && task.WorktreeID != "" && repoRoot != "" {
		_ = worktree.RemoveWorktree(repoRoot, task.WorktreeID, worktree.RemoveOptions{
			Force:        true,
			DeleteBranch: true,
		})
	}

	m.emitEvent("deleted", task)
	return nil
}
