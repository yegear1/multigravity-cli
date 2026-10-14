package dispatch

import (
	"time"
)

// TaskStatus represents the lifecycle state of a dispatched agent task.
type TaskStatus string

const (
	StatusPending     TaskStatus = "pending"
	StatusRunning     TaskStatus = "running"
	StatusCompleted   TaskStatus = "completed"
	StatusFailed      TaskStatus = "failed"
	StatusCancelled   TaskStatus = "cancelled"
	StatusAuditFailed TaskStatus = "audit_failed"
)

// Task models a persistent dispatched agent task execution with its worktree, profile, and logs.
type Task struct {
	ID                   string            `json:"id"`
	Profile              string            `json:"profile"`
	Status               TaskStatus        `json:"status"`
	Prompt               string            `json:"prompt,omitempty"`
	AgentType            string            `json:"agent_type,omitempty"`
	Command              string            `json:"command"`
	Args                 []string          `json:"args,omitempty"`
	RepoPath             string            `json:"repo_path,omitempty"`
	WorktreeID           string            `json:"worktree_id,omitempty"`
	WorktreePath         string            `json:"worktree_path,omitempty"`
	Sandbox              bool              `json:"sandbox,omitempty"`
	Role                 string            `json:"role,omitempty"`
	Verify               string            `json:"verify,omitempty"`
	VerificationOutput   string            `json:"verification_output,omitempty"`
	VerificationExitCode int               `json:"verification_exit_code,omitempty"`
	Governance           bool              `json:"governance,omitempty"`
	Branch               string            `json:"branch,omitempty"`
	BaseCommit           string            `json:"base_commit,omitempty"`
	HeadCommit           string            `json:"head_commit,omitempty"`
	SessionID            string            `json:"session_id,omitempty"`
	PID                  int               `json:"pid,omitempty"`
	ExitCode             int               `json:"exit_code"`
	CreatedAt            time.Time         `json:"created_at"`
	StartedAt            time.Time         `json:"started_at"`
	EndedAt              *time.Time        `json:"ended_at,omitempty"`
	DurationSeconds      float64           `json:"duration_seconds,omitempty"`
	LogPath              string            `json:"log_path,omitempty"`
	Error                string            `json:"error,omitempty"`
	Metadata             map[string]string `json:"metadata,omitempty"`
}

// DispatchOptions specifies parameters for launching a new dispatched agent task.
type DispatchOptions struct {
	ID           string            `json:"id,omitempty"`
	Profile      string            `json:"profile"`
	RepoPath     string            `json:"repo_path,omitempty"`
	Prompt       string            `json:"prompt,omitempty"`
	AgentType    string            `json:"agent_type,omitempty"`
	Command      string            `json:"command,omitempty"`
	Args         []string          `json:"args,omitempty"`
	Sandbox      bool              `json:"sandbox,omitempty"`
	Role         string            `json:"role,omitempty"`
	Verify       string            `json:"verify,omitempty"`
	NoGovernance bool              `json:"no_governance,omitempty"`
	WorktreeID   string            `json:"worktree_id,omitempty"`
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

// DiffLineType categorizes a line in a unified diff chunk.
type DiffLineType string

const (
	DiffLineContext  DiffLineType = "context"
	DiffLineAddition DiffLineType = "addition"
	DiffLineDeletion DiffLineType = "deletion"
	DiffLineHeader   DiffLineType = "header"
)

// DiffLine represents a single line inside a diff hunk with its type and line numbers.
type DiffLine struct {
	Type      DiffLineType `json:"type"`
	Content   string       `json:"content"`
	OldLineNo int          `json:"old_line_no,omitempty"`
	NewLineNo int          `json:"new_line_no,omitempty"`
}

// DiffHunk models a chunk of modifications within a file diff.
type DiffHunk struct {
	Header   string     `json:"header"`
	OldStart int        `json:"old_start"`
	OldLines int        `json:"old_lines"`
	NewStart int        `json:"new_start"`
	NewLines int        `json:"new_lines"`
	Lines    []DiffLine `json:"lines"`
}

// DiffFileStatus indicates how a file was changed.
type DiffFileStatus string

const (
	DiffFileModified DiffFileStatus = "modified"
	DiffFileAdded    DiffFileStatus = "added"
	DiffFileDeleted  DiffFileStatus = "deleted"
	DiffFileRenamed  DiffFileStatus = "renamed"
)

// DiffFile holds the structured diff for a single file including hunks and stats.
type DiffFile struct {
	OldPath   string         `json:"old_path,omitempty"`
	NewPath   string         `json:"new_path"`
	Status    DiffFileStatus `json:"status"`
	Binary    bool           `json:"binary,omitempty"`
	Additions int            `json:"additions"`
	Deletions int            `json:"deletions"`
	Hunks     []DiffHunk     `json:"hunks,omitempty"`
}

// DiffSummary consolidates changes across all files in a diff.
type DiffSummary struct {
	FilesChanged int `json:"files_changed"`
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
}

// StructuredDiff represents a complete parsed unified git diff.
type StructuredDiff struct {
	Summary DiffSummary `json:"summary"`
	Files   []DiffFile  `json:"files"`
	Raw     string      `json:"raw,omitempty"`
}

// TaskDashboardSummary models aggregated execution statistics for UI dashboards.
type TaskDashboardSummary struct {
	Total        int       `json:"total"`
	Running      int       `json:"running"`
	Completed    int       `json:"completed"`
	Failed       int       `json:"failed"`
	Cancelled    int       `json:"cancelled"`
	AuditFailed  int       `json:"audit_failed,omitempty"`
	RecentTasks  []Task    `json:"recent_tasks"`
	ActiveTasks  []Task    `json:"active_tasks"`
	GeneratedAt  time.Time `json:"generated_at"`
}

// ApplyOptions specifies configuration for applying a task's ephemeral worktree diff into the base repository.
type ApplyOptions struct {
	RepoPath string `json:"repo_path,omitempty"`
	DryRun   bool   `json:"dry_run,omitempty"`
	NoPrune  bool   `json:"no_prune,omitempty"`
	ThreeWay bool   `json:"three_way,omitempty"`
}

// ApplyResult represents the outcome of applying a task diff to the base repository working tree.
type ApplyResult struct {
	TaskID       string          `json:"task_id"`
	WorktreeID   string          `json:"worktree_id"`
	BaseRepoPath string          `json:"base_repo_path"`
	FilesApplied int             `json:"files_applied"`
	Additions    int             `json:"additions"`
	Deletions    int             `json:"deletions"`
	Structured   *StructuredDiff `json:"structured,omitempty"`
	DryRun       bool            `json:"dry_run"`
	Pruned       bool            `json:"pruned"`
}
