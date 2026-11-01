package dispatch

import (
	"os"
	"path/filepath"
	"testing"
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
- [ ] **[05.11]** Feature B [APPROVED] [PARALLEL: W1]
  - ` + "`Read:` `internal/cmd/dispatch.go:930-1045`" + `
  - ` + "`Paths:` `internal/dispatch/wave.go, internal/cmd/dispatch.go`" + `
  - ` + "`Verify:` `go test ./internal/dispatch -run TestParseTaskWave`" + `
- [ ] **[05.12]** Feature C [APPROVED] [SEQ: W2]
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
