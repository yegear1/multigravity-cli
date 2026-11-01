package dispatch

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
	"github.com/ye-dev/multigravity-cli/internal/worktree"
)

// SubtaskSpec defines parameters for a single subtask within an execution plan.
type SubtaskSpec struct {
	ID            string            `json:"id,omitempty"`
	Profile       string            `json:"profile"`
	Prompt        string            `json:"prompt,omitempty"`
	AgentType     string            `json:"agent_type,omitempty"`
	Command       string            `json:"command,omitempty"`
	Args          []string          `json:"args,omitempty"`
	Branch        string            `json:"branch,omitempty"`
	BaseCommit    string            `json:"base_commit,omitempty"`
	Cwd           string            `json:"cwd,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	GatewayURL    string            `json:"gateway_url,omitempty"`
	GatewayKey    string            `json:"gateway_key,omitempty"`
	Verify        string            `json:"verify,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Read          []string          `json:"read,omitempty"`
	Paths         []string          `json:"paths,omitempty"`
	CommitMessage string            `json:"commit_message,omitempty"`
	Model         string            `json:"model,omitempty"`
}

// PlanRequest defines the input payload for dispatching a batch of subtasks concurrently across worktrees.
type PlanRequest struct {
	PlanID       string        `json:"plan_id,omitempty"`
	RepoPath     string        `json:"repo_path,omitempty"`
	BaseBranch   string        `json:"base_branch,omitempty"`
	BaseCommit   string        `json:"base_commit,omitempty"`
	Timeout      string        `json:"timeout,omitempty"`
	Workers      int           `json:"workers,omitempty"`
	Subtasks     []SubtaskSpec `json:"subtasks,omitempty"`
	Tasks        []SubtaskSpec `json:"tasks,omitempty"` // Alias for Subtasks
	DefaultModel string        `json:"default_model,omitempty"`
	ApplyAtomic  bool          `json:"apply_atomic,omitempty"`
	OmitDiff     bool          `json:"omit_diff,omitempty"`
	InlineRead   bool          `json:"inline_read,omitempty"`
}

// SubtaskResult encapsulates execution details and git diff produced by a single subtask.
type SubtaskResult struct {
	ID                   string          `json:"id"`
	TaskID               string          `json:"task_id"`
	Profile              string          `json:"profile"`
	Prompt               string          `json:"prompt,omitempty"`
	AgentType            string          `json:"agent_type,omitempty"`
	Status               TaskStatus      `json:"status"`
	WorktreeID           string          `json:"worktree_id,omitempty"`
	WorktreePath         string          `json:"worktree_path,omitempty"`
	Branch               string          `json:"branch,omitempty"`
	BaseCommit           string          `json:"base_commit,omitempty"`
	HeadCommit           string          `json:"head_commit,omitempty"`
	ExitCode             int             `json:"exit_code"`
	VerificationExitCode int             `json:"verification_exit_code,omitempty"`
	VerificationOutput   string          `json:"verification_output,omitempty"`
	LogTail              string          `json:"log_tail,omitempty"`
	DurationSeconds      float64         `json:"duration_seconds"`
	Error                string          `json:"error,omitempty"`
	Diff                 string          `json:"diff,omitempty"`
	StructuredDiff       *StructuredDiff `json:"structured_diff,omitempty"`
	FilesChanged         []string        `json:"files_changed,omitempty"`
	Commit               string          `json:"commit,omitempty"`
	RecoveredFromTimeout bool            `json:"recovered_from_timeout,omitempty"`
	Model                string          `json:"model,omitempty"`
	FailureReason        string          `json:"failure_reason,omitempty"`
	TokensUsed           int             `json:"tokens_used,omitempty"`
}

// UnifiedDiffSummary aggregates diff statistics, affected files, and scope collision analysis.
type UnifiedDiffSummary struct {
	TotalFilesChanged   int      `json:"total_files_changed"`
	TotalAdditions      int      `json:"total_additions"`
	TotalDeletions      int      `json:"total_deletions"`
	ModifiedFiles       []string `json:"modified_files"`
	DisjointScopesClean bool     `json:"disjoint_scopes_clean"`
	ConflictingFiles    []string `json:"conflicting_files,omitempty"`
	CombinedDiff        string   `json:"combined_diff,omitempty"`
}

// PlanResult models the aggregated outcome of a multi-agent plan execution.
type PlanResult struct {
	PlanID          string             `json:"plan_id"`
	RepoPath        string             `json:"repo_path"`
	Status          string             `json:"status"` // "completed", "failed", "partial", "cancelled", "timeout"
	TotalSubtasks   int                `json:"total_subtasks"`
	Succeeded       int                `json:"succeeded"`
	Failed          int                `json:"failed"`
	DurationSeconds float64            `json:"duration_seconds"`
	CreatedAt       time.Time          `json:"created_at"`
	EndedAt         time.Time          `json:"ended_at"`
	Subtasks        []SubtaskResult    `json:"subtasks"`
	UnifiedSummary  UnifiedDiffSummary `json:"unified_summary"`
}

