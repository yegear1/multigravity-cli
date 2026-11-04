package dispatch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/agent"
	"github.com/ye-dev/multigravity-cli/internal/worktree"
)

// TaskWorkspace represents the resolved workspace environment for a dispatched task.
type TaskWorkspace struct {
	Mode       WorkspaceMode
	WorktreeID string
	Path       string
	Branch     string
	BaseCommit string
	ExecDir    string
}

// setupTaskWorkspace prepares the filesystem isolation for a task based on opts.Workspace (worktree, mirror, inherit).
func setupTaskWorkspace(repoRoot, taskID string, opts DispatchOptions) (*TaskWorkspace, error) {
	execDir := opts.Cwd

	workspaceMode := strings.ToLower(strings.TrimSpace(opts.Workspace))
	if workspaceMode == "" {
		if opts.NewWorktree {
			workspaceMode = string(WorkspaceWorktree)
		} else if opts.WorktreeID != "" {
			workspaceMode = string(WorkspaceWorktree)
		} else if opts.ReadOnly || agent.IsReadOnlyRole(opts.Role) {
			workspaceMode = string(WorkspaceInherit)
		} else {
			workspaceMode = string(WorkspaceInherit)
		}
	}

	switch WorkspaceMode(workspaceMode) {
	case WorkspaceWorktree, WorkspaceMirror, WorkspaceInherit:
	default:
		return nil, fmt.Errorf("invalid workspace mode %q: must be one of %q, %q, %q",
			opts.Workspace, WorkspaceWorktree, WorkspaceMirror, WorkspaceInherit)
	}

	ws := &TaskWorkspace{
		Mode: WorkspaceMode(workspaceMode),
	}

	if ws.Mode == WorkspaceWorktree {
		if opts.WorktreeID != "" {
			if repoRoot == "" {
				return nil, errors.New("cannot associate worktree: not in a git repository")
			}
			wt, err := worktree.GetWorktree(repoRoot, opts.WorktreeID)
			if err != nil {
				return nil, fmt.Errorf("failed to get worktree %q: %w", opts.WorktreeID, err)
			}
			ws.WorktreeID = wt.ID
			ws.Path = wt.Path
			ws.Branch = wt.Branch
			ws.BaseCommit = wt.BaseCommit
			if execDir == "" {
				execDir = wt.Path
			}
		} else if opts.NewWorktree || opts.Workspace == string(WorkspaceWorktree) {
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

			ws.WorktreeID = wt.ID
			ws.Path = wt.Path
			ws.Branch = wt.Branch
			ws.BaseCommit = wt.BaseCommit
			execDir = wt.Path
		} else if execDir == "" {
			if repoRoot != "" {
				execDir = repoRoot
			} else {
				cwd, _ := os.Getwd()
				execDir = cwd
			}
		}
	} else if ws.Mode == WorkspaceMirror {
		if repoRoot == "" {
			return nil, errors.New("cannot create mirror workspace: working directory is not inside a git repository")
		}
		mirrorPath, err := createMirrorWorkspace(repoRoot, taskID)
		if err != nil {
			return nil, fmt.Errorf("failed to create mirror workspace: %w", err)
		}
		ws.WorktreeID = taskID
		ws.Path = mirrorPath
		execDir = mirrorPath
	} else {
		// WorkspaceInherit: zero-I/O in-place execution in repoRoot (or opts.Cwd)
		if execDir == "" {
			if repoRoot != "" {
				execDir = repoRoot
			} else {
				cwd, _ := os.Getwd()
				execDir = cwd
			}
		}
	}

	ws.ExecDir = execDir

	if ws.Mode == WorkspaceWorktree && ws.ExecDir != "" {
		worktree.CleanWorktreeIndexLock(ws.ExecDir)
	}

	// Exclude .multigravity/ from git if inside a repo
	if repoRoot != "" {
		_ = worktree.EnsureGitExclude(repoRoot, ".multigravity/")
	}

	return ws, nil
}

// createMirrorWorkspace creates an ephemeral fast mirror workspace using symlinks for git repositories.
func createMirrorWorkspace(repoRoot, taskID string) (string, error) {
	mirrorDir := filepath.Join(repoRoot, ".multigravity", "mirrors", taskID)
	if err := os.MkdirAll(mirrorDir, 0755); err != nil {
		return "", err
	}
	_ = worktree.EnsureGitExclude(repoRoot, ".multigravity/")

	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		_ = os.RemoveAll(mirrorDir)
		return "", fmt.Errorf("failed to read repo root for mirror: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == ".git" || name == ".multigravity" {
			continue
		}
		src := filepath.Join(repoRoot, name)
		dst := filepath.Join(mirrorDir, name)
		_ = os.Symlink(src, dst)
	}

	// Symlink .git so git status and git queries work inside mirror
	gitSrc := filepath.Join(repoRoot, ".git")
	if _, err := os.Stat(gitSrc); err == nil {
		gitDst := filepath.Join(mirrorDir, ".git")
		_ = os.Symlink(gitSrc, gitDst)
	}

	_ = worktree.AutoLinkDependencies(repoRoot, mirrorDir)
	_ = worktree.IsolateWorktreeBuildArtifacts(repoRoot, mirrorDir)
	_ = worktree.LinkGlobalCompilerCache(repoRoot, mirrorDir)

	return mirrorDir, nil
}

// cleanupMirrorWorkspace removes the ephemeral mirror workspace directory.
func cleanupMirrorWorkspace(repoRoot, taskID string) error {
	mirrorDir := filepath.Join(repoRoot, ".multigravity", "mirrors", taskID)
	return os.RemoveAll(mirrorDir)
}

// cleanupTaskWorkspace tears down the workspace associated with a task (either worktree or mirror).
func cleanupTaskWorkspace(repoRoot string, task *Task, pruneRepo bool) error {
	if task == nil || task.WorktreeID == "" || repoRoot == "" {
		return nil
	}
	if task.Workspace == string(WorkspaceMirror) {
		return cleanupMirrorWorkspace(repoRoot, task.WorktreeID)
	}
	err := worktree.RemoveWorktree(repoRoot, task.WorktreeID, worktree.RemoveOptions{
		Force:        true,
		DeleteBranch: true,
	})
	if pruneRepo {
		_ = worktree.PruneWorktrees(repoRoot)
	}
	return err
}
