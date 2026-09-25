package dispatch

import (
	"time"
)

// TaskStatus represents the lifecycle state of a dispatched agent task.
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
	StatusCancelled TaskStatus = "cancelled"
)

// Task models a persistent dispatched agent task execution with its worktree, profile, and logs.
type Task struct {
	ID              string            `json:"id"`
	Profile         string            `json:"profile"`
	Status          TaskStatus        `json:"status"`
	Prompt          string            `json:"prompt,omitempty"`
	AgentType       string            `json:"agent_type,omitempty"`
	Command         string            `json:"command"`
	Args            []string          `json:"args,omitempty"`
	RepoPath        string            `json:"repo_path,omitempty"`
	WorktreeID      string            `json:"worktree_id,omitempty"`
	WorktreePath    string            `json:"worktree_path,omitempty"`
	Branch          string            `json:"branch,omitempty"`
	BaseCommit      string            `json:"base_commit,omitempty"`
	HeadCommit      string            `json:"head_commit,omitempty"`
	SessionID       string            `json:"session_id,omitempty"`
	PID             int               `json:"pid,omitempty"`
	ExitCode        int               `json:"exit_code"`
	CreatedAt       time.Time         `json:"created_at"`
	StartedAt       time.Time         `json:"started_at"`
	EndedAt         *time.Time        `json:"ended_at,omitempty"`
	DurationSeconds float64           `json:"duration_seconds,omitempty"`
	LogPath         string            `json:"log_path,omitempty"`
	Error           string            `json:"error,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// DispatchOptions specifies parameters for launching a new dispatched agent task.
type DispatchOptions struct {
	ID          string            `json:"id,omitempty"`
	Profile     string            `json:"profile"`
	RepoPath    string            `json:"repo_path,omitempty"`
	Prompt      string            `json:"prompt,omitempty"`
	AgentType   string            `json:"agent_type,omitempty"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	WorktreeID  string            `json:"worktree_id,omitempty"`
	NewWorktree bool              `json:"new_worktree,omitempty"`
	Branch      string            `json:"branch,omitempty"`
	BaseCommit  string            `json:"base_commit,omitempty"`
	Cwd         string            `json:"cwd,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	GatewayURL  string            `json:"gateway_url,omitempty"`
	GatewayKey  string            `json:"gateway_key,omitempty"`
	Detached    bool              `json:"detached,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// TaskFilter defines criteria for querying and listing tasks.
type TaskFilter struct {
	Profile    string     `json:"profile,omitempty"`
	Status     TaskStatus `json:"status,omitempty"`
	AgentType  string     `json:"agent_type,omitempty"`
	WorktreeID string     `json:"worktree_id,omitempty"`
}

// TaskManifest represents the serializable index in .multigravity/tasks/tasks.json.
type TaskManifest struct {
	Version int             `json:"version"`
	Tasks   map[string]Task `json:"tasks"`
}
