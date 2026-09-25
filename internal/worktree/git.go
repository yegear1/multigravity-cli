package worktree

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// RawWorktreeInfo contains the raw fields parsed from `git worktree list --porcelain`.
type RawWorktreeInfo struct {
	Worktree string
	HEAD     string
	Branch   string
	Bare     bool
	Detached bool
	Locked   bool
	Prunable string
}

// runGit executes a git command in the specified directory and returns combined stdout.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = strings.TrimSpace(stdout.String())
		}
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), errMsg)
	}
	return strings.TrimRight(stdout.String(), "\r\n"), nil
}

// FindRepoRoot determines the top-level directory of the Git repository for the given dir.
func FindRepoRoot(dir string) (string, error) {
	out, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository (or any parent up to mount point): %w", err)
	}
	return filepath.Clean(out), nil
}

// GetGitCommonDir returns the absolute path to the common .git directory.
func GetGitCommonDir(dir string) (string, error) {
	out, err := runGit(dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(out) {
		return filepath.Clean(out), nil
	}
	return filepath.Clean(filepath.Join(dir, out)), nil
}

// GetHeadCommit returns the commit hash at HEAD for the given directory.
func GetHeadCommit(dir string) (string, error) {
	return runGit(dir, "rev-parse", "HEAD")
}

// GetCurrentBranch returns the active branch name or "HEAD" if detached.
func GetCurrentBranch(dir string) (string, error) {
	branch, err := runGit(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return branch, nil
}

// BranchExists checks whether a local branch exists in the repository.
func BranchExists(repoRoot, branch string) bool {
	_, err := runGit(repoRoot, "rev-parse", "--verify", "refs/heads/"+branch)
	return err == nil
}

// EnsureGitExclude ensures the given pattern exists in .git/info/exclude.
func EnsureGitExclude(repoRoot, pattern string) error {
	commonDir, err := GetGitCommonDir(repoRoot)
	if err != nil {
		return err
	}
	excludePath := filepath.Join(commonDir, "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		return fmt.Errorf("failed to create info dir: %w", err)
	}

	content, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read exclude file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	for _, l := range lines {
		if strings.TrimSpace(l) == pattern {
			return nil // already present
		}
	}

	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open exclude file for writing: %w", err)
	}
	defer f.Close()

	prefix := ""
	if len(content) > 0 && !strings.HasSuffix(string(content), "\n") {
		prefix = "\n"
	}
	_, err = f.WriteString(prefix + pattern + "\n")
	return err
}

// GitWorktreeAdd adds a worktree at targetDir.
// If the branch already exists, it checks it out. Otherwise, it creates it starting at baseCommit.
func GitWorktreeAdd(repoRoot, targetDir, branch, baseCommit string) error {
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}

	if BranchExists(repoRoot, branch) {
		_, err = runGit(repoRoot, "worktree", "add", absTarget, branch)
	} else {
		args := []string{"worktree", "add", "-b", branch, absTarget}
		if baseCommit != "" {
			args = append(args, baseCommit)
		}
		_, err = runGit(repoRoot, args...)
	}
	return err
}

// GitWorktreeRemove removes the worktree at targetDir.
func GitWorktreeRemove(repoRoot, targetDir string, force bool) error {
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, absTarget)
	_, err = runGit(repoRoot, args...)
	return err
}

// GitWorktreePrune prunes stale worktree entries.
func GitWorktreePrune(repoRoot string) error {
	_, err := runGit(repoRoot, "worktree", "prune")
	return err
}

// GitWorktreeList parses the output of `git worktree list --porcelain`.
func GitWorktreeList(repoRoot string) ([]RawWorktreeInfo, error) {
	out, err := runGit(repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	var results []RawWorktreeInfo
	blocks := strings.Split(out, "\n\n")

	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		info := RawWorktreeInfo{}
		lines := strings.Split(block, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "worktree ") {
				info.Worktree = strings.TrimPrefix(line, "worktree ")
			} else if strings.HasPrefix(line, "HEAD ") {
				info.HEAD = strings.TrimPrefix(line, "HEAD ")
			} else if strings.HasPrefix(line, "branch ") {
				ref := strings.TrimPrefix(line, "branch ")
				info.Branch = strings.TrimPrefix(ref, "refs/heads/")
			} else if line == "bare" {
				info.Bare = true
			} else if line == "detached" {
				info.Detached = true
			} else if strings.HasPrefix(line, "locked") {
				info.Locked = true
			} else if strings.HasPrefix(line, "prunable ") {
				info.Prunable = strings.TrimPrefix(line, "prunable ")
			}
		}

		if info.Worktree != "" {
			results = append(results, info)
		}
	}

	return results, nil
}

// GitBranchDelete deletes the specified branch in the repository.
func GitBranchDelete(repoRoot, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := runGit(repoRoot, "branch", flag, branch)
	return err
}

// GitStatusPorcelain checks whether the worktree is clean and lists modified and untracked files.
func GitStatusPorcelain(worktreeDir string) (isClean bool, modified, untracked []string, err error) {
	out, err := runGit(worktreeDir, "status", "--porcelain")
	if err != nil {
		return false, nil, nil, err
	}

	if strings.TrimSpace(out) == "" {
		return true, nil, nil, nil
	}

	lines := strings.Split(out, "\n")
	for _, l := range lines {
		l = strings.TrimRight(l, "\r")
		if len(l) < 3 {
			continue
		}
		status := l[:2]
		file := strings.TrimSpace(l[2:])
		if status == "??" {
			untracked = append(untracked, file)
		} else {
			modified = append(modified, file)
		}
	}

	isClean = len(modified) == 0 && len(untracked) == 0
	return isClean, modified, untracked, nil
}

// GitDiff executes git diff in the worktree directory.
func GitDiff(worktreeDir, base string, statOnly, cached bool) (string, error) {
	args := []string{"diff"}
	if statOnly {
		args = append(args, "--stat")
	}
	if cached {
		args = append(args, "--cached")
	}
	if base != "" {
		args = append(args, base)
	}
	return runGit(worktreeDir, args...)
}

// GitCommitCount returns the number of commits ahead and behind between two refs.
func GitCommitCount(repoRoot, from, to string) (ahead, behind int, err error) {
	if from == "" || to == "" {
		return 0, 0, nil
	}
	out, err := runGit(repoRoot, "rev-list", "--left-right", "--count", from+"..."+to)
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(out)
	if len(fields) >= 2 {
		left, _ := strconv.Atoi(fields[0])
		right, _ := strconv.Atoi(fields[1])
		return right, left, nil // 'right' is commits ahead in 'to' relative to 'from'
	}
	return 0, 0, nil
}
