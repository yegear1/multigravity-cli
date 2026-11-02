package dispatch

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/agent"
	"github.com/ye-dev/multigravity-cli/internal/quota"
	"github.com/ye-dev/multigravity-cli/internal/worktree"
)

// WaveTaskSpec represents a parsed task item from TASK.md belonging to a wave.
type WaveTaskSpec struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	WaveTag  string   `json:"wave_tag"`
	Parallel bool     `json:"parallel"`
	Read     []string `json:"read,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	Verify   string   `json:"verify,omitempty"`
	Model    string   `json:"model,omitempty"`
	Role     string   `json:"role,omitempty"`
	ReadOnly bool     `json:"read_only,omitempty"`
}

// WaveSpec represents a collection of tasks belonging to the same wave tag.
type WaveSpec struct {
	WaveTag string         `json:"wave_tag"`
	Tasks   []WaveTaskSpec `json:"tasks"`
}

// WaveOptions defines configuration options for running a task wave.
type WaveOptions struct {
	RepoRoot     string        `json:"repo_root,omitempty"`
	TaskFile     string        `json:"task_file,omitempty"`
	WaveTag      string        `json:"wave_tag,omitempty"`
	Timeout      time.Duration `json:"timeout,omitempty"`
	DefaultModel string        `json:"default_model,omitempty"`
	Role         string        `json:"role,omitempty"`
	ReadOnly     bool          `json:"read_only,omitempty"`
	NoRetry      bool          `json:"no_retry,omitempty"`
	Probe        bool          `json:"probe,omitempty"`
}

// WaveExecutionResult encapsulates the outcome of executing a wave of tasks.
type WaveExecutionResult struct {
	WaveStatus     string          `json:"wave_status"` // "completed", "failed", "partial"
	WaveTag        string          `json:"wave_tag"`
	Tasks          []SubtaskResult `json:"tasks"`
	TaskFileCommit string          `json:"task_file_commit,omitempty"`
}

// ToCompact returns a copy of WaveExecutionResult where each SubtaskResult in Tasks
// is sanitized with SanitizeSubtaskCompact (omitting prompt dumps and raw diffs).
func (r *WaveExecutionResult) ToCompact() *WaveExecutionResult {
	if r == nil {
		return nil
	}
	compact := *r
	if len(r.Tasks) > 0 {
		compact.Tasks = make([]SubtaskResult, len(r.Tasks))
		for i, t := range r.Tasks {
			compact.Tasks[i] = SanitizeSubtaskCompact(t)
		}
	}
	return &compact
}

// Compact is an alias for ToCompact.
func (r *WaveExecutionResult) Compact() *WaveExecutionResult {
	return r.ToCompact()
}

var (
	taskItemRegex        = regexp.MustCompile(`^-\s*\[([ xX])\]\s*\*\*\[([^\]]+)\]\*\*\s*(.*)$`)
	waveTagRegex         = regexp.MustCompile(`\[(PARALLEL|SEQ):\s*([^\]]+)\]`)
	backtickWaveTagRegex = regexp.MustCompile("`?\\[(PARALLEL|SEQ):\\s*([^\\]]+)\\]`?")
	checkboxRegex        = regexp.MustCompile(`^(\s*-\s*\[)[ ](\].*)$`)
)

func stripTaskTags(s string) string {
	s = backtickWaveTagRegex.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "`[APPROVED]`", "")
	s = strings.ReplaceAll(s, "[APPROVED]", "")
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

func cleanBackticksAndSpaces(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`")
	return strings.TrimSpace(s)
}

