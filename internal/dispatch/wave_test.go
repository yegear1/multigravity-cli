package dispatch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/agent"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func TestParseTaskWave(t *testing.T) {
	tempDir := t.TempDir()
	taskFilePath := filepath.Join(tempDir, "TASK.md")

	taskContent := `# Tasks

## Backlog

- [ ] **[05.10]** Feature A [APPROVED] [PARALLEL: W1]
  - ` + "`Read:` `internal/dispatch/plan.go:20-120`" + `
  - ` + "`Paths:` `internal/dispatch/plan.go, internal/dispatch/plan_test.go`" + `
  - ` + "`Verify:` `go test ./internal/dispatch -run TestPlan`" + `
  - ` + "`Model:` `gemini-3.8-flash-low`" + `
- [ ] **[05.11]** Feature   B ` + "`[APPROVED]` `[PARALLEL: W1]`" + `
  - ` + "`Read:` `internal/cmd/dispatch.go:930-1045`" + `
  - ` + "`Paths:` `internal/dispatch/wave.go, internal/cmd/dispatch.go`" + `
  - ` + "`Verify:` `go test ./internal/dispatch -run TestParseTaskWave`" + `
- [ ] **[05.12]** Feature C ` + "`[APPROVED]` `[SEQ: W2]`" + `
  - ` + "`Read:` `README.md`" + `
  - ` + "`Paths:` `README.md`" + `
- [x] **[05.09]** Completed Feature [APPROVED] [PARALLEL: W0]
  - ` + "`Paths:` `internal/dispatch/other.go`" + `
`

	if err := os.WriteFile(taskFilePath, []byte(taskContent), 0644); err != nil {
		t.Fatalf("failed to write task file: %v", err)
	}

	// 1. Auto-select first wave (W1)
	spec, err := ParseTaskWave(taskFilePath, "")
	if err != nil {
		t.Fatalf("ParseTaskWave failed for auto wave: %v", err)
	}
	if spec.WaveTag != "W1" {
		t.Errorf("expected WaveTag W1, got %s", spec.WaveTag)
	}
	if len(spec.Tasks) != 2 {
		t.Fatalf("expected 2 tasks in W1, got %d", len(spec.Tasks))
	}

	// Verify task 05.10
	t1 := spec.Tasks[0]
	if t1.ID != "05.10" {
		t.Errorf("expected ID 05.10, got %s", t1.ID)
	}
	if t1.Title != "Feature A" {
		t.Errorf("expected clean Title %q for 05.10, got %q", "Feature A", t1.Title)
	}
	if !t1.Parallel {
		t.Errorf("expected parallel true for 05.10")
	}
	if len(t1.Read) != 1 || t1.Read[0] != "internal/dispatch/plan.go:20-120" {
		t.Errorf("unexpected Read for 05.10: %v", t1.Read)
	}
	if len(t1.Paths) != 2 || t1.Paths[0] != "internal/dispatch/plan.go" || t1.Paths[1] != "internal/dispatch/plan_test.go" {
		t.Errorf("unexpected Paths for 05.10: %v", t1.Paths)
	}
	if t1.Verify != "go test ./internal/dispatch -run TestPlan" {
		t.Errorf("unexpected Verify for 05.10: %s", t1.Verify)
	}
	if t1.Model != "gemini-3.8-flash-low" {
		t.Errorf("unexpected Model for 05.10: %s", t1.Model)
	}

	// Verify task 05.11
	t2 := spec.Tasks[1]
	if t2.ID != "05.11" {
		t.Errorf("expected ID 05.11, got %s", t2.ID)
	}
	if t2.Title != "Feature B" {
		t.Errorf("expected clean Title %q for 05.11, got %q", "Feature B", t2.Title)
	}
	if len(t2.Paths) != 2 || t2.Paths[0] != "internal/dispatch/wave.go" || t2.Paths[1] != "internal/cmd/dispatch.go" {
		t.Errorf("unexpected Paths for 05.11: %v", t2.Paths)
	}

	// 2. Select explicit wave W2
	specW2, err := ParseTaskWave(taskFilePath, "W2")
	if err != nil {
		t.Fatalf("ParseTaskWave failed for W2: %v", err)
	}
	if specW2.WaveTag != "W2" {
		t.Errorf("expected WaveTag W2, got %s", specW2.WaveTag)
	}
	if len(specW2.Tasks) != 1 {
		t.Fatalf("expected 1 task in W2, got %d", len(specW2.Tasks))
	}
	if specW2.Tasks[0].Title != "Feature C" {
		t.Errorf("expected clean Title %q for 05.12, got %q", "Feature C", specW2.Tasks[0].Title)
	}
	if specW2.Tasks[0].Parallel {
		t.Errorf("expected parallel false for W2 task")
	}

	// 3. Detect overlapping parallel paths
	overlapTaskContent := `# Tasks
- [ ] **[05.20]** Task 1 [APPROVED] [PARALLEL: W_ERR]
  - ` + "`Paths:` `pkg/common/util.go, pkg/service/app.go`" + `
- [ ] **[05.21]** Task 2 [APPROVED] [PARALLEL: W_ERR]
  - ` + "`Paths:` `pkg/common/util.go`" + `
`
	overlapFile := filepath.Join(tempDir, "OVERLAP.md")
	if err := os.WriteFile(overlapFile, []byte(overlapTaskContent), 0644); err != nil {
		t.Fatalf("failed to write overlap file: %v", err)
	}

	_, err = ParseTaskWave(overlapFile, "W_ERR")
	if err == nil {
		t.Fatalf("expected error due to overlapping parallel paths, got nil")
	}
}

