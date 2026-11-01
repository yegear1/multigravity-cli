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

	"github.com/ye-dev/multigravity-cli/internal/quota"
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
	NoRetry      bool          `json:"no_retry,omitempty"`
}

// WaveExecutionResult encapsulates the outcome of executing a wave of tasks.
type WaveExecutionResult struct {
	WaveStatus     string          `json:"wave_status"` // "completed", "failed", "partial"
	WaveTag        string          `json:"wave_tag"`
	Tasks          []SubtaskResult `json:"tasks"`
	TaskFileCommit string          `json:"task_file_commit,omitempty"`
}

var (
	taskItemRegex = regexp.MustCompile(`^-\s*\[([ xX])\]\s*\*\*\[([^\]]+)\]\*\*\s*(.*)$`)
	waveTagRegex  = regexp.MustCompile(`\[(PARALLEL|SEQ):\s*([^\]]+)\]`)
)

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
				Title:    rest,
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

	timeoutStr := "4m"
	if opts.Timeout > 0 {
		timeoutStr = opts.Timeout.String()
	}

	// Build PlanRequest
	subtasks := make([]SubtaskSpec, len(waveSpec.Tasks))
	for i, t := range waveSpec.Tasks {
		subtasks[i] = SubtaskSpec{
			ID:            t.ID,
			Profile:       "auto",
			Prompt:        t.Title,
			Verify:        t.Verify,
			Read:          t.Read,
			Paths:         t.Paths,
			Model:         t.Model,
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

			retryModel := "gemini-4-argon-medium"
			if res.Model != "" && res.Model != "gemini-3.8-flash-low" {
				if nextM, ok := quota.EscalateModel(res.Model); ok && nextM != "" {
					retryModel = nextM
				}
			}

			// Retry with escalated model excluding the failed profile
			failedProfile := res.Profile
			var eligibleProfiles []string
			if candidates, qErr := quota.SelectEligibleProfilesForModel(100, retryModel, quota.WeightHeavy); qErr == nil {
				for _, p := range candidates {
					if p != failedProfile {
						eligibleProfiles = append(eligibleProfiles, p)
					}
				}
			}

			retryProfile := "auto"
			if len(eligibleProfiles) > 0 {
				retryProfile = eligibleProfiles[0]
			}

			retryPrompt := SynthesizeCascadeRetryPrompt(t.Title, res.Model, res.VerificationOutput, res.LogTail)

			retrySubtask := SubtaskSpec{
				ID:            t.ID,
				Profile:       retryProfile,
				Prompt:        retryPrompt,
				Verify:        t.Verify,
				Read:          t.Read,
				Paths:         t.Paths,
				Model:         retryModel,
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

	return execRes, nil
}

func markTasksCompletedInFile(filePath string, commitMap map[string]string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	modified := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if taskItemRegex.MatchString(trimmed) {
			matches := taskItemRegex.FindStringSubmatch(trimmed)
			taskID := matches[2]
			if commit, ok := commitMap[taskID]; ok {
				// Replace - [ ] with - [x] and append commit short sha if available
				// Example: - [ ] **[05.10]** Title -> - [x] **[05.10]** Title (sha)
				indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				rest := matches[3]
				commitSuffix := ""
				if commit != "" && !strings.Contains(rest, "("+commit+")") {
					commitSuffix = fmt.Sprintf(" (%s)", commit)
				}
				lines[i] = fmt.Sprintf("%s- [x] **[%s]** %s%s", indent, taskID, rest, commitSuffix)
				modified = true
			}
		}
	}

	if modified {
		return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
	}
	return nil
}