func parseListItems(val string) []string {
	parts := strings.Split(val, ",")
	var result []string
	for _, p := range parts {
		cleaned := cleanBackticksAndSpaces(p)
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}

// ParseTaskWave parses .agent/TASK.md backlog items matching - [ ] **[XX.Y]** with [APPROVED]
// and [PARALLEL: Wx] or [SEQ: Wx], plus their indented bullets.
// When waveTag == "", it selects the first uncompleted [APPROVED] wave tag.
// Returns an error if parallel tasks have overlapping Paths: or if no matching wave is found.
func ParseTaskWave(taskFilePath, waveTag string) (*WaveSpec, error) {
	f, err := os.Open(taskFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open task file %s: %w", taskFilePath, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var allTasks []WaveTaskSpec
	var currentTask *WaveTaskSpec

	flushCurrent := func() {
		if currentTask != nil {
			allTasks = append(allTasks, *currentTask)
			currentTask = nil
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Check if line is a task header
		if taskItemRegex.MatchString(trimmed) {
			flushCurrent()
			matches := taskItemRegex.FindStringSubmatch(trimmed)
			checked := strings.ToLower(matches[1]) == "x"
			taskID := matches[2]
			rest := matches[3]

			// Only process uncompleted tasks
			if checked {
				continue
			}

			// Must contain [APPROVED]
			if !strings.Contains(rest, "[APPROVED]") {
				continue
			}

			// Check for [PARALLEL: Wx] or [SEQ: Wx]
			waveMatches := waveTagRegex.FindStringSubmatch(rest)
			if waveMatches == nil {
				continue
			}

			mode := waveMatches[1]
			tag := strings.TrimSpace(waveMatches[2])
			parallel := (mode == "PARALLEL")

			currentTask = &WaveTaskSpec{
				ID:       taskID,
				Title:    stripTaskTags(rest),
				WaveTag:  tag,
				Parallel: parallel,
			}
			continue
		}

		if currentTask != nil {
			// Check for bullet lines under task
			// Bullets look like: - `Read:` `...` or - `Read:` ... or - Read: ...
			// Could be indented with spaces or tabs.
			if strings.HasPrefix(trimmed, "-") {
				bulletContent := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
				// Check for known fields: Read:, Paths:, Verify:, Model:
				// Strip surrounding backticks if present around the field name, e.g. `Read:` or Read:
				lower := strings.ToLower(bulletContent)

				if matchField(&bulletContent, lower, "read:") {
					currentTask.Read = parseListItems(bulletContent)
				} else if matchField(&bulletContent, lower, "paths:") {
					currentTask.Paths = parseListItems(bulletContent)
				} else if matchField(&bulletContent, lower, "verify:") {
					currentTask.Verify = cleanBackticksAndSpaces(bulletContent)
				} else if matchField(&bulletContent, lower, "model:") {
					currentTask.Model = cleanBackticksAndSpaces(bulletContent)
				} else if matchField(&bulletContent, lower, "role:") {
					currentTask.Role = cleanBackticksAndSpaces(bulletContent)
				} else if matchField(&bulletContent, lower, "read-only:") || matchField(&bulletContent, lower, "readonly:") {
					val := strings.ToLower(cleanBackticksAndSpaces(bulletContent))
					currentTask.ReadOnly = (val == "true" || val == "1" || val == "yes")
				}
			} else if trimmed != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				// Reached an unindented non-bullet line, flush current task
				flushCurrent()
			}
		}
	}
	flushCurrent()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading task file: %w", err)
	}

	// Select wave tag if empty
	targetWave := waveTag
	if targetWave == "" {
		for _, t := range allTasks {
			if t.WaveTag != "" {
				targetWave = t.WaveTag
				break
			}
		}
	}

	if targetWave == "" {
		return nil, fmt.Errorf("no matching approved wave found in %s", taskFilePath)
	}

	var waveTasks []WaveTaskSpec
	for _, t := range allTasks {
		if t.WaveTag == targetWave {
			waveTasks = append(waveTasks, t)
		}
	}

	if len(waveTasks) == 0 {
		return nil, fmt.Errorf("no tasks found for wave tag %q in %s", targetWave, taskFilePath)
	}

	// Validate overlapping paths for parallel tasks
	seenPaths := make(map[string]string) // path -> task ID
	for _, t := range waveTasks {
		if t.Parallel {
			for _, p := range t.Paths {
				cleanP := filepath.Clean(p)
				if otherID, exists := seenPaths[cleanP]; exists {
					return nil, fmt.Errorf("overlapping path %q detected between parallel tasks [%s] and [%s]", cleanP, otherID, t.ID)
				}
				seenPaths[cleanP] = t.ID
			}
		}
	}

	return &WaveSpec{
		WaveTag: targetWave,
		Tasks:   waveTasks,
	}, nil
}

// LintTaskWave validates a parsed WaveSpec against governance and architectural rules.
// It reports violations when:
// (1) a TierMicro ('gemini-3.8-flash-low' or quota.TierForModel(t.Model) == quota.TierMicro) task
//     has > 2 Paths: files or spans multiple Go packages (distinct filepath.Dir of Paths),
// (2) any task has > 4 Read: entries, or
// (3) any task is missing Verify: (strings.TrimSpace(t.Verify) == "").
func LintTaskWave(waveSpec *WaveSpec) []string {
	if waveSpec == nil {
		return []string{}
	}

	violations := make([]string, 0)

	for _, t := range waveSpec.Tasks {
		model := t.Model
		if model == "" {
			model = "gemini-3.8-flash-medium"
		}
		isTierMicro := model == "gemini-3.8-flash-low" || quota.TierForModel(model) == quota.TierMicro
		if isTierMicro {
			if len(t.Paths) > 2 {
				violations = append(violations, fmt.Sprintf("task [%s]: TierMicro task has %d Paths files (maximum 2)", t.ID, len(t.Paths)))
			}

			dirs := make(map[string]struct{})
			for _, p := range t.Paths {
				dirs[filepath.Dir(filepath.Clean(p))] = struct{}{}
			}
			if len(dirs) > 1 {
				violations = append(violations, fmt.Sprintf("task [%s]: TierMicro task spans %d Go packages (distinct directories: %d)", t.ID, len(dirs), len(dirs)))
			}
		}

		if len(t.Read) > 4 {
			violations = append(violations, fmt.Sprintf("task [%s]: task has %d Read entries (maximum 4)", t.ID, len(t.Read)))
		}

		if strings.TrimSpace(t.Verify) == "" {
			violations = append(violations, fmt.Sprintf("task [%s]: missing Verify command", t.ID))
		}
	}

	return violations
}

func matchField(content *string, lower, fieldPrefix string) bool {
	// matches `field:` or field:
	// If bulletContent is `Read:` `...`, or `Read:` ..., or Read: ...
	c := strings.TrimSpace(*content)
	// Check `field:`
	backtickedField := "`" + fieldPrefix + "`"
	if strings.HasPrefix(strings.ToLower(c), backtickedField) {
		*content = strings.TrimSpace(c[len(backtickedField):])
		return true
	}
	// Check `field`
	if strings.HasPrefix(strings.ToLower(c), fieldPrefix) {
		*content = strings.TrimSpace(c[len(fieldPrefix):])
		return true
	}
	// Also check if field without colon is in backticks followed by colon, e.g. `Read`:
	rawField := strings.TrimSuffix(fieldPrefix, ":")
	if strings.HasPrefix(strings.ToLower(c), "`"+rawField+"`:") {
		*content = strings.TrimSpace(c[len(rawField)+3:])
		return true
	}
	return false
}

// Manager is an alias to TaskManager for wave execution.
type Manager = TaskManager

// ExecuteTaskWave executes a wave of tasks defined in TaskFile.
func (m *TaskManager) ExecuteTaskWave(ctx context.Context, opts WaveOptions) (*WaveExecutionResult, error) {
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %w", err)
		}
		repoRoot = cwd
	}

	taskFile := opts.TaskFile
	if taskFile == "" {
		taskFile = filepath.Join(repoRoot, ".agent", "TASK.md")
	} else if !filepath.IsAbs(taskFile) {
		taskFile = filepath.Join(repoRoot, taskFile)
	}

	waveSpec, err := ParseTaskWave(taskFile, opts.WaveTag)
	if err != nil {
		return nil, err
	}

	if opts.Probe {
		// Probe inactive profiles before planning to refresh quota metrics and awaken secondary accounts
		_, _ = quota.ProbeInactiveProfiles(nil)
	}

	timeoutStr := "4m"
	if opts.Timeout > 0 {
		timeoutStr = opts.Timeout.String()
	}

	// Build PlanRequest
	subtasks := make([]SubtaskSpec, len(waveSpec.Tasks))
	for i, t := range waveSpec.Tasks {
		taskRole := t.Role
		if taskRole == "" {
			taskRole = opts.Role
		}
		taskReadOnly := t.ReadOnly || opts.ReadOnly
		subtasks[i] = SubtaskSpec{
			ID:            t.ID,
			Profile:       "auto",
			Prompt:        t.Title,
			Verify:        t.Verify,
			Read:          t.Read,
			Paths:         t.Paths,
			Model:         t.Model,
			Role:          taskRole,
			ReadOnly:      taskReadOnly,
			CommitMessage: fmt.Sprintf("feat(%s): %s", t.ID, t.Title),
		}
	}

	req := PlanRequest{
		PlanID:       fmt.Sprintf("wave-%s-%d", waveSpec.WaveTag, time.Now().Unix()),
		RepoPath:     repoRoot,
		Timeout:      timeoutStr,
		Workers:      len(subtasks),
		Subtasks:     subtasks,
		DefaultModel: opts.DefaultModel,
		ApplyAtomic:  true,
		OmitDiff:     true,
		InlineRead:   true,
	}

	planRes, err := m.ExecutePlan(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("wave plan execution failed: %w", err)
	}

	resultMap := make(map[string]SubtaskResult)
	for _, st := range planRes.Subtasks {
		resultMap[st.ID] = st
	}

	// Retry failed tasks once if !opts.NoRetry
	if !opts.NoRetry {
		for _, t := range waveSpec.Tasks {
			res := resultMap[t.ID]
			if res.Status == StatusCompleted {
				continue
			}

			// (3) on 'empty_diff' or 'timeout' when VerificationOutput == "" and LogTail == "", skip retry altogether and preserve the failure reason.
			if (res.FailureReason == "empty_diff" || res.FailureReason == "timeout") && res.VerificationOutput == "" && res.LogTail == "" {
				continue
			}

			currentModel := res.Model
			if currentModel == "" {
				currentModel = t.Model
			}
			if currentModel == "" {
				currentModel = opts.DefaultModel
			}
			if currentModel == "" {
				currentModel = "gemini-3.8-flash-medium"
			}

			retryModel := currentModel
			if res.FailureReason == "rate_limited" {
				if res.Profile != "" {
					dur := quota.ParseQuotaResetDuration(res.LogTail+"\n"+res.Error, 5*time.Minute)
					quota.RecordProfileCooldown(res.Profile, dur)
				}
				// retry with the SAME model on an alternate profile (do not escalate model)
				retryModel = currentModel
			} else if (res.FailureReason == "verify_failed" && res.VerificationOutput != "") || res.FailureReason == "agent_error" {
				if nextM, ok := quota.EscalateModel(currentModel); ok && nextM != "" {
					retryModel = nextM
				}
			}

			// Retry with selected model excluding the failed profile
			failedProfile := res.Profile
			var eligibleProfiles []string
			threshold := quota.SafetyThresholdForModel(retryModel, quota.WeightMedium)
			if candidates, qErr := quota.PreflightProfilesForModel(100, retryModel, quota.WeightMedium, threshold, failedProfile); qErr == nil {
				for _, c := range candidates {
					if c.Profile != failedProfile {
						eligibleProfiles = append(eligibleProfiles, c.Profile)
					}
				}
			}
			if len(eligibleProfiles) == 0 && threshold > quota.DefaultSafetyQuotaThreshold {
				if fallbackCandidates, fbErr := quota.PreflightProfilesForModel(100, retryModel, quota.WeightMedium, quota.DefaultSafetyQuotaThreshold, failedProfile); fbErr == nil {
					for _, c := range fallbackCandidates {
						if c.Profile != failedProfile {
							eligibleProfiles = append(eligibleProfiles, c.Profile)
						}
					}
				}
			}

			retryProfile := "auto"
			if len(eligibleProfiles) > 0 {
				retryProfile = eligibleProfiles[0]
				_ = agent.EnsureProfileReady(retryProfile)
			}

			retryPrompt := SynthesizeCascadeRetryPrompt(t.Title, res.Model, res.VerificationOutput, res.LogTail)

			retryRole := t.Role
			if retryRole == "" {
				retryRole = opts.Role
			}
			retryReadOnly := t.ReadOnly || opts.ReadOnly

			retrySubtask := SubtaskSpec{
				ID:            t.ID,
				Profile:       retryProfile,
				Prompt:        retryPrompt,
				Verify:        t.Verify,
				Read:          t.Read,
				Paths:         t.Paths,
				Model:         retryModel,
				Role:          retryRole,
				ReadOnly:      retryReadOnly,
				CommitMessage: fmt.Sprintf("feat(%s): %s", t.ID, t.Title),
			}

			retryReq := PlanRequest{
				PlanID:       fmt.Sprintf("retry-%s-%s-%d", waveSpec.WaveTag, t.ID, time.Now().Unix()),
				RepoPath:     repoRoot,
				Timeout:      timeoutStr,
				Workers:      1,
				Subtasks:     []SubtaskSpec{retrySubtask},
				ApplyAtomic:  true,
				OmitDiff:     true,
				InlineRead:   true,
			}

			retryPlanRes, retryErr := m.ExecutePlan(ctx, retryReq)
			if retryErr == nil && len(retryPlanRes.Subtasks) > 0 {
				resultMap[t.ID] = retryPlanRes.Subtasks[0]
			}
		}
	}

	finalTasks := make([]SubtaskResult, len(waveSpec.Tasks))
	allSucceeded := true
	for i, t := range waveSpec.Tasks {
		finalTasks[i] = resultMap[t.ID]
		if finalTasks[i].Status != StatusCompleted {
			allSucceeded = false
		}
	}

	waveStatus := "completed"
	if !allSucceeded {
		hasSuccess := false
		for _, r := range finalTasks {
			if r.Status == StatusCompleted {
				hasSuccess = true
				break
			}
		}
		if hasSuccess {
			waveStatus = "partial"
		} else {
			waveStatus = "failed"
		}
	}

	execRes := &WaveExecutionResult{
		WaveStatus: waveStatus,
		WaveTag:    waveSpec.WaveTag,
		Tasks:      finalTasks,
	}

	// When all wave tasks succeed, mark them [x] with (<commit>) in TaskFile and commit TaskFile.
	if allSucceeded {
		commitMap := make(map[string]string)
		for _, st := range finalTasks {
			commitMap[st.ID] = st.Commit
		}

		if err := markTasksCompletedInFile(taskFile, commitMap); err != nil {
			return execRes, fmt.Errorf("failed to update task file: %w", err)
		}

		// Stage and commit TaskFile
		relTaskFile, relErr := filepath.Rel(repoRoot, taskFile)
		if relErr != nil {
			relTaskFile = taskFile
		}

		addCmd := exec.Command("git", "add", relTaskFile)
		addCmd.Dir = repoRoot
		if out, err := addCmd.CombinedOutput(); err != nil {
			return execRes, fmt.Errorf("git add %s failed: %v (%s)", relTaskFile, err, strings.TrimSpace(string(out)))
		}

		commitMsg := fmt.Sprintf("docs(tasks): complete wave %s", waveSpec.WaveTag)
		commitCmd := exec.Command("git", "commit", "-m", commitMsg)
		commitCmd.Dir = repoRoot
		if out, err := commitCmd.CombinedOutput(); err != nil {
			return execRes, fmt.Errorf("git commit failed: %v (%s)", err, strings.TrimSpace(string(out)))
		}

		revCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
		revCmd.Dir = repoRoot
		if sha, err := revCmd.Output(); err == nil {
			execRes.TaskFileCommit = strings.TrimSpace(string(sha))
		}
	}

	// Ensure worktree and branch cleanup for all wave tasks
	for _, st := range finalTasks {
		if st.WorktreeID != "" {
			_ = worktree.RemoveWorktree(repoRoot, st.WorktreeID, worktree.RemoveOptions{
				Force:        true,
				DeleteBranch: true,
			})
		}
	}
	_ = worktree.PruneWorktrees(repoRoot)

	return execRes, nil
}