func TestExecuteTaskWaveCascadeRetry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping shell script mock test on windows")
	}

	repoDir := setupTestGitRepo(t)
	profNames := []string{"dev1", "dev2"}
	if candidates, err := quota.SelectEligibleProfilesForModel(100, "gemini-4-argon-medium", quota.WeightHeavy); err == nil {
		profNames = append(profNames, candidates...)
	}
	setupTestProfiles(t, profNames...)

	binDir := t.TempDir()
	fakeClaude := filepath.Join(binDir, "claude")
	script := `#!/bin/sh
case "$1 $2 $3 $4" in
  *"gemini-4-argon-high"*)
    echo "fixed" > feature.txt
    git add feature.txt
    ;;
  *)
    echo "broken" > feature.txt
    git add feature.txt
    ;;
esac
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake claude script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	agentDir := filepath.Join(repoDir, ".agent")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("failed to create .agent dir: %v", err)
	}
	taskFilePath := filepath.Join(agentDir, "TASK.md")
	taskContent := `# Tasks

## Backlog

- [ ] **[05.99]** Implement feature Z [APPROVED] [SEQ: W1]
  - ` + "`Paths:` `feature.txt`" + `
  - ` + "`Verify:` `sh -c 'grep -q \"fixed\" feature.txt || (echo \"verification failed: missing fixed token\"; exit 1)'`" + `
  - ` + "`Model:` `gemini-4-argon-medium`" + `
