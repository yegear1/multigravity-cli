package dispatch

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/worktree"
)

// SubtaskSpec defines parameters for a single subtask within an execution plan.
type SubtaskSpec struct {
	ID         string            `json:"id,omitempty"`
	Profile    string            `json:"profile"`
	Prompt     string            `json:"prompt,omitempty"`
	AgentType  string            `json:"agent_type,omitempty"`
	Command    string            `json:"command,omitempty"`
	Args       []string          `json:"args,omitempty"`
	Branch     string            `json:"branch,omitempty"`
	BaseCommit string            `json:"base_commit,omitempty"`
	Cwd        string            `json:"cwd,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	GatewayURL string            `json:"gateway_url,omitempty"`
	GatewayKey string            `json:"gateway_key,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// PlanRequest defines the input payload for dispatching a batch of subtasks concurrently across worktrees.
type PlanRequest struct {
	PlanID     string        `json:"plan_id,omitempty"`
	RepoPath   string        `json:"repo_path,omitempty"`
	BaseBranch string        `json:"base_branch,omitempty"`
	BaseCommit string        `json:"base_commit,omitempty"`
	Timeout    string        `json:"timeout,omitempty"`
	Workers    int           `json:"workers,omitempty"`
	Subtasks   []SubtaskSpec `json:"subtasks,omitempty"`
	Tasks      []SubtaskSpec `json:"tasks,omitempty"` // Alias for Subtasks
}

// SubtaskResult encapsulates execution details and git diff produced by a single subtask.
type SubtaskResult struct {
	ID              string          `json:"id"`
	TaskID          string          `json:"task_id"`
	Profile         string          `json:"profile"`
	Prompt          string          `json:"prompt,omitempty"`
	AgentType       string          `json:"agent_type,omitempty"`
	Status          TaskStatus      `json:"status"`
	WorktreeID      string          `json:"worktree_id,omitempty"`
	WorktreePath    string          `json:"worktree_path,omitempty"`
	Branch          string          `json:"branch,omitempty"`
	BaseCommit      string          `json:"base_commit,omitempty"`
	HeadCommit      string          `json:"head_commit,omitempty"`
	ExitCode        int             `json:"exit_code"`
	DurationSeconds float64         `json:"duration_seconds"`
	Error           string          `json:"error,omitempty"`
	Diff            string          `json:"diff,omitempty"`
	StructuredDiff  *StructuredDiff `json:"structured_diff,omitempty"`
	FilesChanged    []string        `json:"files_changed,omitempty"`
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

// GeneratePlanID produces a unique identifier for a subtask aggregation plan.
func GeneratePlanID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("plan-%d-%s", time.Now().Unix(), hex.EncodeToString(b))
}

func getPlansDir(repoRoot string) string {
	if repoRoot != "" {
		return filepath.Join(repoRoot, ".multigravity", "plans")
	}
	return filepath.Join(config.GetMultigravityHome(), ".plans")
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

	// Validate profiles
	for i, st := range subtasks {
		if strings.TrimSpace(st.Profile) == "" {
			return nil, fmt.Errorf("subtask [%d]: profile name is required", i)
		}
		if err := config.ValidateProfileName(st.Profile); err != nil {
			return nil, fmt.Errorf("subtask [%d]: invalid profile name %q: %w", i, st.Profile, err)
		}
		if !profile.ProfileExists(st.Profile) {
			return nil, fmt.Errorf("subtask [%d]: profile %q does not exist", i, st.Profile)
		}
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

				res := SubtaskResult{
					ID:         stID,
					TaskID:     taskID,
					Profile:    st.Profile,
					Prompt:     st.Prompt,
					AgentType:  st.AgentType,
					Branch:     branch,
					BaseCommit: baseCommit,
					ExitCode:   -1,
				}

				dOpts := DispatchOptions{
					ID:          taskID,
					Profile:     st.Profile,
					RepoPath:    repoRoot,
					Prompt:      st.Prompt,
					AgentType:   st.AgentType,
					Command:     st.Command,
					Args:        st.Args,
					NewWorktree: true,
					Branch:      branch,
					BaseCommit:  baseCommit,
					Cwd:         st.Cwd,
					Env:         st.Env,
					GatewayURL:  st.GatewayURL,
					GatewayKey:  st.GatewayKey,
					Detached:    true,
					Metadata:    st.Metadata,
				}

				dispatchedTask, err := m.Dispatch(dOpts)
				if err != nil {
					res.Status = StatusFailed
					res.Error = err.Error()
					subtaskResults[idx] = res
					continue
				}

				res.WorktreeID = dispatchedTask.WorktreeID
				res.WorktreePath = dispatchedTask.WorktreePath

				// Wait for task completion
				completedTask, waitErr := m.WaitForTask(planCtx, repoRoot, taskID)
				if waitErr != nil {
					res.Status = StatusFailed
					res.Error = waitErr.Error()
					if completedTask != nil {
						res.ExitCode = completedTask.ExitCode
						res.DurationSeconds = completedTask.DurationSeconds
						res.HeadCommit = completedTask.HeadCommit
					}
					subtaskResults[idx] = res
					continue
				}

				if completedTask != nil {
					res.Status = completedTask.Status
					res.ExitCode = completedTask.ExitCode
					res.DurationSeconds = completedTask.DurationSeconds
					res.HeadCommit = completedTask.HeadCommit
					if completedTask.Error != "" {
						res.Error = completedTask.Error
					}
				}

				// Capture diffs
				if res.WorktreePath != "" {
					rawDiff, diffErr := m.GetTaskDiff(repoRoot, taskID, false)
					if diffErr == nil {
						res.Diff = rawDiff
						sd := ParseUnifiedDiff(rawDiff)
						res.StructuredDiff = sd
						files := make([]string, 0, len(sd.Files))
						for _, f := range sd.Files {
							files = append(files, f.NewPath)
						}
						res.FilesChanged = files
					} else {
						res.Error = fmt.Sprintf("diff error: %v", diffErr)
					}
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
	if planCtx.Err() == context.DeadlineExceeded {
		overallStatus = "timeout"
	} else if planCtx.Err() == context.Canceled {
		overallStatus = "cancelled"
	} else if failedCount > 0 && succeededCount > 0 {
		overallStatus = "partial"
	} else if failedCount > 0 && succeededCount == 0 {
		overallStatus = "failed"
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
			CombinedDiff:        combinedDiffBuilder.String(),
		},
	}

	// Persist plan manifest to .multigravity/plans/<plan_id>.json
	plansDir := getPlansDir(repoRoot)
	_ = os.MkdirAll(plansDir, 0755)
	planFile := filepath.Join(plansDir, planID+".json")
	if data, err := json.MarshalIndent(planResult, "", "  "); err == nil {
		_ = os.WriteFile(planFile, append(data, '\n'), 0644)
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
