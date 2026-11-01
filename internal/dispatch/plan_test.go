package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/agent"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func TestSynthesizeCascadeRetryPrompt(t *testing.T) {
	base := "Implement feature X according to spec."

	// (a) both verifyOutput and logTail empty returns basePrompt unchanged
	gotA := SynthesizeCascadeRetryPrompt(base, "gemini-pro", "   ", "  \n\t ")
	if gotA != base {
		t.Fatalf("expected base prompt unchanged, got: %q", gotA)
	}

	// (b) verifyOutput non-empty includes [PREVIOUS ATTEMPT FAILED ON MODEL ...] and verifyOutput
	gotB := SynthesizeCascadeRetryPrompt(base, "gemini-pro", "syntax error on line 42", "tail logs that should be ignored")
	if !strings.Contains(gotB, "[PREVIOUS ATTEMPT FAILED ON MODEL gemini-pro]") {
		t.Errorf("expected model header in output, got: %q", gotB)
	}
	if !strings.Contains(gotB, "Diagnostic Output:\nsyntax error on line 42") {
		t.Errorf("expected verifyOutput in diagnostic output, got: %q", gotB)
	}
	if strings.Contains(gotB, "tail logs that should be ignored") {
		t.Errorf("expected logTail to be ignored when verifyOutput is present, got: %q", gotB)
	}

	// (c) verifyOutput empty and logTail non-empty includes logTail
	gotC := SynthesizeCascadeRetryPrompt(base, "claude-sonnet", "", "panic: runtime error\ngoroutine 1...")
	if !strings.Contains(gotC, "[PREVIOUS ATTEMPT FAILED ON MODEL claude-sonnet]") {
		t.Errorf("expected model header in output, got: %q", gotC)
	}
	if !strings.Contains(gotC, "Diagnostic Output:\npanic: runtime error\ngoroutine 1...") {
		t.Errorf("expected logTail in diagnostic output, got: %q", gotC)
	}

	// (d) empty prevModel defaults to unknown
	gotD := SynthesizeCascadeRetryPrompt(base, "   ", "compilation failed", "")
	if !strings.Contains(gotD, "[PREVIOUS ATTEMPT FAILED ON MODEL unknown]") {
		t.Errorf("expected unknown model header in output, got: %q", gotD)
	}
	if !strings.Contains(gotD, "Diagnostic Output:\ncompilation failed") {
		t.Errorf("expected diagnostic output, got: %q", gotD)
	}
}

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

	// Test Verify injection
	stVerify := SubtaskSpec{
		Prompt: "implement verify feature",
		Verify: "go test -v ./...",
	}
	promptVerify, _ := SynthesizeSubtaskPrompt(stVerify, "custom-model")
	if !strings.Contains(promptVerify, "Verify: go test -v ./...") {
		t.Errorf("expected prompt to contain 'Verify: go test -v ./...', got:\n%s", promptVerify)
	}

	stNoVerify := SubtaskSpec{
		Prompt: "no verify subtask",
	}
	promptNoVerify, _ := SynthesizeSubtaskPrompt(stNoVerify, "custom-model")
	if strings.Contains(promptNoVerify, "Verify:") {
		t.Errorf("expected prompt not to contain 'Verify:', got:\n%s", promptNoVerify)
	}
}

