package agent

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Execution roles / personas.
const (
	RoleDeveloper  = "developer"
	RoleResearcher = "researcher"
	RoleVerifier   = "verifier"
	RoleReviewer   = "reviewer"
	RoleAuditor    = "auditor"
)

// IsReadOnlyRole reports whether an execution role is inherently read-only.
func IsReadOnlyRole(role string) bool {
	r := strings.ToLower(strings.TrimSpace(role))
	return r == RoleResearcher || r == RoleReviewer || r == RoleAuditor
}

// SupportsNativeSandbox returns true if the runner has built-in sandboxing capabilities (e.g. Codex).
func SupportsNativeSandbox(agentType string) bool {
	return strings.ToLower(strings.TrimSpace(agentType)) == "codex"
}

// GuardOptions specifies configuration parameters for agent-guard process wrapping.
type GuardOptions struct {
	RepoPath string
	Role     string
	ReadOnly bool
}

// WrapWithAgentGuard wraps an agent command with the agent-guard sandbox runner:
// agent-guard wrap --role <role> --repo <repoPath> -- <command> [args...]
func WrapWithAgentGuard(command string, args []string, repoPath, role string) (string, []string) {
	return WrapWithAgentGuardWithOptions(command, args, GuardOptions{
		RepoPath: repoPath,
		Role:     role,
		ReadOnly: false,
	})
}

// WrapWithAgentGuardWithOptions wraps an agent command with the agent-guard sandbox runner:
// agent-guard wrap --role <role> --repo <repoPath> [--read-only] -- <command> [args...]
func WrapWithAgentGuardWithOptions(command string, args []string, opts GuardOptions) (string, []string) {
	if opts.Role == "" {
		opts.Role = RoleDeveloper
	}
	if opts.RepoPath == "" {
		opts.RepoPath = "."
	}

	// Avoid wrapping recursively if already wrapped
	if command == "agent-guard" || (len(args) > 0 && args[0] == "wrap") {
		return command, args
	}

	wrapperCmd := "agent-guard"
	wrapperArgs := []string{
		"wrap",
		"--role", opts.Role,
		"--repo", opts.RepoPath,
	}
	if opts.ReadOnly || IsReadOnlyRole(opts.Role) {
		wrapperArgs = append(wrapperArgs, "--read-only")
	}
	wrapperArgs = append(wrapperArgs, "--", command)
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

// mutatingGitSubcommands contains git subcommands that modify repository state or history.
var mutatingGitSubcommands = map[string]bool{
	"commit":      true,
	"push":        true,
	"reset":       true,
	"clean":       true,
	"restore":     true,
	"merge":       true,
	"rebase":      true,
	"apply":       true,
	"cherry-pick": true,
	"tag":         true,
	"rm":          true,
	"mv":          true,
	"revert":      true,
	"checkout":    true,
}

// isSourceFile returns true if the target path corresponds to a source code or config file.
func isSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".c", ".cpp", ".cc", ".h", ".hpp",
		".java", ".kt", ".cs", ".rb", ".php", ".swift", ".md", ".json", ".yaml", ".yml", ".toml",
		".html", ".css", ".scss", ".sass", ".vue", ".svelte", ".sql", ".sh", ".bash", ".zsh":
		return true
	default:
		base := strings.ToLower(filepath.Base(path))
		return base == "dockerfile" || base == "makefile" || base == "cmakelists.txt"
	}
}

// InterceptSubshellMutation validates shell commands and intercepts mutation attempts
// targeting files or directories outside target repository root using default developer role.
func InterceptSubshellMutation(repoRoot string, subshellCmd string) error {
	return InterceptSubshellMutationWithRole(repoRoot, subshellCmd, RoleDeveloper, false)
}