// ToCompact returns a copy of PlanResult where each SubtaskResult has Prompt: "" and Diff: "",
// and UnifiedSummary.CombinedDiff is empty, omitting prompt dumps and raw diffs.
func (p *PlanResult) ToCompact() *PlanResult {
	if p == nil {
		return nil
	}
	compact := *p
	if len(p.Subtasks) > 0 {
		compact.Subtasks = make([]SubtaskResult, len(p.Subtasks))
		for i, st := range p.Subtasks {
			stCopy := st
			stCopy.Prompt = ""
			stCopy.Diff = ""
			compact.Subtasks[i] = stCopy
		}
	}
	compact.UnifiedSummary.CombinedDiff = ""
	return &compact
}

// Compact is an alias for ToCompact.
func (p *PlanResult) Compact() *PlanResult {
	return p.ToCompact()
}

func hasLineRangeSlice(entries []string) bool {
	return quota.HasLineRangedRead(entries)
}

// SynthesizeSubtaskPrompt formats a subtask prompt and resolves the model to use.
func SynthesizeSubtaskPrompt(st SubtaskSpec, defaultModel string) (string, string) {
	return SynthesizeSubtaskPromptWithRepo(st, defaultModel, "", false)
}

// SynthesizeSubtaskPromptWithRepo formats a subtask prompt, resolving the model and optionally embedding file contents.
func SynthesizeSubtaskPromptWithRepo(st SubtaskSpec, defaultModel, repoRoot string, inlineRead bool) (string, string) {
	resolvedModel := st.Model
	if resolvedModel == "" {
		if defaultModel != "" {
			resolvedModel = defaultModel
		} else if len(st.Paths) == 1 && len(st.Read) > 0 && hasLineRangeSlice(st.Read) {
			resolvedModel = "gemini-3.8-flash-low"
		} else {
			resolvedModel = quota.ClassifyTaskComplexity(quota.TaskComplexityInput{
				Prompt: st.Prompt,
				Paths:  st.Paths,
				Read:   st.Read,
			}).RecommendedModel
		}
	}

	var sb strings.Builder
	if strings.TrimSpace(st.Prompt) != "" {
		sb.WriteString(st.Prompt)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Guardrails:\n")
	sb.WriteString("- Read ONLY the specified files first. If a referenced type or constant is missing, you may run at most ONE targeted `grep` in that package instead of guessing. Do NOT run baseline tests before editing. Exit immediately once validation passes without writing a prose summary.\n")

	if len(st.Read) > 0 {
		sb.WriteString("\nFiles to read:\n")
		for _, r := range st.Read {
			sb.WriteString(fmt.Sprintf("- %s\n", r))
		}
	}

	if len(st.Paths) > 0 {
		sb.WriteString("\nAllowed mutable paths:\n")
		for _, p := range st.Paths {
			sb.WriteString(fmt.Sprintf("- %s\n", p))
		}
	}

	if st.Verify != "" {
		sb.WriteString("\nVerify: " + st.Verify + "\n")
	}

	if inlineRead && repoRoot != "" {
		var embeddedSection strings.Builder
		totalBytes := 0
		const (
			maxBudget         = 15360
			maxNonRangedLines = 150
			maxNonRangedBytes = 5120
		)

		for _, entry := range st.Read {
			if totalBytes >= maxBudget {
				break
			}
			filePath := entry
			startLine := 0
			endLine := 0

			if idx := strings.LastIndex(entry, ":"); idx != -1 {
				linePart := entry[idx+1:]
				parts := strings.Split(linePart, "-")
				if len(parts) == 2 {
					s, err1 := strconv.Atoi(parts[0])
					e, err2 := strconv.Atoi(parts[1])
					if err1 == nil && err2 == nil && s > 0 && e >= s {
						filePath = entry[:idx]
						startLine = s
						endLine = e
					}
				}
			}

			fullPath := filepath.Join(repoRoot, filePath)
			f, err := os.Open(fullPath)
			if err != nil {
				continue
			}

			scanner := bufio.NewScanner(f)
			currentLine := 0
			var lines []string
			for scanner.Scan() {
				currentLine++
				if startLine > 0 && currentLine < startLine {
					continue
				}
				if endLine > 0 && currentLine > endLine {
					break
				}
				if startLine == 0 && endLine == 0 && currentLine > maxNonRangedLines {
					break
				}
				lines = append(lines, scanner.Text())
			}
			f.Close()

			if len(lines) == 0 {
				continue
			}

			content := strings.Join(lines, "\n")
			if startLine == 0 && endLine == 0 && len(content) > maxNonRangedBytes {
				content = content[:maxNonRangedBytes]
			}
			snippet := fmt.Sprintf("\n--- %s ---\n%s\n", entry, content)
			if totalBytes+len(snippet) > maxBudget {
				remaining := maxBudget - totalBytes
				if remaining > 0 {
					embeddedSection.WriteString(snippet[:remaining])
					totalBytes += remaining
				}
				break
			} else {
				embeddedSection.WriteString(snippet)
				totalBytes += len(snippet)
			}
		}

		if totalBytes < maxBudget {
			readFiles := make(map[string]bool)
			for _, entry := range st.Read {
				filePath := entry
				if idx := strings.LastIndex(entry, ":"); idx != -1 {
					filePath = entry[:idx]
				}
				readFiles[filepath.Clean(filePath)] = true
				readFiles[filePath] = true
				readFiles[filepath.ToSlash(filepath.Clean(filePath))] = true
			}

			for _, p := range st.Paths {
				if totalBytes >= maxBudget {
					break
				}
				cleanP := filepath.Clean(p)
				if !strings.HasSuffix(cleanP, ".go") || strings.HasSuffix(cleanP, "_test.go") {
					continue
				}
				testPath := strings.TrimSuffix(p, ".go") + "_test.go"
				cleanTestPath := filepath.Clean(testPath)
				slashTestPath := filepath.ToSlash(cleanTestPath)
				if readFiles[cleanTestPath] || readFiles[testPath] || readFiles[slashTestPath] {
					continue
				}
				readFiles[cleanTestPath] = true
				readFiles[testPath] = true
				readFiles[slashTestPath] = true

				fullPath := filepath.Join(repoRoot, testPath)
				fi, err := os.Stat(fullPath)
				if err != nil || fi.IsDir() {
					continue
				}

				f, err := os.Open(fullPath)
				if err != nil {
					continue
				}

				scanner := bufio.NewScanner(f)
				var lines []string
				for scanner.Scan() {
					lines = append(lines, scanner.Text())
				}
				f.Close()

				if len(lines) == 0 {
					continue
				}
				if len(lines) > 60 {
					lines = lines[len(lines)-60:]
				}

				content := strings.Join(lines, "\n")
				snippet := fmt.Sprintf("\n--- %s ---\n%s\n", testPath, content)
				if totalBytes+len(snippet) > maxBudget {
					remaining := maxBudget - totalBytes
					if remaining > 0 {
						embeddedSection.WriteString(snippet[:remaining])
						totalBytes += remaining
					}
					break
				} else {
					embeddedSection.WriteString(snippet)
					totalBytes += len(snippet)
				}
			}
		}

		if embeddedSection.Len() > 0 {
			sb.WriteString("\n[PRE-LOADED READ CONTEXT]")
			sb.WriteString(embeddedSection.String())
		}
	}

	return sb.String(), resolvedModel
}

// ValidatePlan validates a PlanRequest before execution, allowing empty or "auto" profile.
func ValidatePlan(req *PlanRequest) error {
	subtasks := req.Subtasks
	if len(subtasks) == 0 && len(req.Tasks) > 0 {
		subtasks = req.Tasks
	}
	if len(subtasks) == 0 {
		return errors.New("subtasks are required: at least one subtask must be specified")
	}

	for i, st := range subtasks {
		prof := strings.TrimSpace(st.Profile)
		if prof != "" && prof != "auto" {
			if err := config.ValidateProfileName(prof); err != nil {
				return fmt.Errorf("subtask [%d]: invalid profile name %q: %w", i, prof, err)
			}
			if !profile.ProfileExists(prof) {
				return fmt.Errorf("subtask [%d]: profile %q does not exist", i, prof)
			}
		}
	}
	return nil
}

// GeneratePlanID produces a unique identifier for a subtask aggregation plan.
func GeneratePlanID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("plan-%d-%s", time.Now().Unix(), hex.EncodeToString(b))
}

