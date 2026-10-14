package dispatch

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const maxActiveTaskSnippetBytes = 1500

// GovernanceContext encapsulates discovered repository governance paths and directives.
type GovernanceContext struct {
	RootPath          string `json:"root_path,omitempty"`
	AgentsPath        string `json:"agents_path,omitempty"`
	TaskPath          string `json:"task_path,omitempty"`
	NotesPath         string `json:"notes_path,omitempty"`
	ActiveTaskSnippet string `json:"active_task_snippet,omitempty"`
}

// Found returns true if at least one governance artifact was detected.
func (g *GovernanceContext) Found() bool {
	return g != nil && (g.AgentsPath != "" || g.TaskPath != "" || g.NotesPath != "")
}

// DiscoverGovernance locates project governance files across the provided candidate search directories.
// Candidate search directories are evaluated in order (e.g. worktree directory, then base repo root).
func DiscoverGovernance(searchDirs ...string) *GovernanceContext {
	ctx := &GovernanceContext{}

	for _, dir := range searchDirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}

		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}

		if ctx.RootPath == "" {
			ctx.RootPath = dir
		}

		// 1. Discover AGENTS.md
		if ctx.AgentsPath == "" {
			candidate := filepath.Join(dir, "AGENTS.md")
			if isRegularFile(candidate) {
				ctx.AgentsPath = candidate
			}
		}

		// 2. Discover TASK.md (.agent/TASK.md preferred, TASK.md fallback)
		if ctx.TaskPath == "" {
			candidate := filepath.Join(dir, ".agent", "TASK.md")
			if isRegularFile(candidate) {
				ctx.TaskPath = candidate
			} else {
				fallback := filepath.Join(dir, "TASK.md")
				if isRegularFile(fallback) {
					ctx.TaskPath = fallback
				}
			}
		}

		// 3. Discover NOTES.md (.agent/NOTES.md preferred, NOTES.md fallback)
		if ctx.NotesPath == "" {
			candidate := filepath.Join(dir, ".agent", "NOTES.md")
			if isRegularFile(candidate) {
				ctx.NotesPath = candidate
			} else {
				fallback := filepath.Join(dir, "NOTES.md")
				if isRegularFile(fallback) {
					ctx.NotesPath = fallback
				}
			}
		}

		if ctx.AgentsPath != "" && ctx.TaskPath != "" && ctx.NotesPath != "" {
			break
		}
	}

	if !ctx.Found() {
		return nil
	}

	// Extract active task snippet if TASK.md exists
	if ctx.TaskPath != "" {
		ctx.ActiveTaskSnippet = extractActiveTaskSnippet(ctx.TaskPath)
	}

	return ctx
}

func isRegularFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// extractActiveTaskSnippet reads the ## Active Task section from TASK.md up to the next top-level heading.
func extractActiveTaskSnippet(taskPath string) string {
	f, err := os.Open(taskPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	var (
		scanner        = bufio.NewScanner(f)
		inActiveTask   = false
		lines          []string
		collectedBytes int
	)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## Active Task") {
			inActiveTask = true
			continue
		}

		if inActiveTask {
			// Stop at next markdown h2 section
			if strings.HasPrefix(trimmed, "## ") {
				break
			}

			lines = append(lines, line)
			collectedBytes += len(line) + 1
			if collectedBytes >= maxActiveTaskSnippetBytes {
				lines = append(lines, "...")
				break
			}
		}
	}

	snippet := strings.TrimSpace(strings.Join(lines, "\n"))
	return snippet
}

// SynthesizePrompt produces a structured prompt prepending governance directives to basePrompt.
// If no governance context is provided or found, or if basePrompt is empty, basePrompt is returned.
// Synthesis is strictly idempotent to avoid duplicate governance injection.
func SynthesizePrompt(basePrompt string, gov *GovernanceContext) string {
	trimmedPrompt := strings.TrimSpace(basePrompt)
	if trimmedPrompt == "" || gov == nil || !gov.Found() {
		return basePrompt
	}

	// Prevent duplicate injection
	if strings.Contains(trimmedPrompt, "[PROJECT GOVERNANCE CONTEXT]") ||
		strings.Contains(trimmedPrompt, "[PROJECT GOVERNANCE DIRECTIVES]") {
		return basePrompt
	}

	var sb strings.Builder
	sb.WriteString("[PROJECT GOVERNANCE CONTEXT]\n")
	sb.WriteString("Strictly adhere to repository governance directives and invariants:\n")
	sb.WriteString("- Rules & Invariants: AGENTS.md (Follow Chesterton's fence, backward compatibility, zero opportunistic refactoring)\n")
	sb.WriteString("- Active Decisions: .agent/NOTES.md (Consult active contracts and runtime decisions)\n")
	sb.WriteString("- Active Backlog & DoD: .agent/TASK.md (Satisfy all acceptance criteria and Definition of Done)\n")

	if gov.ActiveTaskSnippet != "" {
		sb.WriteString("\n[ACTIVE TASK CONTEXT]\n")
		sb.WriteString(gov.ActiveTaskSnippet)
		sb.WriteString("\n")
	}

	sb.WriteString("\n[TASK INSTRUCTION]\n")
	sb.WriteString(trimmedPrompt)

	return sb.String()
}

// InjectGovernanceEnv populates governance environment variables for autonomous agent execution.
func InjectGovernanceEnv(env map[string]string, gov *GovernanceContext) map[string]string {
	if gov == nil || !gov.Found() {
		return env
	}

	res := make(map[string]string)
	for k, v := range env {
		res[k] = v
	}

	if _, ok := res["MULTIGRAVITY_GOVERNANCE"]; !ok {
		res["MULTIGRAVITY_GOVERNANCE"] = "true"
	}
	if gov.AgentsPath != "" {
		if _, ok := res["MULTIGRAVITY_GOVERNANCE_AGENTS"]; !ok {
			res["MULTIGRAVITY_GOVERNANCE_AGENTS"] = gov.AgentsPath
		}
	}
	if gov.TaskPath != "" {
		if _, ok := res["MULTIGRAVITY_GOVERNANCE_TASK"]; !ok {
			res["MULTIGRAVITY_GOVERNANCE_TASK"] = gov.TaskPath
		}
	}
	if gov.NotesPath != "" {
		if _, ok := res["MULTIGRAVITY_GOVERNANCE_NOTES"]; !ok {
			res["MULTIGRAVITY_GOVERNANCE_NOTES"] = gov.NotesPath
		}
	}

	return res
}