`
	if err := os.WriteFile(taskFilePath, []byte(taskContent), 0644); err != nil {
		t.Fatalf("failed to write TASK.md: %v", err)
	}

	addCmd := exec.Command("git", "add", ".agent/TASK.md")
	addCmd.Dir = repoDir
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v (%s)", err, string(out))
	}
	commitCmd := exec.Command("git", "commit", "-m", "docs: add TASK.md")
	commitCmd.Dir = repoDir
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v (%s)", err, string(out))
	}

	taskMgr := NewTaskManager(agent.NewManager())
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	res, err := taskMgr.ExecuteTaskWave(ctx, WaveOptions{
		RepoRoot: repoDir,
		TaskFile: taskFilePath,
		WaveTag:  "W1",
		Timeout:  15 * time.Second,
	})
	if err != nil {
		t.Fatalf("ExecuteTaskWave failed: %v", err)
	}

	if res.WaveStatus != "completed" {
		t.Fatalf("expected wave status 'completed', got %q (tasks: %+v)", res.WaveStatus, res.Tasks)
	}
	if len(res.Tasks) != 1 {
		t.Fatalf("expected 1 task result, got %d", len(res.Tasks))
	}

	stRes := res.Tasks[0]
	if stRes.Status != StatusCompleted {
		t.Errorf("expected task status 'completed', got %q (error: %s)", stRes.Status, stRes.Error)
	}
	if stRes.Model != "gemini-4-argon-high" {
		t.Errorf("expected escalated model 'gemini-4-argon-high', got %q", stRes.Model)
	}
	if !strings.Contains(stRes.Prompt, "[PREVIOUS ATTEMPT FAILED ON MODEL gemini-4-argon-medium]") {
		t.Errorf("expected retry prompt to contain failure model header, got:\n%s", stRes.Prompt)
	}
	if !strings.Contains(stRes.Prompt, "verification failed: missing fixed token") {
		t.Errorf("expected retry prompt to contain diagnostic verification output, got:\n%s", stRes.Prompt)
	}
}

func TestMarkTasksCompletedAndPromote(t *testing.T) {
	tempDir := t.TempDir()
	taskFilePath := filepath.Join(tempDir, "TASK.md")

	today := time.Now().Format("2006-01-02")

	initialContent := `# Task Queue

## Active Task

### [05.10] Wave 1 Execution
Status: RUNNING

### Acceptance Criteria
- [ ] First criterion passes
- [ ] Second criterion passes

## Backlog

- [ ] **[05.10]** Feature A [APPROVED] [PARALLEL: W1]
  - ` + "`Paths:` `internal/dispatch/a.go`" + `
- [ ] **[05.11]** Feature B [APPROVED] [PARALLEL: W1]
  - ` + "`Paths:` `internal/dispatch/b.go`" + `
- [ ] **[05.12]** Feature C [APPROVED] [SEQ: W2]
  - ` + "`Paths:` `internal/dispatch/c.go`" + `

## Completed Tasks Log

