package ui

import (
	_ "embed"
	"html/template"
	"io"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/dispatch"
)

//go:embed assets/diff.html
var diffHTMLTemplate string

//go:embed assets/tasks.html
var tasksHTMLTemplate string

var (
	diffTmpl  = template.Must(template.New("diff").Parse(diffHTMLTemplate))
	tasksTmpl = template.Must(template.New("tasks").Parse(tasksHTMLTemplate))
)

// DiffPageData supplies context to render the diff.html template.
type DiffPageData struct {
	Task        dispatch.Task
	Structured  *dispatch.StructuredDiff
	GeneratedAt time.Time
}

// DashboardPageData supplies context to render the tasks.html template.
type DashboardPageData struct {
	Summary     *dispatch.TaskDashboardSummary
	GeneratedAt time.Time
}

// RenderDiff writes the rendered HTML diff view to w.
func RenderDiff(w io.Writer, data DiffPageData) error {
	return diffTmpl.Execute(w, data)
}

// RenderDashboard writes the rendered HTML tasks dashboard to w.
func RenderDashboard(w io.Writer, data DashboardPageData) error {
	return tasksTmpl.Execute(w, data)
}