// InterceptSubshellMutationWithRole validates shell commands and intercepts mutation attempts
// based on role capabilities (developer, researcher, verifier) and read-only constraints.
func InterceptSubshellMutationWithRole(repoRoot string, subshellCmd string, role string, readOnly bool) error {
	if repoRoot == "" || strings.TrimSpace(subshellCmd) == "" {
		return nil
	}

	cleanRoot, err := filepath.Abs(filepath.Clean(repoRoot))
	if err != nil {
		return fmt.Errorf("invalid repository root %q: %w", repoRoot, err)
	}

	cleanRole := strings.ToLower(strings.TrimSpace(role))
	if cleanRole == "" {
		cleanRole = RoleDeveloper
	}
	if IsReadOnlyRole(cleanRole) {
		readOnly = true
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
			var target string
			if token == ">" || token == ">>" || token == "1>" || token == "1>>" || token == "2>" || token == "2>>" {
				if i+1 < len(fields) {
					target = fields[i+1]
				}
			} else if strings.HasPrefix(token, ">") || strings.HasPrefix(token, ">>") {
				target = strings.TrimLeft(token, ">")
			} else if strings.HasPrefix(token, "1>") || strings.HasPrefix(token, "1>>") {
				target = strings.TrimLeft(strings.TrimPrefix(token, "1"), ">")
			} else if strings.HasPrefix(token, "2>") || strings.HasPrefix(token, "2>>") {
				target = strings.TrimLeft(strings.TrimPrefix(token, "2"), ">")
			}

			if target != "" {
				cleanTarget := strings.Trim(target, "\"'`")
				// Safe null sinks and descriptor duplicates
				if cleanTarget == "/dev/null" || strings.ToLower(cleanTarget) == "nul" || strings.HasPrefix(cleanTarget, "&") {
					continue
				}

				if readOnly {
					return fmt.Errorf("subshell execution violation: redirection mutation target %q blocked in read-only mode (role: %s)", cleanTarget, cleanRole)
				}

				if cleanRole == RoleVerifier && isSourceFile(cleanTarget) {
					return fmt.Errorf("subshell execution violation: verifier persona cannot redirect output into source file %q", cleanTarget)
				}

				if err := ValidatePathWithinRepo(cleanRoot, cleanTarget); err != nil {
					return fmt.Errorf("subshell execution violation: redirection mutation target %q is outside repository root %q", cleanTarget, cleanRoot)
				}
			}
		}

		// 2. Check git commands attempting mutations outside repo (-C, --git-dir, --work-tree) or restricted git actions
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

			if readOnly || cleanRole == RoleVerifier {
				gitSubCmd := ""
				for i := 1; i < len(fields); i++ {
					arg := fields[i]
					if strings.HasPrefix(arg, "-") {
						if arg == "-C" && i+1 < len(fields) {
							i++
						}
						continue
					}
					gitSubCmd = strings.ToLower(arg)
					break
				}

				if gitSubCmd != "" {
					modeDesc := "read-only mode"
					if !readOnly && cleanRole == RoleVerifier {
						modeDesc = "verifier persona"
					}

					if mutatingGitSubcommands[gitSubCmd] {
						return fmt.Errorf("subshell execution violation: git %s is prohibited in %s (role: %s)", gitSubCmd, modeDesc, cleanRole)
					}
					if gitSubCmd == "branch" {
						for _, a := range fields[1:] {
							if a == "-d" || a == "-D" || a == "-m" || a == "-M" || (!strings.HasPrefix(a, "-") && a != "branch") {
								return fmt.Errorf("subshell execution violation: git branch mutation is prohibited in %s (role: %s)", modeDesc, cleanRole)
							}
						}
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
				if readOnly {
					return fmt.Errorf("subshell execution violation: sed in-place modification is prohibited in read-only mode (role: %s)", cleanRole)
				}
				if cleanRole == RoleVerifier {
					return fmt.Errorf("subshell execution violation: sed in-place modification is prohibited for verifier persona")
				}
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
			if readOnly {
				return fmt.Errorf("subshell execution violation: mutating command %q is prohibited in read-only mode (role: %s)", cmdName, cleanRole)
			}
			if cleanRole == RoleVerifier {
				return fmt.Errorf("subshell execution violation: mutating command %q is prohibited for verifier persona", cmdName)
			}
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