func TestSynthesizeSubtaskPromptSiblingTestInjection(t *testing.T) {
	tmpDir := t.TempDir()

	// Create pkg/alpha.go and pkg/alpha_test.go with 70 lines
	pkgDir := filepath.Join(tmpDir, "pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "alpha.go"), []byte("package pkg\nfunc Alpha() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var testLines []string
	for i := 1; i <= 70; i++ {
		testLines = append(testLines, fmt.Sprintf("// test line %02d", i))
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "alpha_test.go"), []byte(strings.Join(testLines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}

	// Case A: Sibling test automatically pre-injected when not in st.Read
	stA := SubtaskSpec{
		Prompt: "update alpha",
		Paths:  []string{"pkg/alpha.go"},
	}
	promptA, _ := SynthesizeSubtaskPromptWithRepo(stA, "", tmpDir, true)
	if !strings.Contains(promptA, "[PRE-LOADED READ CONTEXT]") {
		t.Fatalf("expected pre-loaded read context for sibling test, got:\n%s", promptA)
	}
	if !strings.Contains(promptA, "--- pkg/alpha_test.go ---") {
		t.Fatalf("expected sibling header '--- pkg/alpha_test.go ---', got:\n%s", promptA)
	}
	// Last 60 lines: line 11 to 70 should be present, line 01 to 10 should not be present
	if !strings.Contains(promptA, "// test line 70") || !strings.Contains(promptA, "// test line 11") {
		t.Errorf("expected trailing 60 lines (including line 11 and 70), got:\n%s", promptA)
	}
	if strings.Contains(promptA, "// test line 01") || strings.Contains(promptA, "// test line 10") {
		t.Errorf("expected lines 01-10 to be truncated to last 60 lines, got:\n%s", promptA)
	}

	// Case B: When sibling test is already in st.Read, do NOT duplicate it
	stB := SubtaskSpec{
		Prompt: "update alpha with explicit read",
		Read:   []string{"pkg/alpha_test.go"},
		Paths:  []string{"pkg/alpha.go"},
	}
	promptB, _ := SynthesizeSubtaskPromptWithRepo(stB, "", tmpDir, true)
	if c := strings.Count(promptB, "--- pkg/alpha_test.go ---"); c != 1 {
		t.Errorf("expected exactly 1 occurrence of sibling test when in st.Read, got %d", c)
	}

	// Case C: Budget limit (15 KB maxBudget)
	largeData := strings.Repeat("x", 15330)
	if err := os.WriteFile(filepath.Join(tmpDir, "large.txt"), []byte(largeData), 0644); err != nil {
		t.Fatal(err)
	}
	stC := SubtaskSpec{
		Prompt: "budget test",
		Read:   []string{"large.txt:1-1"},
		Paths:  []string{"pkg/alpha.go"},
	}
	promptC, _ := SynthesizeSubtaskPromptWithRepo(stC, "", tmpDir, true)
	idxContext := strings.Index(promptC, "[PRE-LOADED READ CONTEXT]")
	if idxContext == -1 {
		t.Fatalf("expected pre-loaded context in budget test")
	}
	injectedLen := len(promptC[idxContext+len("[PRE-LOADED READ CONTEXT]"):])
	if injectedLen > 15360 {
		t.Errorf("expected injected context to be within 15360 bytes, got %d", injectedLen)
	}
}

func TestSynthesizeSubtaskPromptWithRepoSizeBounds(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Non-ranged line limit (150 lines)
	var lines []string
	for i := 1; i <= 200; i++ {
		lines = append(lines, fmt.Sprintf("line %03d", i))
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "lines.txt"), []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}

	stLines := SubtaskSpec{
		Prompt: "test line bound",
		Read:   []string{"lines.txt"},
	}
	promptLines, _ := SynthesizeSubtaskPromptWithRepo(stLines, "", tmpDir, true)
	if !strings.Contains(promptLines, "line 150") {
		t.Errorf("expected line 150 to be present in prompt")
	}
	if strings.Contains(promptLines, "line 151") {
		t.Errorf("expected line 151 to be truncated by maxNonRangedLines bound")
	}

	// 2. Non-ranged byte limit (5 KB / 5120 bytes)
	var largeLines []string
	for i := 1; i <= 10; i++ {
		largeLines = append(largeLines, strings.Repeat(fmt.Sprintf("%d", i%10), 1000))
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "large_bytes.txt"), []byte(strings.Join(largeLines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}

	stBytes := SubtaskSpec{
		Prompt: "test byte bound",
		Read:   []string{"large_bytes.txt"},
	}
	promptBytes, _ := SynthesizeSubtaskPromptWithRepo(stBytes, "", tmpDir, true)
	header := "\n--- large_bytes.txt ---\n"
	idx := strings.Index(promptBytes, header)
	if idx == -1 {
		t.Fatalf("expected header %q in prompt", header)
	}
	contentInjected := strings.TrimSuffix(promptBytes[idx+len(header):], "\n")
	if len(contentInjected) > 5120 {
		t.Errorf("expected injected content for non-ranged read to be capped at 5120 bytes, got %d", len(contentInjected))
	}
	if len(contentInjected) != 5120 {
		t.Errorf("expected exactly 5120 bytes of capped content, got %d", len(contentInjected))
	}

	// 3. Ranged entry is not capped by non-ranged 150 lines bound
	stRanged := SubtaskSpec{
		Prompt: "test ranged entry",
		Read:   []string{"lines.txt:1-180"},
	}
	promptRanged, _ := SynthesizeSubtaskPromptWithRepo(stRanged, "", tmpDir, true)
	if !strings.Contains(promptRanged, "line 180") {
		t.Errorf("expected line 180 to be present for ranged read entry")
	}
}

func TestExecutePlanTokensUsed(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev1")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	req := PlanRequest{
		PlanID:   "plan-test-tokens-used",
		RepoPath: repoDir,
		Workers:  1,
		Subtasks: []SubtaskSpec{
			{
				ID:      "subtask-token-test",
				Profile: "dev1",
				Prompt:  "Please write greeting to file",
				Command: "sh",
				Args:    []string{"-c", "echo 'hello world token' > token.txt"},
				Paths:   []string{"token.txt"},
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
	if stRes.TokensUsed <= 0 {
		t.Errorf("expected TokensUsed > 0, got %d", stRes.TokensUsed)
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

func TestClassifySubtaskFailure(t *testing.T) {
	tests := []struct {
		name string
		res  SubtaskResult
		want string
	}{
		{
			name: "completed success returns empty",
			res: SubtaskResult{
				Status:   StatusCompleted,
				ExitCode: 0,
			},
			want: "",
		},
		{
			name: "rate limited via 429 in log tail",
			res: SubtaskResult{
				Status:   StatusFailed,
				ExitCode: 1,
				LogTail:  "API request failed: HTTP 429 Too Many Requests",
			},
			want: "rate_limited",
		},
		{
			name: "rate limited via quota in error",
			res: SubtaskResult{
				Status:   StatusFailed,
				ExitCode: 1,
				Error:    "RESOURCE_EXHAUSTED: quota exceeded for model",
			},
			want: "rate_limited",
		},
		{
			name: "rate limited via rate-limit indicator",
			res: SubtaskResult{
				Status:   StatusFailed,
				ExitCode: 1,
				LogTail:  "Error: rate-limit reached, please retry later",
			},
			want: "rate_limited",
		},
		{
			name: "timeout via context deadline exceeded",
			res: SubtaskResult{
				Status:   StatusCancelled,
				ExitCode: -1,
				Error:    "context deadline exceeded",
			},
			want: "timeout",
		},
		{
			name: "timeout via cancelled status",
			res: SubtaskResult{
				Status:   StatusCancelled,
				ExitCode: -1,
			},
			want: "timeout",
		},
		{
			name: "empty diff via audit error",
			res: SubtaskResult{
				Status:   StatusAuditFailed,
				ExitCode: 0,
				Error:    "empty_diff: subtask exited without modifying any allowed paths",
			},
			want: "empty_diff",
		},
		{
			name: "verify failed via non-zero verification exit code",
			res: SubtaskResult{
				Status:               StatusAuditFailed,
				ExitCode:             0,
				VerificationExitCode: 1,
				VerificationOutput:   "FAIL\tgithub.com/ye-dev/multigravity-cli/internal/dispatch",
				Diff:                 "diff --git a/foo.go b/foo.go",
			},
			want: "verify_failed",
		},
		{
			name: "agent error via non-zero exit code",
			res: SubtaskResult{
				Status:   StatusFailed,
				ExitCode: 2,
				Error:    "exit status 2",
				LogTail:  "fatal: unexpected error in session",
			},
			want: "agent_error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifySubtaskFailure(tc.res)
			if got != tc.want {
				t.Errorf("ClassifySubtaskFailure() = %q, want %q", got, tc.want)
			}
		})
	}

	// Verify ExecutePlan populates FailureReason on non-succeeded subtasks and leaves it empty on success
	repoDir := setupTestGitRepo(t)
	setupTestProfiles(t, "dev1")
	taskMgr := NewTaskManager(agent.NewManager())

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	planRes, err := taskMgr.ExecutePlan(ctx, PlanRequest{
		PlanID:   "plan-failure-reasons",
		RepoPath: repoDir,
		Workers:  2,
		Subtasks: []SubtaskSpec{
			{
				ID:      "st-ok",
				Profile: "dev1",
				Command: "sh",
				Args:    []string{"-c", "echo 'ok' > ok_reason.txt"},
			},
			{
				ID:      "st-empty-diff",
				Profile: "dev1",
				Command: "sh",
				Args:    []string{"-c", "echo 'noop'"},
				Paths:   []string{"missing_reason.txt"},
			},
			{
				ID:      "st-verify-fail",
				Profile: "dev1",
				Command: "sh",
				Args:    []string{"-c", "echo 'v' > v_reason.txt"},
				Verify:  "sh -c 'echo verify-err && exit 1'",
			},
			{
				ID:      "st-rate-limit",
				Profile: "dev1",
				Command: "sh",
				Args:    []string{"-c", "echo 'Error 429: rate limit exceeded' && exit 1"},
			},
			{
				ID:      "st-agent-err",
				Profile: "dev1",
				Command: "sh",
				Args:    []string{"-c", "echo 'crash' && exit 3"},
			},
		},
	})
	if err != nil {
		t.Fatalf("ExecutePlan failed: %v", err)
	}

	expectedReasons := map[string]string{
		"st-ok":          "",
		"st-empty-diff":  "empty_diff",
		"st-verify-fail": "verify_failed",
		"st-rate-limit":  "rate_limited",
		"st-agent-err":   "agent_error",
	}
	for _, st := range planRes.Subtasks {
		want, ok := expectedReasons[st.ID]
		if !ok {
			t.Fatalf("unexpected subtask ID %q", st.ID)
		}
		if st.FailureReason != want {
			t.Errorf("subtask %q: FailureReason = %q, want %q (status=%s, err=%q)", st.ID, st.FailureReason, want, st.Status, st.Error)
		}
	}
}

func TestPlanResultToCompact(t *testing.T) {
	planRes := &PlanResult{
		PlanID: "plan-compact-test",
		Subtasks: []SubtaskResult{
			{
				ID:     "st-1",
				Prompt: "do task 1 with very long prompt...",
				Diff:   "diff --git a/file1.go b/file1.go\n...",
			},
			{
				ID:     "st-2",
				Prompt: "do task 2",
				Diff:   "diff --git a/file2.go b/file2.go\n...",
			},
		},
		UnifiedSummary: UnifiedDiffSummary{
			TotalFilesChanged: 2,
			CombinedDiff:      "combined diff content...",
		},
	}

	compact := planRes.ToCompact()
	if compact == nil {
		t.Fatal("expected non-nil compact plan result")
	}

	// Verify original is not modified
	if planRes.Subtasks[0].Prompt == "" || planRes.Subtasks[0].Diff == "" || planRes.UnifiedSummary.CombinedDiff == "" {
		t.Errorf("original plan result should not be mutated")
	}

	// Verify compact clears prompts and diffs
	if compact.PlanID != "plan-compact-test" {
		t.Errorf("expected plan ID to be preserved, got %q", compact.PlanID)
	}
	if compact.UnifiedSummary.CombinedDiff != "" {
		t.Errorf("expected CombinedDiff to be cleared, got %q", compact.UnifiedSummary.CombinedDiff)
	}
	for i, st := range compact.Subtasks {
		if st.Prompt != "" {
			t.Errorf("subtask %d: expected empty Prompt, got %q", i, st.Prompt)
		}
		if st.Diff != "" {
			t.Errorf("subtask %d: expected empty Diff, got %q", i, st.Diff)
		}
	}
}

func TestWaveExecutionResultToCompact(t *testing.T) {
	waveRes := &WaveExecutionResult{
		WaveStatus: "completed",
		WaveTag:    "W1",
		Tasks: []SubtaskResult{
			{
				ID:     "task-1",
				Prompt: "prompt 1",
				Diff:   "diff 1",
			},
		},
	}

	compact := waveRes.ToCompact()
	if compact == nil {
		t.Fatal("expected non-nil compact result")
	}
	if waveRes.Tasks[0].Prompt == "" || waveRes.Tasks[0].Diff == "" {
		t.Errorf("original wave result should not be mutated")
	}
	if compact.Tasks[0].Prompt != "" || compact.Tasks[0].Diff != "" {
		t.Errorf("compact wave result should have empty prompt and diff")
	}
}

func TestExecutePlanPersistsRunArtifact(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfiles(t, "dev1")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	planID := "plan-run-persist-test"
	req := PlanRequest{
		PlanID:   planID,
		RepoPath: repoDir,
		Workers:  1,
		Subtasks: []SubtaskSpec{
			{
				ID:      "st-1",
				Profile: "dev1",
				Prompt:  "echo test run artifact",
				Command: "sh",
				Args:    []string{"-c", "echo 'hello' > run_test.txt && git add -A"},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := taskMgr.ExecutePlan(ctx, req)
	if err != nil {
		t.Fatalf("ExecutePlan failed: %v", err)
	}

	runFile := filepath.Join(repoDir, ".multigravity", "runs", planID+".json")
	data, err := os.ReadFile(runFile)
	if err != nil {
		t.Fatalf("failed to read persisted run artifact at %s: %v", runFile, err)
	}

	var persisted PlanResult
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("failed to unmarshal persisted run artifact: %v", err)
	}

	if persisted.PlanID != planID {
		t.Errorf("expected persisted plan ID %q, got %q", planID, persisted.PlanID)
	}
	if len(persisted.Subtasks) != 1 {
		t.Fatalf("expected 1 subtask, got %d", len(persisted.Subtasks))
	}
	// Verify run artifact is complete and uncompacted (has prompt and diff)
	if persisted.Subtasks[0].Prompt == "" {
		t.Errorf("expected persisted subtask to retain prompt")
	}
	if persisted.Subtasks[0].Diff == "" {
		t.Errorf("expected persisted subtask to retain diff")
	}
	if persisted.UnifiedSummary.CombinedDiff == "" {
		t.Errorf("expected persisted unified summary to retain CombinedDiff")
	}
	_ = result
}

func TestExecutePlanVerifyOnTimeout(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	setupTestProfile(t, "dev1")

	agentMgr := agent.NewManager()
	taskMgr := NewTaskManager(agentMgr)

	t.Run("recovers as completed when verify succeeds", func(t *testing.T) {
		req := PlanRequest{
			PlanID:   "plan-verify-timeout-success",
			RepoPath: repoDir,
			Workers:  1,
			Timeout:  "500ms",
			Subtasks: []SubtaskSpec{
				{
					ID:      "subtask-slow-success",
					Profile: "dev1",
					Command: "sh",
					Args:    []string{"-c", "echo 'recovered content' > recovered.txt && sleep 10"},
					Verify:  "cat recovered.txt",
				},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result, err := taskMgr.ExecutePlan(ctx, req)
		if err != nil {
			t.Fatalf("ExecutePlan failed: %v", err)
		}

		if result.Status != "completed" {
			t.Errorf("expected plan status 'completed', got %q", result.Status)
		}
		if len(result.Subtasks) != 1 {
			t.Fatalf("expected 1 subtask result, got %d", len(result.Subtasks))
		}

		stRes := result.Subtasks[0]
		if stRes.Status != StatusCompleted {
			t.Errorf("expected subtask status 'completed', got %q (error: %s)", stRes.Status, stRes.Error)
		}
		if stRes.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", stRes.ExitCode)
		}
		if stRes.VerificationExitCode != 0 {
			t.Errorf("expected verification exit code 0, got %d", stRes.VerificationExitCode)
		}
		if !strings.Contains(stRes.VerificationOutput, "recovered content") {
			t.Errorf("expected verification output to contain 'recovered content', got %q", stRes.VerificationOutput)
		}
		if !strings.Contains(stRes.Diff, "recovered.txt") {
			t.Errorf("expected diff to contain 'recovered.txt', got:\n%s", stRes.Diff)
		}
	})

	t.Run("audit fails when verify fails on timeout", func(t *testing.T) {
		req := PlanRequest{
			PlanID:   "plan-verify-timeout-failure",
			RepoPath: repoDir,
			Workers:  1,
			Timeout:  "500ms",
			Subtasks: []SubtaskSpec{
				{
					ID:      "subtask-slow-fail",
					Profile: "dev1",
					Command: "sh",
					Args:    []string{"-c", "echo 'partial content' > unverified.txt && sleep 10"},
					Verify:  "sh -c 'echo \"test failed\" >&2 && exit 1'",
				},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
		if stRes.Status != StatusAuditFailed {
			t.Errorf("expected subtask status 'audit_failed', got %q", stRes.Status)
		}
		if stRes.VerificationExitCode != 1 {
			t.Errorf("expected verification exit code 1, got %d", stRes.VerificationExitCode)
		}
	})
}

func TestProfileAwareConcurrency(t *testing.T) {
	t.Run("countActiveEligibleProfiles with explicit profiles", func(t *testing.T) {
		subtasks := []SubtaskSpec{
			{ID: "st1", Profile: "p1"},
			{ID: "st2", Profile: "p2"},
			{ID: "st3", Profile: "p1"},
		}
		count := countActiveEligibleProfiles(subtasks, "")
		if count != 2 {
			t.Errorf("expected 2 active profiles, got %d", count)
		}
	})

	t.Run("countActiveEligibleProfiles excludes profiles on cooldown", func(t *testing.T) {
		home := setupTestProfiles(t, "healthy-p1", "cooldown-p2")
		_ = home
		quota.RecordProfileCooldown("cooldown-p2", 10*time.Minute)
		defer quota.RecordProfileCooldown("cooldown-p2", -1*time.Minute)

		subtasks := []SubtaskSpec{
			{ID: "st1", Profile: "healthy-p1"},
			{ID: "st2", Profile: "cooldown-p2"},
		}
		count := countActiveEligibleProfiles(subtasks, "")
		if count != 1 {
			t.Errorf("expected 1 active profile, got %d", count)
		}
	})

	t.Run("countActiveEligibleProfiles resolves auto with available profiles", func(t *testing.T) {
		setupTestProfiles(t, "auto-prof1", "auto-prof2")
		subtasks := []SubtaskSpec{
			{ID: "st1", Profile: "auto"},
			{ID: "st2", Profile: "auto"},
		}
		count := countActiveEligibleProfiles(subtasks, "")
		if count < 2 {
			t.Errorf("expected at least 2 active profiles for auto, got %d", count)
		}
	})

	t.Run("ExecutePlan clamps workers to active eligible profile count", func(t *testing.T) {
		repoDir := setupTestGitRepo(t)
		setupTestProfiles(t, "single-prof")

		agentMgr := agent.NewManager()
		taskMgr := NewTaskManager(agentMgr)

		req := PlanRequest{
			PlanID:   "plan-clamp-workers",
			RepoPath: repoDir,
			Workers:  8, // Requested 8 workers, but only 1 profile exists
			Subtasks: []SubtaskSpec{
				{
					ID:      "subtask-1",
					Profile: "auto",
					Command: "sh",
					Args:    []string{"-c", "echo 'st1' > st1.txt && git add -A"},
				},
				{
					ID:      "subtask-2",
					Profile: "auto",
					Command: "sh",
					Args:    []string{"-c", "echo 'st2' > st2.txt && git add -A"},
				},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		result, err := taskMgr.ExecutePlan(ctx, req)
		if err != nil {
			t.Fatalf("ExecutePlan failed: %v", err)
		}
		if result.Status != "completed" {
			t.Errorf("expected status completed, got %q", result.Status)
		}
		if len(result.Subtasks) != 2 {
			t.Fatalf("expected 2 subtask results, got %d", len(result.Subtasks))
		}
		for _, st := range result.Subtasks {
			if st.Profile != "single-prof" {
				t.Errorf("expected subtask profile 'single-prof', got %q", st.Profile)
			}
		}
	})
}