// tailString trims and returns the last maxBytes bytes of s.
func tailString(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	return s[len(s)-maxBytes:]
}

// SynthesizeCascadeRetryPrompt constructs an enriched prompt for retry attempts incorporating failure diagnostics.
func SynthesizeCascadeRetryPrompt(basePrompt, prevModel, verifyOutput, logTail string) string {
	diag := strings.TrimSpace(verifyOutput)
	if diag == "" {
		diag = strings.TrimSpace(logTail)
	}
	if diag == "" {
		return basePrompt
	}
	modelName := strings.TrimSpace(prevModel)
	if modelName == "" {
		modelName = "unknown"
	}
	return fmt.Sprintf("%s\n\n[PREVIOUS ATTEMPT FAILED ON MODEL %s]\nDiagnostic Output:\n%s\nFix the root cause above without regressing allowed paths.", basePrompt, modelName, diag)
}

// ClassifySubtaskFailure inspects a SubtaskResult and returns a structured failure reason
// ('timeout' | 'rate_limited' | 'empty_diff' | 'verify_failed' | 'agent_error'), or "" if succeeded.
func ClassifySubtaskFailure(res SubtaskResult) string {
	errLower := strings.ToLower(res.Error)
	logLower := strings.ToLower(res.LogTail)
	verifyLower := strings.ToLower(res.VerificationOutput)

	if res.Status == StatusCompleted && res.ExitCode == 0 && res.VerificationExitCode == 0 && strings.TrimSpace(res.Error) == "" {
		return ""
	}
	if res.Status == "" && res.ExitCode == 0 && res.VerificationExitCode == 0 && strings.TrimSpace(res.Error) == "" && strings.TrimSpace(res.VerificationOutput) == "" && strings.TrimSpace(res.LogTail) == "" {
		return ""
	}

	combined := errLower + "\n" + logLower + "\n" + verifyLower
	for _, indicator := range []string{
		"429",
		"rate limit",
		"rate-limit",
		"rate_limit",
		"ratelimit",
		"too many requests",
		"quota",
		"resource_exhausted",
		"resource exhausted",
	} {
		if strings.Contains(combined, indicator) {
			return "rate_limited"
		}
	}

	for _, indicator := range []string{
		"context deadline exceeded",
		"deadline exceeded",
		"timed out",
		"timeout",
	} {
		if strings.Contains(errLower, indicator) || strings.Contains(logLower, indicator) {
			return "timeout"
		}
	}
	if res.Status == StatusCancelled {
		return "timeout"
	}

	if strings.Contains(errLower, "empty_diff") || strings.Contains(errLower, "empty diff") || strings.Contains(errLower, "without modifying") {
		return "empty_diff"
	}
	if res.Status == StatusAuditFailed && res.VerificationExitCode == 0 && strings.TrimSpace(res.VerificationOutput) == "" && strings.TrimSpace(res.Diff) == "" {
		return "empty_diff"
	}

	if res.VerificationExitCode != 0 || strings.TrimSpace(res.VerificationOutput) != "" || res.Status == StatusAuditFailed || strings.Contains(errLower, "verify") || strings.Contains(errLower, "verification") {
		return "verify_failed"
	}

	return "agent_error"
}