func markTasksCompletedInFile(filePath string, commitMap map[string]string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	modified := false

	type completedEntry struct {
		id     string
		title  string
		commit string
	}
	var newlyCompleted []completedEntry

	type nextTaskInfo struct {
		id    string
		title string
	}
	var nextApproved *nextTaskInfo

	inBacklog := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inBacklog = strings.HasPrefix(trimmed, "## Backlog")
		}

		if taskItemRegex.MatchString(trimmed) {
			matches := taskItemRegex.FindStringSubmatch(trimmed)
			checked := strings.ToLower(matches[1]) == "x"
			taskID := matches[2]
			rest := matches[3]

			if commit, ok := commitMap[taskID]; ok {
				indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				commitSuffix := ""
				if commit != "" && !strings.Contains(rest, "("+commit+")") {
					commitSuffix = fmt.Sprintf(" (%s)", commit)
				}
				lines[i] = fmt.Sprintf("%s- [x] **[%s]** %s%s", indent, taskID, rest, commitSuffix)
				modified = true
				if !checked {
					newlyCompleted = append(newlyCompleted, completedEntry{
						id:     taskID,
						title:  stripTaskTags(rest),
						commit: commit,
					})
				}
			} else if !checked && inBacklog && nextApproved == nil && strings.Contains(rest, "[APPROVED]") {
				nextApproved = &nextTaskInfo{
					id:    taskID,
					title: stripTaskTags(rest),
				}
			}
		}
	}

	if len(newlyCompleted) == 0 && !modified {
		return nil
	}

	// (1) Prepend completed wave tasks to '## Completed Tasks Log' table
	if len(newlyCompleted) > 0 {
		today := time.Now().Format("2006-01-02")
		var newRows []string
		for _, c := range newlyCompleted {
			shaCol := "`-`"
			if c.commit != "" {
				shaCol = fmt.Sprintf("`%s`", c.commit)
			}
			newRows = append(newRows, fmt.Sprintf("| [%s] | %s | %s | %s |", c.id, c.title, shaCol, today))
		}

		inLogSection := false
		inserted := false
		for i := 0; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if strings.HasPrefix(trimmed, "## ") {
				if trimmed == "## Completed Tasks Log" {
					inLogSection = true
					continue
				} else if inLogSection {
					break
				}
			}
			if inLogSection && strings.HasPrefix(trimmed, "|") && strings.Contains(trimmed, "---") {
				updated := make([]string, 0, len(lines)+len(newRows))
				updated = append(updated, lines[:i+1]...)
				updated = append(updated, newRows...)
				updated = append(updated, lines[i+1:]...)
				lines = updated
				inserted = true
				modified = true
				break
			}
		}
		_ = inserted
	}

	// (2) Mark '### Acceptance Criteria' or '## Acceptance Criteria' checkboxes under '## Active Task' as '- [x]'
	// and (3) Promote '## Active Task' header/description to the next uncompleted '[APPROVED]' task in '## Backlog'
	// (or 'Queue Empty — Awaiting Next Ideation' with 'Status: READY FOR PLANNING' when none remain).
	activeStart := -1
	activeEnd := len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "## Active Task" {
			activeStart = i
			continue
		}
		if activeStart != -1 && i > activeStart && strings.HasPrefix(trimmed, "## ") && trimmed != "## Acceptance Criteria" {
			activeEnd = i
			break
		}
	}

	if activeStart != -1 {
		inAC := false
		for i := activeStart + 1; i < activeEnd; i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "### Acceptance Criteria" || trimmed == "## Acceptance Criteria" {
				inAC = true
				continue
			}
			if inAC {
				if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "---") {
					inAC = false
					continue
				}
				if m := checkboxRegex.FindStringSubmatch(lines[i]); m != nil {
					lines[i] = m[1] + "x" + m[2]
					modified = true
				}
			}
		}

		// Promote Active Task header/description & Status
		statusUpdated := false
		titleUpdated := false
		for i := activeStart + 1; i < activeEnd; i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "### Acceptance Criteria" || trimmed == "## Acceptance Criteria" || strings.HasPrefix(trimmed, "---") {
				break
			}
			lower := strings.ToLower(trimmed)
			if !statusUpdated && (strings.HasPrefix(lower, "**status:**") || strings.HasPrefix(lower, "status:") || strings.HasPrefix(lower, "- **status:**") || strings.HasPrefix(lower, "- status:")) {
				if nextApproved != nil {
					lines[i] = replaceStatusLine(lines[i], "APPROVED")
				} else {
					lines[i] = replaceStatusLine(lines[i], "READY FOR PLANNING")
				}
				statusUpdated = true
				modified = true
				continue
			}
			if !titleUpdated && (strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(lower, "**task:**") || strings.HasPrefix(lower, "task:") || strings.HasPrefix(trimmed, "**[")) {
				if nextApproved != nil {
					lines[i] = formatPromotedTaskLine(lines[i], nextApproved.id, nextApproved.title)
				} else {
					lines[i] = formatQueueEmptyLine(lines[i])
				}
				titleUpdated = true
				modified = true
			}
		}

		if !titleUpdated || !statusUpdated {
			var insertLines []string
			if !titleUpdated {
				if nextApproved != nil {
					insertLines = append(insertLines, fmt.Sprintf("### [%s] %s", nextApproved.id, nextApproved.title))
				} else {
					insertLines = append(insertLines, "### Queue Empty — Awaiting Next Ideation")
				}
			}
			if !statusUpdated {
				if nextApproved != nil {
					insertLines = append(insertLines, "Status: APPROVED")
				} else {
					insertLines = append(insertLines, "Status: READY FOR PLANNING")
				}
			}
			if len(insertLines) > 0 {
				updated := make([]string, 0, len(lines)+len(insertLines))
				updated = append(updated, lines[:activeStart+1]...)
				updated = append(updated, insertLines...)
				updated = append(updated, lines[activeStart+1:]...)
				lines = updated
				modified = true
			}
		}
	}

	if modified {
		return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
	}
	return nil
}

