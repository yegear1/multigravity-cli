package workspace

import (
	"os"
	"os/exec"
	"strings"
)

var defaultGitRunner = func(dir string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

var gitRunnerFn = defaultGitRunner

// SetGitRunnerFn sets the git command runner and returns a restore closure (for testing)
func SetGitRunnerFn(fn func(dir string, args ...string) (string, error)) func() {
	prev := gitRunnerFn
	gitRunnerFn = fn
	return func() {
		gitRunnerFn = prev
	}
}

// DetectGitRepoInfo queries Git status and metadata for a given filesystem path
func DetectGitRepoInfo(dirPath string) *GitRepoInfo {
	if dirPath == "" {
		return nil
	}
	fi, err := os.Stat(dirPath)
	if err != nil || !fi.IsDir() {
		return nil
	}

	// Check if directory is a git repository
	insideWorkTree, err := gitRunnerFn(dirPath, "rev-parse", "--is-inside-work-tree")
	if err != nil || insideWorkTree != "true" {
		return &GitRepoInfo{
			IsGitRepo: false,
		}
	}

	info := &GitRepoInfo{
		IsGitRepo: true,
		IsClean:   true,
	}

	// Current branch
	if branch, err := gitRunnerFn(dirPath, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		info.Branch = branch
	}

	// Remote origin URL
	if remote, err := gitRunnerFn(dirPath, "config", "--get", "remote.origin.url"); err == nil {
		info.RemoteURL = remote
	}

	// Last commit hash & message
	if logOut, err := gitRunnerFn(dirPath, "log", "-1", "--format=%h|%s"); err == nil && logOut != "" {
		parts := strings.SplitN(logOut, "|", 2)
		info.CommitHash = parts[0]
		if len(parts) > 1 {
			info.CommitMessage = parts[1]
		}
	}

	// Status porcelain
	if statusOut, err := gitRunnerFn(dirPath, "status", "--porcelain"); err == nil && statusOut != "" {
		lines := strings.Split(statusOut, "\n")
		var modCount, untrackedCount int
		for _, line := range lines {
			if len(line) < 2 {
				continue
			}
			if strings.HasPrefix(line, "??") {
				untrackedCount++
			} else {
				modCount++
			}
		}
		info.ModifiedFiles = modCount
		info.UntrackedFiles = untrackedCount
		if modCount > 0 || untrackedCount > 0 {
			info.IsClean = false
		}
	}

	return info
}
