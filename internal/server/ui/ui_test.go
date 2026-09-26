package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/dispatch"
)

func TestRenderDiff(t *testing.T) {
	data := DiffPageData{
		Task: dispatch.Task{
			ID:         "task-123",
			Profile:    "dev",
			Status:     dispatch.StatusCompleted,
			Branch:     "feature-test",
			WorktreeID: "wt-456",
		},
		Structured: &dispatch.StructuredDiff{
			Summary: dispatch.DiffSummary{
				FilesChanged: 1,
				Additions:    2,
				Deletions:    1,
			},
			Files: []dispatch.DiffFile{
				{
					NewPath:   "foo.go",
					Status:    dispatch.DiffFileModified,
					Additions: 2,
					Deletions: 1,
					Hunks: []dispatch.DiffHunk{
						{
							Header:   "func test()",
							OldStart: 1,
							OldLines: 2,
							NewStart: 1,
							NewLines: 3,
							Lines: []dispatch.DiffLine{
								{Type: dispatch.DiffLineContext, Content: "package foo", OldLineNo: 1, NewLineNo: 1},
								{Type: dispatch.DiffLineDeletion, Content: "old", OldLineNo: 2},
								{Type: dispatch.DiffLineAddition, Content: "new1", NewLineNo: 2},
								{Type: dispatch.DiffLineAddition, Content: "new2", NewLineNo: 3},
							},
						},
					},
				},
			},
		},
		GeneratedAt: time.Now().UTC(),
	}

	var buf bytes.Buffer
	err := RenderDiff(&buf, data)
	if err != nil {
		t.Fatalf("failed to render diff template: %v", err)
	}

	html := buf.String()
	if !strings.Contains(html, "Diff: task-123") {
		t.Errorf("rendered html missing title: %s", html[:200])
	}
	if !strings.Contains(html, "foo.go") {
		t.Errorf("rendered html missing file path: %s", html)
	}
}

func TestRenderDashboard(t *testing.T) {
	data := DashboardPageData{
		Summary: &dispatch.TaskDashboardSummary{
			Total:     2,
			Running:   1,
			Completed: 1,
			RecentTasks: []dispatch.Task{
				{ID: "task-1", Profile: "dev", Status: dispatch.StatusRunning},
				{ID: "task-2", Profile: "test", Status: dispatch.StatusCompleted},
			},
		},
		GeneratedAt: time.Now().UTC(),
	}

	var buf bytes.Buffer
	err := RenderDashboard(&buf, data)
	if err != nil {
		t.Fatalf("failed to render dashboard template: %v", err)
	}

	html := buf.String()
	if !strings.Contains(html, "Task Execution Dashboard") {
		t.Errorf("rendered html missing title: %s", html[:200])
	}
	if !strings.Contains(html, "task-1") {
		t.Errorf("rendered html missing task-1: %s", html)
	}
}