func replaceStatusLine(line, newStatus string) string {
	trimmed := strings.TrimSpace(line)
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(lower, "- **status:**"):
		return fmt.Sprintf("%s- **Status:** `%s`", indent, newStatus)
	case strings.HasPrefix(lower, "**status:**"):
		return fmt.Sprintf("%s**Status:** %s", indent, newStatus)
	case strings.HasPrefix(lower, "- status:"):
		return fmt.Sprintf("%s- Status: %s", indent, newStatus)
	default:
		return fmt.Sprintf("%sStatus: %s", indent, newStatus)
	}
}

func formatPromotedTaskLine(line, id, title string) string {
	trimmed := strings.TrimSpace(line)
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(trimmed, "### "):
		return fmt.Sprintf("%s### [%s] %s", indent, id, title)
	case strings.HasPrefix(lower, "**task:**"):
		return fmt.Sprintf("%s**Task:** [%s] %s", indent, id, title)
	case strings.HasPrefix(lower, "task:"):
		return fmt.Sprintf("%sTask: [%s] %s", indent, id, title)
	default:
		return fmt.Sprintf("%s**[%s]** %s", indent, id, title)
	}
}

func formatQueueEmptyLine(line string) string {
	trimmed := strings.TrimSpace(line)
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(trimmed, "### "):
		return fmt.Sprintf("%s### Queue Empty — Awaiting Next Ideation", indent)
	case strings.HasPrefix(lower, "**task:**"):
		return fmt.Sprintf("%s**Task:** Queue Empty — Awaiting Next Ideation", indent)
	case strings.HasPrefix(lower, "task:"):
		return fmt.Sprintf("%sTask: Queue Empty — Awaiting Next Ideation", indent)
	default:
		return fmt.Sprintf("%sQueue Empty — Awaiting Next Ideation", indent)
	}
}