func getPlansDir(repoRoot string) string {
	if repoRoot != "" {
		return filepath.Join(repoRoot, ".multigravity", "plans")
	}
	return filepath.Join(config.GetMultigravityHome(), ".plans")
}

func resolveEligibleProfilesForModel(model string) []string {
	threshold := quota.SafetyThresholdForModel(model, quota.WeightMedium)
	candidates, err := quota.PreflightProfilesForModel(0, model, quota.WeightMedium, threshold)
	if (err != nil || len(candidates) == 0) && threshold > quota.DefaultSafetyQuotaThreshold {
		candidates, err = quota.PreflightProfilesForModel(0, model, quota.WeightMedium, quota.DefaultSafetyQuotaThreshold)
	}
	var eligible []string
	if err == nil && len(candidates) > 0 {
		for _, c := range candidates {
			if profile.ProfileExists(c.Profile) && !quota.IsProfileOnCooldown(c.Profile) {
				eligible = append(eligible, c.Profile)
			}
		}
	}
	if len(eligible) > 0 {
		return eligible
	}
	if profs, err := profile.ListProfiles(); err == nil {
		for _, p := range profs {
			if !quota.IsProfileOnCooldown(p) {
				eligible = append(eligible, p)
			}
		}
	}
	return eligible
}

func countActiveEligibleProfiles(subtasks []SubtaskSpec, defaultModel string) int {
	hasAuto := false
	explicitSet := make(map[string]struct{})
	model := strings.TrimSpace(defaultModel)

	for _, st := range subtasks {
		if model == "" && strings.TrimSpace(st.Model) != "" {
			model = strings.TrimSpace(st.Model)
		}
		prof := strings.TrimSpace(st.Profile)
		if prof == "" || prof == "auto" {
			hasAuto = true
		} else if !quota.IsProfileOnCooldown(prof) {
			explicitSet[prof] = struct{}{}
		}
	}

	if !hasAuto {
		return len(explicitSet)
	}

	for _, p := range resolveEligibleProfilesForModel(model) {
		explicitSet[p] = struct{}{}
	}
	return len(explicitSet)
}

