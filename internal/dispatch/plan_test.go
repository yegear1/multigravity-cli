package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/agent"
)

func setupTestProfiles(t *testing.T, names ...string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	for _, name := range names {
		profDir := filepath.Join(home, name)
		if err := os.MkdirAll(profDir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestExecutePlanDisjointScopesAndAggregation(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfiles(t, "dev1", "dev2")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	// Create plan with 2 subtasks in disjoint files
	req := PlanRequest{
		PlanID:   "plan-test-disjoint",
		RepoPath: repoDir,
		Workers:  2,
		Subtasks: []SubtaskSpec{
			{
				ID:      "subtask-backend",
				Profile: "dev1",
				Command: "sh",
				Args:    []string{"-c", "mkdir -p pkg/backend && echo 'package backend' > pkg/backend/backend.go && git add -A"},
			},
			{
				ID:      "subtask-frontend",
				Profile: "dev2",
				Command: "sh",
				Args:    []string{"-c", "mkdir -p pkg/frontend && echo 'package frontend' > pkg/frontend/frontend.go && git add -A"},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := taskMgr.ExecutePlan(ctx, req)
	if err != nil {
		t.Fatalf("ExecutePlan failed: %v", err)
	}

	if result.PlanID != "plan-test-disjoint" {
		t.Errorf("expected plan ID 'plan-test-disjoint', got %q", result.PlanID)
	}
	if result.Status != "completed" {
		t.Errorf("expected plan status 'completed', got %q", result.Status)
	}
	if result.TotalSubtasks != 2 {
		t.Errorf("expected 2 total subtasks, got %d", result.TotalSubtasks)
	}
	if result.Succeeded != 2 {
		t.Errorf("expected 2 succeeded subtasks, got %d", result.Succeeded)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed subtasks, got %d", result.Failed)
	}

	// Verify Unified Summary
	summary := result.UnifiedSummary
	if summary.TotalFilesChanged != 2 {
		t.Errorf("expected 2 files changed, got %d (files: %v)", summary.TotalFilesChanged, summary.ModifiedFiles)
	}
	if !summary.DisjointScopesClean {
		t.Errorf("expected DisjointScopesClean to be true, got false (conflicting: %v)", summary.ConflictingFiles)
	}
	if len(summary.ConflictingFiles) != 0 {
		t.Errorf("expected no conflicting files, got %v", summary.ConflictingFiles)
	}
	if !strings.Contains(summary.CombinedDiff, "pkg/backend/backend.go") || !strings.Contains(summary.CombinedDiff, "pkg/frontend/frontend.go") {
		t.Errorf("expected CombinedDiff to contain both files, got:\n%s", summary.CombinedDiff)
	}

	// Verify persistence: GetPlan and ListPlans
	retrieved, err := taskMgr.GetPlan(repoDir, "plan-test-disjoint")
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if retrieved.PlanID != result.PlanID || retrieved.Succeeded != 2 {
		t.Errorf("retrieved plan mismatch: %+v", retrieved)
	}

	plans, err := taskMgr.ListPlans(repoDir)
	if err != nil {
		t.Fatalf("ListPlans failed: %v", err)
	}
	if len(plans) < 1 {
		t.Errorf("expected at least 1 plan in ListPlans, got %d", len(plans))
	}
}

func TestExecutePlanScopeCollisionDetection(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfiles(t, "dev1", "dev2")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	// Create plan with 2 subtasks that touch the SAME file
	req := PlanRequest{
		PlanID:   "plan-test-collision",
		RepoPath: repoDir,
		Workers:  2,
		Subtasks: []SubtaskSpec{
			{
				ID:      "subtask-a",
				Profile: "dev1",
				Command: "sh",
				Args:    []string{"-c", "echo 'update A' >> README.md"},
			},
			{
				ID:      "subtask-b",
				Profile: "dev2",
				Command: "sh",
				Args:    []string{"-c", "echo 'update B' >> README.md"},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := taskMgr.ExecutePlan(ctx, req)
	if err != nil {
		t.Fatalf("ExecutePlan failed: %v", err)
	}

	// Both subtasks modified README.md -> collision should be detected
	if result.UnifiedSummary.DisjointScopesClean {
		t.Errorf("expected DisjointScopesClean to be false when both touch README.md")
	}
	if len(result.UnifiedSummary.ConflictingFiles) != 1 || result.UnifiedSummary.ConflictingFiles[0] != "README.md" {
		t.Errorf("expected ConflictingFiles to be ['README.md'], got: %v", result.UnifiedSummary.ConflictingFiles)
	}
}

func TestExecutePlanValidationErrors(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev1")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	ctx := context.Background()

	// Empty subtasks
	_, err := taskMgr.ExecutePlan(ctx, PlanRequest{RepoPath: repoDir})
	if err == nil || !strings.Contains(err.Error(), "subtasks are required") {
		t.Errorf("expected error for empty subtasks, got: %v", err)
	}

	// Non-existent profile
	_, err = taskMgr.ExecutePlan(ctx, PlanRequest{
		RepoPath: repoDir,
		Subtasks: []SubtaskSpec{{Profile: "non-existent-prof-12345"}},
	})
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected error for non-existent profile, got: %v", err)
	}

	// Outside git repo
	nonGitDir := t.TempDir()
	_, err = taskMgr.ExecutePlan(ctx, PlanRequest{
		RepoPath: nonGitDir,
		Subtasks: []SubtaskSpec{{Profile: "dev1"}},
	})
	if err == nil || !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("expected error for non-git repo, got: %v", err)
	}
}

func TestExecutePlanPartialFailure(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfiles(t, "dev1", "dev2")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	req := PlanRequest{
		PlanID:   "plan-test-partial",
		RepoPath: repoDir,
		Workers:  2,
		Subtasks: []SubtaskSpec{
			{
				ID:      "subtask-ok",
				Profile: "dev1",
				Command: "sh",
				Args:    []string{"-c", "echo 'ok' > ok.txt"},
			},
			{
				ID:      "subtask-fail",
				Profile: "dev2",
				Command: "sh",
				Args:    []string{"-c", "exit 2"},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := taskMgr.ExecutePlan(ctx, req)
	if err != nil {
		t.Fatalf("ExecutePlan failed: %v", err)
	}

	if result.Status != "partial" {
		t.Errorf("expected status 'partial', got %q", result.Status)
	}
	if result.Succeeded != 1 || result.Failed != 1 {
		t.Errorf("expected 1 succeeded and 1 failed, got %d and %d", result.Succeeded, result.Failed)
	}
}

func TestTailString(t *testing.T) {
	if tailString("", 100) != "" {
		t.Errorf("expected empty string")
	}
	if tailString("hello world", 0) != "hello world" {
		t.Errorf("expected unchanged string for 0 maxBytes")
	}
	if tailString("hello world", -5) != "hello world" {
		t.Errorf("expected unchanged string for negative maxBytes")
	}
	if tailString("hello world", 50) != "hello world" {
		t.Errorf("expected unchanged string when len < maxBytes")
	}
	if tailString("0123456789", 4) != "6789" {
		t.Errorf("expected last 4 bytes '6789', got %q", tailString("0123456789", 4))
	}
}

func TestExecutePlanTimeoutSubtaskCancellation(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev1")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	req := PlanRequest{
		PlanID:   "plan-test-timeout",
		RepoPath: repoDir,
		Workers:  1,
		Timeout:  "500ms",
		Subtasks: []SubtaskSpec{
			{
				ID:      "subtask-slow",
				Profile: "dev1",
				Command: "sh",
				Args:    []string{"-c", "echo 'partial work' > partial.txt && sleep 10"},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := taskMgr.ExecutePlan(ctx, req)
	if err != nil {
		t.Fatalf("ExecutePlan failed: %v", err)
	}

	if result.Status != "timeout" {
		t.Errorf("expected plan status 'timeout', got %q", result.Status)
	}
	if len(result.Subtasks) != 1 {
		t.Fatalf("expected 1 subtask result, got %d", len(result.Subtasks))
	}

	stRes := result.Subtasks[0]
	if stRes.Status != StatusCancelled {
		t.Errorf("expected subtask status 'cancelled', got %q", stRes.Status)
	}

	// Verify partial diff is preserved even when timed out
	if !strings.Contains(stRes.Diff, "partial.txt") {
		t.Errorf("expected partial diff to contain 'partial.txt', got:\n%s", stRes.Diff)
	}
}

func TestExecutePlanVerificationOutputAndLogTail(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev1")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	req := PlanRequest{
		PlanID:   "plan-test-verify-logtail",
		RepoPath: repoDir,
		Workers:  1,
		Subtasks: []SubtaskSpec{
			{
				ID:      "subtask-audit-fail",
				Profile: "dev1",
				Command: "sh",
				Args:    []string{"-c", "echo 'initial agent log output' && echo 'working' > done.txt"},
				Verify:  "sh -c 'echo [failed-verification-reason] && exit 1'",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := taskMgr.ExecutePlan(ctx, req)
	if err != nil {
		t.Fatalf("ExecutePlan failed: %v", err)
	}

	if len(result.Subtasks) != 1 {
		t.Fatalf("expected 1 subtask, got %d", len(result.Subtasks))
	}

	stRes := result.Subtasks[0]
	if stRes.Status != StatusAuditFailed {
		t.Errorf("expected subtask status 'audit_failed', got %q", stRes.Status)
	}
	if stRes.VerificationExitCode != 1 {
		t.Errorf("expected VerificationExitCode 1, got %d", stRes.VerificationExitCode)
	}
	if !strings.Contains(stRes.VerificationOutput, "[failed-verification-reason]") {
		t.Errorf("expected VerificationOutput to contain '[failed-verification-reason]', got %q", stRes.VerificationOutput)
	}
	if !strings.Contains(stRes.LogTail, "initial agent log output") {
		t.Errorf("expected LogTail to contain 'initial agent log output', got %q", stRes.LogTail)
	}
	if !strings.Contains(stRes.Diff, "done.txt") {
		t.Errorf("expected Diff to contain 'done.txt', got %q", stRes.Diff)
	}
}

func TestSynthesizeSubtaskPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	fooFile := filepath.Join(tmpDir, "foo.go")
	lines := []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
	}
	if err := os.WriteFile(fooFile, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Test model default to gemini-3.8-flash-low when st.Model == "" && defaultModel == "" and Read/Paths non-empty
	st1 := SubtaskSpec{
		Prompt: "implement feature X",
		Read:   []string{"foo.go:2-4"},
		Paths:  []string{"foo.go"},
	}
	prompt1, model1 := SynthesizeSubtaskPromptWithRepo(st1, "", tmpDir, true)
	if model1 != "gemini-3.8-flash-low" {
		t.Errorf("expected model 'gemini-3.8-flash-low', got %q", model1)
	}
	// Guardrail check
	guardrail := "Read ONLY the specified files first. If a referenced type or constant is missing, you may run at most ONE targeted `grep` in that package instead of guessing. Do NOT run baseline tests before editing. Exit immediately once validation passes without writing a prose summary."
	if !strings.Contains(prompt1, guardrail) {
		t.Errorf("expected prompt to contain guardrail, got:\n%s", prompt1)
	}
	// Inline read injection check
	if !strings.Contains(prompt1, "[PRE-LOADED READ CONTEXT]") {
		t.Errorf("expected prompt to contain pre-loaded read context, got:\n%s", prompt1)
	}
	if !strings.Contains(prompt1, "line 2\nline 3\nline 4") {
		t.Errorf("expected prompt to contain lines 2-4, got:\n%s", prompt1)
	}
	if strings.Contains(prompt1, "line 1") || strings.Contains(prompt1, "line 5") {
		t.Errorf("expected prompt NOT to contain line 1 or line 5, got:\n%s", prompt1)
	}

	// Test defaultModel takes precedence over fallback
	_, model2 := SynthesizeSubtaskPrompt(st1, "custom-model")
	if model2 != "custom-model" {
		t.Errorf("expected model 'custom-model', got %q", model2)
	}

	// Test st.Model takes precedence over defaultModel
	st3 := SubtaskSpec{
		Model: "subtask-model",
		Paths: []string{"bar.go"},
	}
	_, model3 := SynthesizeSubtaskPrompt(st3, "custom-model")
	if model3 != "subtask-model" {
		t.Errorf("expected model 'subtask-model', got %q", model3)
	}
}

func TestValidatePlanAutoProfile(t *testing.T) {
	setupTestProfiles(t, "dev1")

	// Empty profile is allowed
	req1 := &PlanRequest{
		Subtasks: []SubtaskSpec{
			{Profile: ""},
		},
	}
	if err := ValidatePlan(req1); err != nil {
		t.Errorf("expected empty profile to be valid, got: %v", err)
	}

	// "auto" profile is allowed
	req2 := &PlanRequest{
		Subtasks: []SubtaskSpec{
			{Profile: "auto"},
		},
	}
	if err := ValidatePlan(req2); err != nil {
		t.Errorf("expected 'auto' profile to be valid, got: %v", err)
	}

	// Existing profile is valid
	req3 := &PlanRequest{
		Subtasks: []SubtaskSpec{
			{Profile: "dev1"},
		},
	}
	if err := ValidatePlan(req3); err != nil {
		t.Errorf("expected 'dev1' profile to be valid, got: %v", err)
	}

	// Non-existent profile fails
	req4 := &PlanRequest{
		Subtasks: []SubtaskSpec{
			{Profile: "nonexistent"},
		},
	}
	if err := ValidatePlan(req4); err == nil {
		t.Errorf("expected error for nonexistent profile")
	}
}

func TestExecutePlanEmptyDiffAuditAndOmitDiff(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev1")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	// 1. Test empty diff audit failure when Paths is specified
	req := PlanRequest{
		PlanID:   "plan-test-audit-fail-empty",
		RepoPath: repoDir,
		Workers:  1,
		Subtasks: []SubtaskSpec{
			{
				ID:      "subtask-no-op",
				Profile: "dev1",
				Command: "sh",
				Args:    []string{"-c", "echo 'doing nothing to paths'"},
				Paths:   []string{"expected_file.go"},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := taskMgr.ExecutePlan(ctx, req)
	if err != nil {
		t.Fatalf("ExecutePlan failed: %v", err)
	}

	if len(result.Subtasks) != 1 {
		t.Fatalf("expected 1 subtask, got %d", len(result.Subtasks))
	}
	stRes := result.Subtasks[0]
	if stRes.Status != StatusAuditFailed {
		t.Errorf("expected status 'audit_failed', got %q", stRes.Status)
	}
	if !strings.Contains(stRes.Error, "empty_diff: subtask exited without modifying any allowed paths") {
		t.Errorf("expected empty_diff error, got %q", stRes.Error)
	}
	if !strings.Contains(stRes.LogTail, "doing nothing to paths") {
		t.Errorf("expected LogTail to be populated on empty diff, got %q", stRes.LogTail)
	}

	// 2. Test OmitDiff clears diffs
	reqOmit := PlanRequest{
		PlanID:   "plan-test-omit-diff",
		RepoPath: repoDir,
		Workers:  1,
		OmitDiff: true,
		Subtasks: []SubtaskSpec{
			{
				ID:      "subtask-edit",
				Profile: "dev1",
				Command: "sh",
				Args:    []string{"-c", "echo 'hello' > omitted.txt"},
				Paths:   []string{"omitted.txt"},
			},
		},
	}

	resOmit, err := taskMgr.ExecutePlan(ctx, reqOmit)
	if err != nil {
		t.Fatalf("ExecutePlan with OmitDiff failed: %v", err)
	}
	if resOmit.UnifiedSummary.CombinedDiff != "" {
		t.Errorf("expected empty CombinedDiff when OmitDiff=true, got %q", resOmit.UnifiedSummary.CombinedDiff)
	}
	if len(resOmit.Subtasks) != 1 || resOmit.Subtasks[0].Diff != "" {
		t.Errorf("expected empty subtask Diff when OmitDiff=true, got %q", resOmit.Subtasks[0].Diff)
	}
}
