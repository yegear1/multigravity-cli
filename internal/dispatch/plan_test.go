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

	// Missing profile
	_, err = taskMgr.ExecutePlan(ctx, PlanRequest{
		RepoPath: repoDir,
		Subtasks: []SubtaskSpec{{Command: "echo 1"}},
	})
	if err == nil || !strings.Contains(err.Error(), "profile name is required") {
		t.Errorf("expected error for missing profile, got: %v", err)
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
