package agent

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SupportsNativeSandbox returns true if the runner has built-in sandboxing capabilities (e.g. Codex).
func SupportsNativeSandbox(agentType string) bool {
	return strings.ToLower(strings.TrimSpace(agentType)) == "codex"
}

// WrapWithAgentGuard wraps an agent command with the agent-guard sandbox runner:
// agent-guard wrap --role <role> --repo <repoPath> -- <command> [args...]
func WrapWithAgentGuard(command string, args []string, repoPath, role string) (string, []string) {
	if role == "" {
		role = "developer"
	}
	if repoPath == "" {
		repoPath = "."
	}

	// Avoid wrapping recursively if already wrapped
	if command == "agent-guard" || (len(args) > 0 && args[0] == "wrap") {
		return command, args
	}

	wrapperCmd := "agent-guard"
	wrapperArgs := []string{
		"wrap",
		"--role", role,
		"--repo", repoPath,
		"--",
		command,
	}
	wrapperArgs = append(wrapperArgs, args...)
	return wrapperCmd, wrapperArgs
}

// ValidatePathWithinRepo checks whether targetPath resides within the repoRoot boundary.
func ValidatePathWithinRepo(repoRoot, targetPath string) error {
	if repoRoot == "" || targetPath == "" {
		return nil
	}

	absRepo, err := filepath.Abs(filepath.Clean(repoRoot))
	if err != nil {
		return fmt.Errorf("invalid repository root: %w", err)
	}

	var absTarget string
	if filepath.IsAbs(targetPath) {
		absTarget = filepath.Clean(targetPath)
	} else {
		absTarget = filepath.Join(absRepo, targetPath)
		absTarget = filepath.Clean(absTarget)
	}

	rel, err := filepath.Rel(absRepo, absTarget)
	if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return fmt.Errorf("path %q escapes repository root %q", targetPath, repoRoot)
	}

	return nil
}

// mutatingCommands contains shell commands that perform filesystem mutations.
var mutatingCommands = map[string]bool{
	"rm":       true,
	"touch":    true,
	"cp":       true,
	"mv":       true,
	"mkdir":    true,
	"rmdir":    true,
	"chmod":    true,
	"chown":    true,
	"truncate": true,
	"tee":      true,
}

// InterceptSubshellMutation validates shell commands and intercepts mutation attempts
// targeting files or directories outside target repository root.
func InterceptSubshellMutation(repoRoot string, subshellCmd string) error {
	if repoRoot == "" || strings.TrimSpace(subshellCmd) == "" {
		return nil
	}

	cleanRoot, err := filepath.Abs(filepath.Clean(repoRoot))
	if err != nil {
		return fmt.Errorf("invalid repository root %q: %w", repoRoot, err)
	}

	// Split by command chain delimiters: ;, &&, ||, |, \n
	lines := strings.FieldsFunc(subshellCmd, func(r rune) bool {
		return r == ';' || r == '&' || r == '|' || r == '\n'
	})

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}

		cmdName := strings.ToLower(filepath.Base(fields[0]))
		cmdName = strings.TrimSuffix(cmdName, ".exe")

		// 1. Check redirection operators (>, >>) anywhere in the subshell pipeline
		for i, token := range fields {
			if token == ">" || token == ">>" || token == "1>" || token == "1>>" || token == "2>" || token == "2>>" {
				if i+1 < len(fields) {
					target := fields[i+1]
					target = strings.Trim(target, "\"'`")
					if err := ValidatePathWithinRepo(cleanRoot, target); err != nil {
						return fmt.Errorf("subshell execution violation: redirection mutation target %q is outside repository root %q", target, cleanRoot)
					}
				}
			} else if strings.HasPrefix(token, ">") || strings.HasPrefix(token, ">>") {
				target := strings.TrimLeft(token, ">")
				target = strings.Trim(target, "\"'`")
				if target != "" {
					if err := ValidatePathWithinRepo(cleanRoot, target); err != nil {
						return fmt.Errorf("subshell execution violation: redirection mutation target %q is outside repository root %q", target, cleanRoot)
					}
				}
			}
		}

		// 2. Check git commands attempting mutations outside repo (-C, --git-dir, --work-tree)
		if cmdName == "git" {
			for i := 1; i < len(fields); i++ {
				f := fields[i]
				if f == "-C" && i+1 < len(fields) {
					target := strings.Trim(fields[i+1], "\"'`")
					if err := ValidatePathWithinRepo(cleanRoot, target); err != nil {
						return fmt.Errorf("subshell execution violation: git working directory %q is outside repository root %q", target, cleanRoot)
					}
				} else if strings.HasPrefix(f, "--git-dir=") {
					target := strings.Trim(strings.TrimPrefix(f, "--git-dir="), "\"'`")
					if err := ValidatePathWithinRepo(cleanRoot, target); err != nil {
						return fmt.Errorf("subshell execution violation: git directory %q is outside repository root %q", target, cleanRoot)
					}
				} else if strings.HasPrefix(f, "--work-tree=") {
					target := strings.Trim(strings.TrimPrefix(f, "--work-tree="), "\"'`")
					if err := ValidatePathWithinRepo(cleanRoot, target); err != nil {
						return fmt.Errorf("subshell execution violation: git work tree %q is outside repository root %q", target, cleanRoot)
					}
				}
			}
		}

		// 3. Check sed with in-place modification (-i)
		if cmdName == "sed" {
			hasInPlace := false
			for _, f := range fields[1:] {
				if f == "-i" || strings.HasPrefix(f, "-i") {
					hasInPlace = true
					break
				}
			}
			if hasInPlace {
				// The last non-flag argument is typically the file to mutate
				for i := len(fields) - 1; i >= 1; i-- {
					arg := fields[i]
					if !strings.HasPrefix(arg, "-") {
						target := strings.Trim(arg, "\"'`")
						if err := ValidatePathWithinRepo(cleanRoot, target); err != nil {
							return fmt.Errorf("subshell execution violation: sed in-place target %q is outside repository root %q", target, cleanRoot)
						}
						break
					}
				}
			}
		}

		// 4. Check known mutating commands (rm, touch, cp, mv, mkdir, rmdir, chmod, chown, truncate, tee)
		if mutatingCommands[cmdName] {
			for _, arg := range fields[1:] {
				if strings.HasPrefix(arg, "-") {
					continue
				}
				target := strings.Trim(arg, "\"'`")
				if target == "" {
					continue
				}
				if err := ValidatePathWithinRepo(cleanRoot, target); err != nil {
					return fmt.Errorf("subshell execution violation: command %q mutation target %q is outside repository root %q", cmdName, target, cleanRoot)
				}
			}
		}
	}

	return nil
}
