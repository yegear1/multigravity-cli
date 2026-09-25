package worktree

import (
	"time"
)

// Worktree represents an ephemeral or managed git worktree associated with an agent or task.
type Worktree struct {
	ID         string            `json:"id"`
	RepoPath   string            `json:"repo_path"`
	Path       string            `json:"path"`
	Branch     string            `json:"branch"`
	BaseCommit string            `json:"base_commit"`
	HeadCommit string            `json:"head_commit,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	Profile    string            `json:"profile,omitempty"`
	TaskID     string            `json:"task_id,omitempty"`
	Status     string            `json:"status"` // "active", "detached", "dirty", "missing"
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// WorktreeStatus contains status telemetry for a worktree.
type WorktreeStatus struct {
	Worktree       Worktree `json:"worktree"`
	IsClean        bool     `json:"is_clean"`
	ModifiedFiles  []string `json:"modified_files,omitempty"`
	UntrackedFiles []string `json:"untracked_files,omitempty"`
	CommitsAhead   int      `json:"commits_ahead"`
	CommitsBehind  int      `json:"commits_behind"`
}

// CreateOptions defines parameters for creating a new worktree.
type CreateOptions struct {
	ID         string            `json:"id"`
	RepoPath   string            `json:"repo_path,omitempty"`
	Branch     string            `json:"branch,omitempty"`
	BaseCommit string            `json:"base_commit,omitempty"`
	TargetDir  string            `json:"target_dir,omitempty"`
	Profile    string            `json:"profile,omitempty"`
	TaskID     string            `json:"task_id,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CopyFiles  []string          `json:"copy_files,omitempty"`
	LinkPaths  []string          `json:"link_paths,omitempty"`
}

// RemoveOptions defines parameters for removing an existing worktree.
type RemoveOptions struct {
	Force        bool `json:"force"`
	DeleteBranch bool `json:"delete_branch"`
}

// DiffOptions defines options for inspecting diffs between worktree and base.
type DiffOptions struct {
	Base     string `json:"base,omitempty"`
	StatOnly bool   `json:"stat_only"`
	Cached   bool   `json:"cached"`
}

// WorktreeManifest models the persistent metadata store in .git/multigravity-worktrees.json.
type WorktreeManifest struct {
	Version   int                 `json:"version"`
	Worktrees map[string]Worktree `json:"worktrees"`
}