| ID | Task | Commit | Date |
| :--- | :--- | :--- | :--- |
| [05.09] | Prior Task | ` + "`0000000`" + ` | 2026-10-31 |
`

	if err := os.WriteFile(taskFilePath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write TASK.md: %v", err)
	}

	// Step 1: Complete wave W1 (05.10 and 05.11) -> promotes 05.12
	commitMapW1 := map[string]string{
		"05.10": "abc1234",
		"05.11": "def5678",
	}
	if err := markTasksCompletedInFile(taskFilePath, commitMapW1); err != nil {
		t.Fatalf("markTasksCompletedInFile W1 failed: %v", err)
	}

	updatedBytes, err := os.ReadFile(taskFilePath)
	if err != nil {
		t.Fatalf("failed to read updated TASK.md: %v", err)
	}
	updated := string(updatedBytes)

	// Verify backlog checkoff
	if !strings.Contains(updated, "- [x] **[05.10]** Feature A [APPROVED] [PARALLEL: W1] (abc1234)") {
		t.Errorf("expected 05.10 checked off with commit, got:\n%s", updated)
	}
	if !strings.Contains(updated, "- [x] **[05.11]** Feature B [APPROVED] [PARALLEL: W1] (def5678)") {
		t.Errorf("expected 05.11 checked off with commit, got:\n%s", updated)
	}

	// Verify Acceptance Criteria checkboxes marked [x]
	if !strings.Contains(updated, "- [x] First criterion passes") || !strings.Contains(updated, "- [x] Second criterion passes") {
		t.Errorf("expected Acceptance Criteria checkboxes to be marked [x], got:\n%s", updated)
	}

	// Verify Completed Tasks Log table insertion (prepended before [05.09])
	expectedRow10 := "| [05.10] | Feature A | `abc1234` | " + today + " |"
	expectedRow11 := "| [05.11] | Feature B | `def5678` | " + today + " |"
	idx10 := strings.Index(updated, expectedRow10)
	idx11 := strings.Index(updated, expectedRow11)
	idx09 := strings.Index(updated, "| [05.09] |")
	if idx10 == -1 || idx11 == -1 {
		t.Errorf("expected completed rows in log table, got:\n%s", updated)
	}
	if idx09 != -1 && (idx10 > idx09 || idx11 > idx09) {
		t.Errorf("expected new log rows to be prepended before [05.09], got:\n%s", updated)
	}

	// Verify Active Task promoted to next uncompleted [APPROVED] task [05.12]
	if !strings.Contains(updated, "### [05.12] Feature C") {
		t.Errorf("expected Active Task promoted to [05.12] Feature C, got:\n%s", updated)
	}

	// Step 2: Complete remaining task 05.12 -> fallback to Queue Empty — Awaiting Next Ideation
	commitMapW2 := map[string]string{
		"05.12": "999aaaa",
	}
	if err := markTasksCompletedInFile(taskFilePath, commitMapW2); err != nil {
		t.Fatalf("markTasksCompletedInFile W2 failed: %v", err)
	}

	finalBytes, err := os.ReadFile(taskFilePath)
	if err != nil {
		t.Fatalf("failed to read final TASK.md: %v", err)
	}
	finalContent := string(finalBytes)

	if !strings.Contains(finalContent, "Queue Empty — Awaiting Next Ideation") {
		t.Errorf("expected Active Task fallback to 'Queue Empty — Awaiting Next Ideation', got:\n%s", finalContent)
	}
	if !strings.Contains(finalContent, "Status: READY FOR PLANNING") {
		t.Errorf("expected Status: READY FOR PLANNING when queue is empty, got:\n%s", finalContent)
	}
	expectedRow12 := "| [05.12] | Feature C | `999aaaa` | " + today + " |"
	if !strings.Contains(finalContent, expectedRow12) {
		t.Errorf("expected [05.12] row in Completed Tasks Log, got:\n%s", finalContent)
	}
}

func TestLintTaskWave(t *testing.T) {
	t.Run("nil or empty wave", func(t *testing.T) {
		if v := LintTaskWave(nil); len(v) != 0 {
			t.Errorf("expected 0 violations for nil wave, got %d", len(v))
		}
		if v := LintTaskWave(&WaveSpec{}); len(v) != 0 {
			t.Errorf("expected 0 violations for empty wave, got %d", len(v))
		}
	})

	t.Run("valid tasks pass lint", func(t *testing.T) {
		spec := &WaveSpec{
			WaveTag: "W1",
			Tasks: []WaveTaskSpec{
				{
					ID:     "05.18",
					Title:  "Valid TierMicro task",
					Model:  "gemini-3.8-flash-low",
					Paths:  []string{"internal/dispatch/wave.go", "internal/dispatch/wave_test.go"},
					Read:   []string{"file1.go", "file2.go", "file3.go", "file4.go"},
					Verify: "go test ./...",
				},
				{
					ID:     "05.19",
					Title:  "Valid Standard tier task with multi-package paths",
					Model:  "gemini-3.8-flash-medium",
					Paths:  []string{"internal/dispatch/wave.go", "internal/cmd/dispatch.go", "pkg/util/util.go"},
					Read:   []string{"file1.go"},
					Verify: "go test ./...",
				},
			},
		}
		violations := LintTaskWave(spec)
		if len(violations) != 0 {
			t.Errorf("expected 0 violations, got %d: %v", len(violations), violations)
		}
	})

	t.Run("TierMicro task has > 2 paths", func(t *testing.T) {
		spec := &WaveSpec{
			WaveTag: "W1",
			Tasks: []WaveTaskSpec{
				{
					ID:     "05.20",
					Model:  "gemini-3.8-flash-low",
					Paths:  []string{"pkg/a/f1.go", "pkg/a/f2.go", "pkg/a/f3.go"},
					Verify: "go test ./...",
				},
			},
		}
		violations := LintTaskWave(spec)
		if len(violations) != 1 {
			t.Fatalf("expected 1 violation for >2 paths, got %d: %v", len(violations), violations)
		}
		if !strings.Contains(violations[0], "05.20") || !strings.Contains(violations[0], "Paths") {
			t.Errorf("unexpected violation message: %s", violations[0])
		}
	})

	t.Run("TierMicro task spans multiple packages", func(t *testing.T) {
		spec := &WaveSpec{
			WaveTag: "W1",
			Tasks: []WaveTaskSpec{
				{
					ID:     "05.21",
					Model:  "gemini-3.8-flash-low",
					Paths:  []string{"pkg/a/f1.go", "pkg/b/f2.go"},
					Verify: "go test ./...",
				},
			},
		}
		violations := LintTaskWave(spec)
		if len(violations) != 1 {
			t.Fatalf("expected 1 violation for multi-package, got %d: %v", len(violations), violations)
		}
		if !strings.Contains(violations[0], "05.21") || !strings.Contains(violations[0], "packages") {
			t.Errorf("unexpected violation message: %s", violations[0])
		}
	})

	t.Run("TierMicro classified via quota.TierForModel", func(t *testing.T) {
		spec := &WaveSpec{
			WaveTag: "W1",
			Tasks: []WaveTaskSpec{
				{
					ID:     "05.22",
					Model:  "gpt-4o-mini", // Classified as TierMicro by quota.TierForModel
					Paths:  []string{"pkg/a/f1.go", "pkg/b/f2.go"},
					Verify: "go test ./...",
				},
			},
		}
		violations := LintTaskWave(spec)
		if len(violations) != 1 {
			t.Fatalf("expected 1 violation for TierMicro via TierForModel, got %d: %v", len(violations), violations)
		}
	})

	t.Run("Task has > 4 read entries", func(t *testing.T) {
		spec := &WaveSpec{
			WaveTag: "W1",
			Tasks: []WaveTaskSpec{
				{
					ID:     "05.23",
					Model:  "gemini-3.8-flash-medium",
					Paths:  []string{"pkg/a/f1.go"},
					Read:   []string{"r1.go", "r2.go", "r3.go", "r4.go", "r5.go"},
					Verify: "go test ./...",
				},
			},
		}
		violations := LintTaskWave(spec)
		if len(violations) != 1 {
			t.Fatalf("expected 1 violation for >4 read entries, got %d: %v", len(violations), violations)
		}
		if !strings.Contains(violations[0], "05.23") || !strings.Contains(violations[0], "Read") {
			t.Errorf("unexpected violation message: %s", violations[0])
		}
	})

	t.Run("Task missing verify command", func(t *testing.T) {
		spec := &WaveSpec{
			WaveTag: "W1",
			Tasks: []WaveTaskSpec{
				{
					ID:     "05.24",
					Model:  "gemini-3.8-flash-medium",
					Paths:  []string{"pkg/a/f1.go"},
					Verify: "   ",
				},
			},
		}
		violations := LintTaskWave(spec)
		if len(violations) != 1 {
			t.Fatalf("expected 1 violation for missing verify, got %d: %v", len(violations), violations)
		}
		if !strings.Contains(violations[0], "05.24") || !strings.Contains(violations[0], "Verify") {
			t.Errorf("unexpected violation message: %s", violations[0])
		}
	})

	t.Run("Multiple violations reported across tasks", func(t *testing.T) {
		spec := &WaveSpec{
			WaveTag: "W1",
			Tasks: []WaveTaskSpec{
				{
					ID:     "05.25",
					Model:  "gemini-3.8-flash-low",
					Paths:  []string{"pkg/a/f1.go", "pkg/a/f2.go", "pkg/b/f3.go"}, // >2 paths AND multi-package
					Read:   []string{"r1", "r2", "r3", "r4", "r5"},               // >4 reads
					Verify: "",                                                    // missing verify
				},
			},
		}
		violations := LintTaskWave(spec)
		if len(violations) != 4 {
			t.Fatalf("expected 4 violations for combined bad task, got %d: %v", len(violations), violations)
		}
	})
}
