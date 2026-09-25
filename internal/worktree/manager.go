package worktree

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	validIDRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`)
	manifestMu   sync.Mutex
)

// ValidateWorktreeID verifies that the given ID conforms to naming requirements.
func ValidateWorktreeID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("worktree ID cannot be empty")
	}
	if strings.Contains(id, "..") {
		return fmt.Errorf("worktree ID cannot contain '..': %s", id)
	}
	if !validIDRegex.MatchString(id) {
		return fmt.Errorf("invalid worktree ID: %q (must start with alphanumeric and contain only letters, numbers, '.', '_', '-', or '/')", id)
	}
	return nil
}

func getManifestPath(repoRoot string) (string, error) {
	commonDir, err := GetGitCommonDir(repoRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(commonDir, "multigravity-worktrees.json"), nil
}

func loadManifest(repoRoot string) (*WorktreeManifest, error) {
	path, err := getManifestPath(repoRoot)
	if err != nil {
		return nil, err
	}

	manifest := &WorktreeManifest{
		Version:   1,
		Worktrees: make(map[string]Worktree),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return manifest, nil
		}
		return nil, fmt.Errorf("failed to read worktree manifest: %w", err)
	}

	if err := json.Unmarshal(data, manifest); err != nil {
		return nil, fmt.Errorf("failed to parse worktree manifest: %w", err)
	}
	if manifest.Worktrees == nil {
		manifest.Worktrees = make(map[string]Worktree)
	}
	return manifest, nil
}

func saveManifest(repoRoot string, manifest *WorktreeManifest) error {
	path, err := getManifestPath(repoRoot)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize manifest: %w", err)
	}

	return os.WriteFile(path, append(data, '\n'), 0644)
}

// CreateWorktree creates a new ephemeral worktree for an agent or task.
func CreateWorktree(opts CreateOptions) (*Worktree, error) {
	if err := ValidateWorktreeID(opts.ID); err != nil {
		return nil, err
	}

	repoDir := opts.RepoPath
	if repoDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to determine working directory: %w", err)
		}
		repoDir = cwd
	}

	repoRoot, err := FindRepoRoot(repoDir)
	if err != nil {
		return nil, err
	}

	manifestMu.Lock()
	defer manifestMu.Unlock()

	manifest, err := loadManifest(repoRoot)
	if err != nil {
		return nil, err
	}

	if _, exists := manifest.Worktrees[opts.ID]; exists {
		return nil, fmt.Errorf("worktree with ID %q already exists in repository", opts.ID)
	}

	branch := opts.Branch
	if branch == "" {
		branch = "multigravity/" + opts.ID
	}

	baseCommit := opts.BaseCommit
	if baseCommit == "" {
		head, err := GetHeadCommit(repoRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to determine base commit: %w", err)
		}
		baseCommit = head
	}

	targetDir := opts.TargetDir
	if targetDir == "" {
		targetDir = filepath.Join(repoRoot, ".multigravity", "worktrees", opts.ID)
	}
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, err
	}

	// Invariant: Exclude .multigravity/ from parent git tracking to keep working tree pristine
	if err := EnsureGitExclude(repoRoot, ".multigravity/"); err != nil {
		return nil, fmt.Errorf("failed to configure git exclude: %w", err)
	}

	// Create git worktree
	if err := GitWorktreeAdd(repoRoot, absTargetDir, branch, baseCommit); err != nil {
		return nil, fmt.Errorf("failed to add git worktree: %w", err)
	}

	headCommit, _ := GetHeadCommit(absTargetDir)

	// Copy requested files
	for _, file := range opts.CopyFiles {
		src := filepath.Join(repoRoot, file)
		dst := filepath.Join(absTargetDir, file)
		if err := copyFile(src, dst); err != nil {
			// Best effort, but keep going
			_ = err
		}
	}

	// Link requested paths
	for _, link := range opts.LinkPaths {
		src := filepath.Join(repoRoot, link)
		dst := filepath.Join(absTargetDir, link)
		if err := linkPath(src, dst); err != nil {
			_ = err
		}
	}

	wt := Worktree{
		ID:         opts.ID,
		RepoPath:   repoRoot,
		Path:       absTargetDir,
		Branch:     branch,
		BaseCommit: baseCommit,
		HeadCommit: headCommit,
		CreatedAt:  time.Now().UTC(),
		Profile:    opts.Profile,
		TaskID:     opts.TaskID,
		Status:     "active",
		Metadata:   opts.Metadata,
	}

	manifest.Worktrees[opts.ID] = wt
	if err := saveManifest(repoRoot, manifest); err != nil {
		return nil, err
	}

	return &wt, nil
}

// ListWorktrees returns all worktrees tracked in the repository, reconciled against git worktree state.
func ListWorktrees(repoPath string) ([]Worktree, error) {
	if repoPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		repoPath = cwd
	}

	repoRoot, err := FindRepoRoot(repoPath)
	if err != nil {
		return nil, err
	}

	manifestMu.Lock()
	defer manifestMu.Unlock()

	manifest, err := loadManifest(repoRoot)
	if err != nil {
		return nil, err
	}

	rawList, err := GitWorktreeList(repoRoot)
	if err != nil {
		return nil, err
	}

	rawMap := make(map[string]RawWorktreeInfo)
	for _, rw := range rawList {
		rawMap[filepath.Clean(rw.Worktree)] = rw
	}

	results := []Worktree{}
	seenPaths := make(map[string]bool)

	for id, wt := range manifest.Worktrees {
		cleanPath := filepath.Clean(wt.Path)
		seenPaths[cleanPath] = true

		// Check disk status
		if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
			wt.Status = "missing"
		} else {
			if rw, ok := rawMap[cleanPath]; ok {
				if rw.HEAD != "" {
					wt.HeadCommit = rw.HEAD
				}
				if rw.Branch != "" {
					wt.Branch = rw.Branch
				}
			}
			isClean, _, _, err := GitStatusPorcelain(cleanPath)
			if err == nil {
				if isClean {
					wt.Status = "active"
				} else {
					wt.Status = "dirty"
				}
			} else {
				wt.Status = "active"
			}
		}

		manifest.Worktrees[id] = wt
		results = append(results, wt)
	}

	// Also detect any worktrees registered in git that are not in manifest (excluding repoRoot itself)
	cleanRepoRoot := filepath.Clean(repoRoot)
	for _, rw := range rawList {
		cleanPath := filepath.Clean(rw.Worktree)
		if cleanPath == cleanRepoRoot || seenPaths[cleanPath] {
			continue
		}

		id := filepath.Base(cleanPath)
		status := "active"
		if isClean, _, _, err := GitStatusPorcelain(cleanPath); err == nil && !isClean {
			status = "dirty"
		}

		untrackedWT := Worktree{
			ID:         id,
			RepoPath:   repoRoot,
			Path:       cleanPath,
			Branch:     rw.Branch,
			BaseCommit: "",
			HeadCommit: rw.HEAD,
			CreatedAt:  time.Time{},
			Status:     status,
		}
		results = append(results, untrackedWT)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})

	return results, nil
}

// GetWorktree finds a specific worktree by ID.
func GetWorktree(repoPath, id string) (*Worktree, error) {
	list, err := ListWorktrees(repoPath)
	if err != nil {
		return nil, err
	}

	for _, wt := range list {
		if wt.ID == id {
			return &wt, nil
		}
	}
	return nil, fmt.Errorf("worktree %q not found", id)
}

// RemoveWorktree deletes a worktree and optionally deletes its associated branch.
func RemoveWorktree(repoPath, id string, opts RemoveOptions) error {
	wt, err := GetWorktree(repoPath, id)
	if err != nil {
		return err
	}

	repoRoot := wt.RepoPath

	// Check if dirty
	if !opts.Force && wt.Status != "missing" {
		isClean, modified, untracked, err := GitStatusPorcelain(wt.Path)
		if err == nil && !isClean {
			return fmt.Errorf("worktree %q has uncommitted changes (%d modified, %d untracked). Use force to remove",
				id, len(modified), len(untracked))
		}
	}

	// Remove via git
	_ = GitWorktreeRemove(repoRoot, wt.Path, opts.Force)

	// Clean up disk directory if it still exists
	if _, err := os.Stat(wt.Path); err == nil {
		_ = os.RemoveAll(wt.Path)
	}

	// Delete branch if requested
	if opts.DeleteBranch && wt.Branch != "" {
		_ = GitBranchDelete(repoRoot, wt.Branch, opts.Force)
	}

	manifestMu.Lock()
	defer manifestMu.Unlock()

	manifest, err := loadManifest(repoRoot)
	if err == nil {
		delete(manifest.Worktrees, id)
		_ = saveManifest(repoRoot, manifest)
	}

	_ = GitWorktreePrune(repoRoot)
	return nil
}

// PruneWorktrees removes stale worktree entries from git and cleans the manifest.
func PruneWorktrees(repoPath string) error {
	if repoPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		repoPath = cwd
	}

	repoRoot, err := FindRepoRoot(repoPath)
	if err != nil {
		return err
	}

	if err := GitWorktreePrune(repoRoot); err != nil {
		return err
	}

	manifestMu.Lock()
	defer manifestMu.Unlock()

	manifest, err := loadManifest(repoRoot)
	if err != nil {
		return err
	}

	modified := false
	for id, wt := range manifest.Worktrees {
		if _, err := os.Stat(wt.Path); os.IsNotExist(err) {
			delete(manifest.Worktrees, id)
			modified = true
		}
	}

	if modified {
		return saveManifest(repoRoot, manifest)
	}
	return nil
}

// GetWorktreeStatus gathers full git status telemetry for the worktree.
func GetWorktreeStatus(repoPath, id string) (*WorktreeStatus, error) {
	wt, err := GetWorktree(repoPath, id)
	if err != nil {
		return nil, err
	}

	isClean, modified, untracked, err := GitStatusPorcelain(wt.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get git status: %w", err)
	}

	ahead, behind, _ := GitCommitCount(wt.RepoPath, wt.BaseCommit, wt.HeadCommit)

	return &WorktreeStatus{
		Worktree:       *wt,
		IsClean:        isClean,
		ModifiedFiles:  modified,
		UntrackedFiles: untracked,
		CommitsAhead:   ahead,
		CommitsBehind:  behind,
	}, nil
}

// GetWorktreeDiff generates a diff for the given worktree.
func GetWorktreeDiff(repoPath, id string, opts DiffOptions) (string, error) {
	wt, err := GetWorktree(repoPath, id)
	if err != nil {
		return "", err
	}

	base := opts.Base
	if base == "" && wt.BaseCommit != "" {
		base = wt.BaseCommit
	}

	return GitDiff(wt.Path, base, opts.StatOnly, opts.Cached)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	return err
}

func linkPath(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.Symlink(src, dst)
}