// ExecutePlan dispatches multiple subtasks concurrently across isolated worktrees and returns a unified summary.
func (m *TaskManager) ExecutePlan(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	subtasks := req.Subtasks
	if len(subtasks) == 0 && len(req.Tasks) > 0 {
		subtasks = req.Tasks
	}
	if len(subtasks) == 0 {
		return nil, errors.New("subtasks are required: at least one subtask must be specified")
	}

	// Resolve repo root (must be in a git repo for ephemeral worktrees)
	repoDir := req.RepoPath
	if repoDir == "" {
		cwd, err := os.Getwd()
		if err == nil {
			repoDir = cwd
		}
	}

	var repoRoot string
	if repoDir != "" {
		root, err := worktree.FindRepoRoot(repoDir)
		if err == nil {
			repoRoot = root
		}
	}
	if repoRoot == "" {
		return nil, errors.New("cannot execute plan: directory is not inside a git repository")
	}

	// Validate plan request
	if err := ValidatePlan(&req); err != nil {
		return nil, err
	}

	planID := strings.TrimSpace(req.PlanID)
	if planID == "" {
		planID = GeneratePlanID()
	}

	// Determine timeout
	timeout := 5 * time.Minute
	if req.Timeout != "" {
		d, err := time.ParseDuration(req.Timeout)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("invalid timeout %q: %w", req.Timeout, err)
		}
		timeout = d
	}

	planCtx, planCancel := context.WithTimeout(ctx, timeout)
	defer planCancel()

	// Determine concurrency workers
	workers := req.Workers
	if workers <= 0 || workers > len(subtasks) {
		workers = len(subtasks)
	}
	activeEligibleProfiles := countActiveEligibleProfiles(subtasks, req.DefaultModel)
	if activeEligibleProfiles <= 0 {
		activeEligibleProfiles = 1
	}
	if workers > activeEligibleProfiles {
		workers = activeEligibleProfiles
	}

	startTime := time.Now().UTC()
	subtaskResults := make([]SubtaskResult, len(subtasks))
	jobs := make(chan int)
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				st := subtasks[idx]
				stID := strings.TrimSpace(st.ID)
				if stID == "" {
					stID = fmt.Sprintf("subtask-%d", idx+1)
				}

				taskID := fmt.Sprintf("%s-%s", planID, stID)
				branch := st.Branch
				if branch == "" {
					branch = fmt.Sprintf("multigravity/%s/%s", planID, stID)
				}
				baseCommit := st.BaseCommit
				if baseCommit == "" {
					if req.BaseCommit != "" {
						baseCommit = req.BaseCommit
					} else {
						baseCommit = req.BaseBranch
					}
				}

				// Resolve prompt and model
				resolvedPrompt, resolvedModel := SynthesizeSubtaskPromptWithRepo(st, req.DefaultModel, repoRoot, req.InlineRead)

				// Resolve profile if empty or "auto"
				effectiveProfile := strings.TrimSpace(st.Profile)
				if effectiveProfile == "" || effectiveProfile == "auto" {
					if eligible := resolveEligibleProfilesForModel(resolvedModel); len(eligible) > 0 {
						effectiveProfile = eligible[idx%len(eligible)]
					} else if profs, err := profile.ListProfiles(); err == nil && len(profs) > 0 {
						effectiveProfile = profs[idx%len(profs)]
					}
				}

				res := SubtaskResult{
					ID:         stID,
					TaskID:     taskID,
					Profile:    effectiveProfile,
					Prompt:     resolvedPrompt,
					AgentType:  st.AgentType,
					Branch:     branch,
					BaseCommit: baseCommit,
					ExitCode:   -1,
					Model:      resolvedModel,
				}

				dArgs := append([]string(nil), st.Args...)
				cmd := strings.TrimSpace(st.Command)
				effectiveAgentType := strings.ToLower(strings.TrimSpace(st.AgentType))
				if cmd == "" && effectiveAgentType == "" {
					effectiveAgentType = DiscoverInstalledAgent()
				}
				isAgy := cmd == "agy" || strings.HasSuffix(cmd, "/agy") || effectiveAgentType == "agy"
				if (cmd == "" || isAgy) && resolvedModel != "" {
					if resolvedPrompt != "" && !hasFlag(dArgs, "-p", "--prompt") {
						dArgs = append([]string{"-p", resolvedPrompt}, dArgs...)
					}
					if isAgy {
						if !hasFlag(dArgs, "--dangerously-skip-permissions") {
							dArgs = append(dArgs, "--dangerously-skip-permissions")
						}
						if !hasFlag(dArgs, "--disable-slash-commands") {
							dArgs = append(dArgs, "--disable-slash-commands")
						}
					}
					hasModel := false
					for _, arg := range dArgs {
						if arg == "--model" || strings.HasPrefix(arg, "--model=") {
							hasModel = true
							break
						}
					}
					if !hasModel {
						dArgs = append(dArgs, "--model", resolvedModel)
					}
				} else if isAgy {
					if resolvedPrompt != "" && !hasFlag(dArgs, "-p", "--prompt") {
						dArgs = append([]string{"-p", resolvedPrompt}, dArgs...)
					}
					if !hasFlag(dArgs, "--dangerously-skip-permissions") {
						dArgs = append(dArgs, "--dangerously-skip-permissions")
					}
					if !hasFlag(dArgs, "--disable-slash-commands") {
						dArgs = append(dArgs, "--disable-slash-commands")
					}
				}

				dOpts := DispatchOptions{
					ID:          taskID,
					Profile:     effectiveProfile,
					RepoPath:    repoRoot,
					Prompt:      resolvedPrompt,
					AgentType:   st.AgentType,
					Command:     st.Command,
					Args:        dArgs,
					NewWorktree: true,
					Branch:      branch,
					BaseCommit:  baseCommit,
					Cwd:         st.Cwd,
					Env:         st.Env,
					GatewayURL:  st.GatewayURL,
					GatewayKey:  st.GatewayKey,
					Verify:      st.Verify,
					Detached:    true,
					Metadata:    st.Metadata,
				}

				dispatchedTask, err := m.Dispatch(dOpts)
				if err != nil {
					res.Status = StatusFailed
					res.Error = err.Error()
					res.FailureReason = ClassifySubtaskFailure(res)
					res.TokensUsed = quota.EstimateTokens(res.Prompt)
					subtaskResults[idx] = res
					continue
				}

				res.WorktreeID = dispatchedTask.WorktreeID
				res.WorktreePath = dispatchedTask.WorktreePath

				// Wait for task completion
				completedTask, waitErr := m.WaitForTask(planCtx, repoRoot, taskID)
				if completedTask == nil {
					completedTask, _ = m.GetTask(repoRoot, taskID)
				}

				if planCtx.Err() != nil || (completedTask != nil && completedTask.Status == StatusRunning) {
					_ = m.CancelTask(repoRoot, taskID, true)
					completedTask, _ = m.GetTask(repoRoot, taskID)
				}

				if completedTask != nil {
					res.Status = completedTask.Status
					res.ExitCode = completedTask.ExitCode
					res.DurationSeconds = completedTask.DurationSeconds
					res.HeadCommit = completedTask.HeadCommit
					if completedTask.Error != "" {
						res.Error = completedTask.Error
					}
					res.VerificationExitCode = completedTask.VerificationExitCode
					res.VerificationOutput = tailString(completedTask.VerificationOutput, 1000)
					if res.WorktreePath == "" && completedTask.WorktreePath != "" {
						res.WorktreePath = completedTask.WorktreePath
						res.WorktreeID = completedTask.WorktreeID
					}
				}

				if res.Status == "" || res.Status == StatusRunning {
					if waitErr != nil {
						res.Status = StatusFailed
						res.Error = waitErr.Error()
					} else if planCtx.Err() != nil {
						res.Status = StatusCancelled
					}
				} else if waitErr != nil && res.Error == "" {
					res.Error = waitErr.Error()
				}

				// Capture diffs even if cancelled/timed out so partial diffs are preserved when non-empty
				if res.WorktreePath != "" {
					rawDiff, diffErr := worktree.GetWorktreeDiff(repoRoot, res.WorktreeID, worktree.DiffOptions{Paths: st.Paths})
					if diffErr == nil {
						res.Diff = rawDiff
						sd := ParseUnifiedDiff(rawDiff)
						res.StructuredDiff = sd
						files := make([]string, 0, len(sd.Files))
						for _, f := range sd.Files {
							files = append(files, f.NewPath)
						}
						res.FilesChanged = files
					} else if res.Error == "" {
						res.Error = fmt.Sprintf("diff error: %v", diffErr)
					}
				}

				// Verify-on-Timeout recovery: run Verify: on deadline if diff exists; recover as completed on exit 0
				isDeadline := errors.Is(planCtx.Err(), context.DeadlineExceeded) ||
					errors.Is(waitErr, context.DeadlineExceeded) ||
					(planCtx.Err() != nil && strings.Contains(planCtx.Err().Error(), "deadline exceeded")) ||
					(waitErr != nil && strings.Contains(waitErr.Error(), "deadline exceeded"))

				if isDeadline && strings.TrimSpace(st.Verify) != "" && strings.TrimSpace(res.Diff) != "" {
					verifyDir := res.WorktreePath
					if verifyDir == "" {
						verifyDir = repoRoot
					}
					out, exitCode, vErr := executeVerifyCommand(verifyDir, st.Verify)
					res.VerificationExitCode = exitCode
					res.VerificationOutput = tailString(string(out), 1000)

					if completedTask != nil && completedTask.LogPath != "" {
						if f, err := os.OpenFile(completedTask.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
							_, _ = fmt.Fprintf(f, "\n=== Verification (%s) ===\n%s\n", st.Verify, string(out))
							_ = f.Close()
						}
					}

					if exitCode == 0 && vErr == nil {
						res.Status = StatusCompleted
						res.ExitCode = 0
						res.Error = ""
						if completedTask != nil {
							completedTask.Status = StatusCompleted
							completedTask.ExitCode = 0
							completedTask.Error = ""
							completedTask.VerificationExitCode = 0
							completedTask.VerificationOutput = string(out)
							taskDir := filepath.Join(getTasksDir(repoRoot), taskID)
							_ = saveTaskFile(taskDir, completedTask)
							taskMu.Lock()
							if manifest, err := m.loadManifest(repoRoot); err == nil {
								manifest.Tasks[taskID] = *completedTask
								_ = m.saveManifest(repoRoot, manifest)
							}
							taskMu.Unlock()
						}
					} else {
						if vErr != nil && exitCode == 0 {
							res.Error = fmt.Sprintf("verification execution error: %v", vErr)
						} else {
							res.Error = fmt.Sprintf("verification command failed with exit code %d", exitCode)
						}
						res.Status = StatusAuditFailed
						if completedTask != nil {
							completedTask.Status = StatusAuditFailed
							completedTask.VerificationExitCode = exitCode
							completedTask.VerificationOutput = string(out)
							completedTask.Error = res.Error
							taskDir := filepath.Join(getTasksDir(repoRoot), taskID)
							_ = saveTaskFile(taskDir, completedTask)
							taskMu.Lock()
							if manifest, err := m.loadManifest(repoRoot); err == nil {
								manifest.Tasks[taskID] = *completedTask
								_ = m.saveManifest(repoRoot, manifest)
							}
							taskMu.Unlock()
						}
					}
				}

				if res.Status != StatusCompleted || strings.TrimSpace(res.Diff) == "" {
					if logs, err := m.GetTaskLogs(repoRoot, taskID, 1000); err == nil && len(logs) > 0 {
						res.LogTail = string(logs)
					} else if completedTask != nil && completedTask.LogPath != "" {
						if data, err := os.ReadFile(completedTask.LogPath); err == nil && len(data) > 0 {
							res.LogTail = tailString(string(data), 1000)
						}
					}
				}

				if len(st.Paths) > 0 && strings.TrimSpace(res.Diff) == "" && res.Status == StatusCompleted {
					res.Status = StatusAuditFailed
					res.Error = "empty_diff: subtask exited without modifying any allowed paths"
				}

				if req.ApplyAtomic && res.Status == StatusCompleted && res.Diff != "" {
					if err := worktree.GitApply(repoRoot, res.Diff, false, false); err != nil {
						res.Status = StatusFailed
						res.Error = fmt.Sprintf("apply diff error: %v", err)
					} else {
						// Stage st.Paths or git add -A
						addArgs := []string{"add"}
						if len(st.Paths) > 0 {
							addArgs = append(addArgs, st.Paths...)
						} else {
							addArgs = append(addArgs, "-A")
						}
						addCmd := exec.Command("git", addArgs...)
						addCmd.Dir = repoRoot
						if out, err := addCmd.CombinedOutput(); err != nil {
							res.Status = StatusFailed
							res.Error = fmt.Sprintf("git add error: %v (%s)", err, strings.TrimSpace(string(out)))
						} else {
							commitMsg := st.CommitMessage
							if commitMsg == "" {
								commitMsg = fmt.Sprintf("chore: apply subtask %s", stID)
							}
							commitCmd := exec.Command("git", "commit", "-m", commitMsg)
							commitCmd.Dir = repoRoot
							if out, err := commitCmd.CombinedOutput(); err != nil {
								res.Status = StatusFailed
								res.Error = fmt.Sprintf("git commit error: %v (%s)", err, strings.TrimSpace(string(out)))
							} else {
								// Retrieve short SHA
								revCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
								revCmd.Dir = repoRoot
								if sha, err := revCmd.Output(); err == nil {
									res.Commit = strings.TrimSpace(string(sha))
								}
							}
						}
					}
				}

				if res.Status != StatusCompleted || res.ExitCode != 0 {
					res.FailureReason = ClassifySubtaskFailure(res)
					if res.FailureReason == "rate_limited" && res.Profile != "" {
						dur := quota.ParseQuotaResetDuration(res.LogTail+"\n"+res.Error, 5*time.Minute)
						quota.RecordProfileCooldown(res.Profile, dur)
					}
				}

				res.TokensUsed = quota.EstimateTokens(res.Prompt + res.LogTail + res.Diff)

				if req.OmitDiff {
					res.Diff = ""
				}

				subtaskResults[idx] = res
			}
		}()
	}

	for idx := range subtasks {
		jobs <- idx
	}
	close(jobs)
	wg.Wait()

	endTime := time.Now().UTC()

	// Aggregate metrics and diffs
	succeededCount := 0
	failedCount := 0
	fileCountMap := make(map[string]int)
	totalAdditions := 0
	totalDeletions := 0
	var combinedDiffBuilder strings.Builder

	for _, res := range subtaskResults {
		if res.Status == StatusCompleted && res.ExitCode == 0 {
			succeededCount++
		} else {
			failedCount++
		}

		if res.StructuredDiff != nil {
			totalAdditions += res.StructuredDiff.Summary.Additions
			totalDeletions += res.StructuredDiff.Summary.Deletions
			for _, f := range res.StructuredDiff.Files {
				fileCountMap[f.NewPath]++
			}
		}

		if strings.TrimSpace(res.Diff) != "" {
			if combinedDiffBuilder.Len() > 0 {
				combinedDiffBuilder.WriteString("\n")
			}
			combinedDiffBuilder.WriteString(fmt.Sprintf("# Subtask: %s (profile: %s, branch: %s)\n", res.ID, res.Profile, res.Branch))
			combinedDiffBuilder.WriteString(res.Diff)
			if !strings.HasSuffix(res.Diff, "\n") {
				combinedDiffBuilder.WriteString("\n")
			}
		}
	}

	allFiles := make([]string, 0, len(fileCountMap))
	conflictingFiles := make([]string, 0)
	for f, count := range fileCountMap {
		allFiles = append(allFiles, f)
		if count > 1 {
			conflictingFiles = append(conflictingFiles, f)
		}
	}
	sort.Strings(allFiles)
	sort.Strings(conflictingFiles)

	overallStatus := "completed"
	if failedCount == 0 && succeededCount > 0 {
		overallStatus = "completed"
	} else if planCtx.Err() == context.DeadlineExceeded {
		overallStatus = "timeout"
	} else if planCtx.Err() == context.Canceled {
		overallStatus = "cancelled"
	} else if failedCount > 0 && succeededCount > 0 {
		overallStatus = "partial"
	} else if failedCount > 0 && succeededCount == 0 {
		overallStatus = "failed"
	}

	combinedDiff := combinedDiffBuilder.String()
	if req.OmitDiff {
		combinedDiff = ""
	}

	planResult := &PlanResult{
		PlanID:          planID,
		RepoPath:        repoRoot,
		Status:          overallStatus,
		TotalSubtasks:   len(subtasks),
		Succeeded:       succeededCount,
		Failed:          failedCount,
		DurationSeconds: endTime.Sub(startTime).Seconds(),
		CreatedAt:       startTime,
		EndedAt:         endTime,
		Subtasks:        subtaskResults,
		UnifiedSummary: UnifiedDiffSummary{
			TotalFilesChanged:   len(allFiles),
			TotalAdditions:      totalAdditions,
			TotalDeletions:      totalDeletions,
			ModifiedFiles:       allFiles,
			DisjointScopesClean: len(conflictingFiles) == 0,
			ConflictingFiles:    conflictingFiles,
			CombinedDiff:        combinedDiff,
		},
	}

	// Persist plan manifest to .multigravity/plans/<plan_id>.json
	plansDir := getPlansDir(repoRoot)
	_ = os.MkdirAll(plansDir, 0755)
	planFile := filepath.Join(plansDir, planID+".json")
	if data, err := json.MarshalIndent(planResult, "", "  "); err == nil {
		_ = os.WriteFile(planFile, append(data, '\n'), 0644)
	}

	// Persist complete run artifact to <repoRoot>/.multigravity/runs/<plan_id>.json
	if repoRoot != "" {
		runsDir := filepath.Join(repoRoot, ".multigravity", "runs")
		_ = os.MkdirAll(runsDir, 0755)
		runFile := filepath.Join(runsDir, planID+".json")
		if data, err := json.MarshalIndent(planResult, "", "  "); err == nil {
			_ = os.WriteFile(runFile, append(data, '\n'), 0644)
		}
	}

	return planResult, nil
}

// GetPlan retrieves a previously executed plan result from disk.
func (m *TaskManager) GetPlan(repoPath, planID string) (*PlanResult, error) {
	if strings.TrimSpace(planID) == "" {
		return nil, errors.New("plan ID is required")
	}

	var repoRoot string
	if repoPath != "" {
		if root, err := worktree.FindRepoRoot(repoPath); err == nil {
			repoRoot = root
		}
	} else {
		if cwd, err := os.Getwd(); err == nil {
			if root, err := worktree.FindRepoRoot(cwd); err == nil {
				repoRoot = root
			}
		}
	}

	plansDir := getPlansDir(repoRoot)
	planFile := filepath.Join(plansDir, planID+".json")
	data, err := os.ReadFile(planFile)
	if err != nil {
		return nil, fmt.Errorf("plan %q not found: %w", planID, err)
	}

	var res PlanResult
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("failed to parse plan %q: %w", planID, err)
	}
	return &res, nil
}

// ListPlans scans and returns all executed plans found in the repository or home.
func (m *TaskManager) ListPlans(repoPath string) ([]PlanResult, error) {
	var repoRoot string
	if repoPath != "" {
		if root, err := worktree.FindRepoRoot(repoPath); err == nil {
			repoRoot = root
		}
	} else {
		if cwd, err := os.Getwd(); err == nil {
			if root, err := worktree.FindRepoRoot(cwd); err == nil {
				repoRoot = root
			}
		}
	}

	plansDir := getPlansDir(repoRoot)
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []PlanResult{}, nil
		}
		return nil, err
	}

	results := make([]PlanResult, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			filePath := filepath.Join(plansDir, entry.Name())
			if data, err := os.ReadFile(filePath); err == nil {
				var res PlanResult
				if err := json.Unmarshal(data, &res); err == nil {
					results = append(results, res)
				}
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results, nil
}
