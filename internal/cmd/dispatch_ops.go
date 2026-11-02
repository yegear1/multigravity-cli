package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/ye-dev/multigravity-cli/internal/agent"
	"github.com/ye-dev/multigravity-cli/internal/dispatch"
)

var (
	dpTail        int
	dpFollow      bool
	dpStatOnly    bool
	dpStructured  bool
	dpWeb         bool
	dpForce       bool
	dpForceDelete bool
	dpRmWorktree  bool
	dpMaxAgeStr   string
	dpFilterProf  string
	dpFilterStat  string
	dpFilterType  string
	dpFilterWT    string
)

func newDispatchListCmd() *cobra.Command {
	listSubCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List active and past dispatched tasks",
		Args:    cobra.NoArgs,
		RunE:    runDispatchList,
	}
	listSubCmd.Flags().StringVar(&dpFilterProf, "profile", "", "Filter tasks by profile name")
	listSubCmd.Flags().StringVar(&dpFilterStat, "status", "", "Filter tasks by status (running, completed, failed, cancelled, audit_failed)")
	listSubCmd.Flags().StringVar(&dpFilterType, "agent", "", "Filter tasks by agent type")
	listSubCmd.Flags().StringVar(&dpFilterWT, "worktree", "", "Filter tasks by worktree ID")
	return listSubCmd
}

func newDispatchStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "status <task-id>",
		Short:             "Display details and telemetry of a dispatched task",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: taskArgsCompletion,
		RunE:              runDispatchStatus,
	}
}

func newDispatchLogsCmd() *cobra.Command {
	logsSubCmd := &cobra.Command{
		Use:               "logs <task-id>",
		Short:             "View or stream execution logs of a dispatched task",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: taskArgsCompletion,
		RunE:              runDispatchLogs,
	}
	logsSubCmd.Flags().IntVarP(&dpTail, "tail", "n", 100, "Number of lines/bytes to view from the end")
	logsSubCmd.Flags().BoolVarP(&dpFollow, "follow", "f", false, "Stream output continuously")
	return logsSubCmd
}

func newDispatchDiffCmd() *cobra.Command {
	diffSubCmd := &cobra.Command{
		Use:               "diff <task-id>",
		Short:             "Inspect git diff produced by a task in its ephemeral worktree",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: taskArgsCompletion,
		RunE:              runDispatchDiff,
	}
	diffSubCmd.Flags().BoolVar(&dpStatOnly, "stat", false, "Display diffstat summary only")
	diffSubCmd.Flags().BoolVar(&dpStructured, "structured", false, "Display structured file-by-file changes and stats")
	diffSubCmd.Flags().BoolVarP(&dpWeb, "web", "w", false, "Display URL or open in visual web diff viewer")
	return diffSubCmd
}

func newDispatchDashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "dashboard",
		Aliases: []string{"dash"},
		Short:   "Display aggregated task execution statistics and status dashboard",
		Args:    cobra.NoArgs,
		RunE:    runDispatchDashboard,
	}
}

func newDispatchCancelCmd() *cobra.Command {
	cancelSubCmd := &cobra.Command{
		Use:               "cancel <task-id>",
		Aliases:           []string{"stop"},
		Short:             "Cancel a running dispatched task",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: taskArgsCompletion,
		RunE:              runDispatchCancel,
	}
	cancelSubCmd.Flags().BoolVarP(&dpForce, "force", "f", false, "Force kill task process immediately")
	return cancelSubCmd
}

func newDispatchDeleteCmd() *cobra.Command {
	deleteSubCmd := &cobra.Command{
		Use:               "delete <task-id>",
		Aliases:           []string{"rm"},
		Short:             "Delete a completed or cancelled task record and its logs",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: taskArgsCompletion,
		RunE:              runDispatchDelete,
	}
	deleteSubCmd.Flags().BoolVarP(&dpRmWorktree, "worktree", "w", false, "Also remove the associated ephemeral git worktree and branch")
	deleteSubCmd.Flags().BoolVarP(&dpForceDelete, "force", "f", false, "Force delete task and remove associated ephemeral git worktree")
	return deleteSubCmd
}

func newDispatchPruneCmd() *cobra.Command {
	pruneSubCmd := &cobra.Command{
		Use:   "prune",
		Short: "Prune old finished tasks older than max age",
		Args:  cobra.NoArgs,
		RunE:  runDispatchPrune,
	}
	pruneSubCmd.Flags().StringVar(&dpMaxAgeStr, "max-age", "24h", "Maximum age of completed/cancelled tasks to preserve (e.g. 24h, 7d)")
	return pruneSubCmd
}

func runDispatchList(cmd *cobra.Command, args []string) error {
	defer func() {
		dpFilterProf = ""
		dpFilterStat = ""
		dpFilterType = ""
		dpFilterWT = ""
		dpJSON = false
	}()

	filter := dispatch.TaskFilter{
		Profile:    dpFilterProf,
		Status:     dispatch.TaskStatus(dpFilterStat),
		AgentType:  dpFilterType,
		WorktreeID: dpFilterWT,
	}

	mgr := dispatch.GetDefaultTaskManager()
	tasks, err := mgr.ListTasks("", filter)
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	out := cmd.OutOrStdout()

	if dpJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(tasks)
	}

	if len(tasks) == 0 {
		fmt.Fprintln(out, "No dispatched tasks found.")
		return nil
	}

	fmt.Fprintf(out, "%-24s %-12s %-12s %-10s %-20s %-10s %-18s\n",
		"TASK ID", "PROFILE", "STATUS", "AGENT", "WORKTREE", "DURATION", "CREATED")
	fmt.Fprintln(out, strings.Repeat("─", 100))

	for _, t := range tasks {
		statusStr := string(t.Status)
		switch t.Status {
		case dispatch.StatusRunning:
			statusStr = dpYellow("● running")
		case dispatch.StatusCompleted:
			statusStr = dpGreen("✓ completed")
		case dispatch.StatusFailed:
			statusStr = dpRed("✗ failed")
		case dispatch.StatusCancelled:
			statusStr = dpDim("! cancelled")
		case dispatch.StatusAuditFailed:
			statusStr = dpRed("✗ audit_failed")
		}

		durStr := "-"
		if t.DurationSeconds > 0 {
			durStr = fmt.Sprintf("%.1fs", t.DurationSeconds)
		} else if t.Status == dispatch.StatusRunning {
			durStr = fmt.Sprintf("%.0fs", time.Since(t.StartedAt).Seconds())
		}

		wtStr := "-"
		if t.WorktreeID != "" {
			wtStr = t.WorktreeID
		}

		createdStr := t.CreatedAt.Local().Format("2006-01-02 15:04:05")

		fmt.Fprintf(out, "%-24s %-12s %-20s %-10s %-20s %-10s %-18s\n",
			t.ID, t.Profile, statusStr, t.AgentType, wtStr, durStr, createdStr)
	}

	return nil
}

func runDispatchStatus(cmd *cobra.Command, args []string) error {
	defer func() {
		dpJSON = false
	}()

	taskID := args[0]
	mgr := dispatch.GetDefaultTaskManager()
	task, err := mgr.GetTask("", taskID)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	if dpJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(task)
	}

	fmt.Fprintf(out, "%s %s\n", dpBold("Dispatched Task:"), dpBold(task.ID))
	fmt.Fprintf(out, "  Status:     %s\n", task.Status)
	fmt.Fprintf(out, "  Profile:    %s\n", dpCyan(task.Profile))
	fmt.Fprintf(out, "  Agent Type: %s\n", task.AgentType)
	fmt.Fprintf(out, "  Command:    %s %s\n", task.Command, strings.Join(task.Args, " "))
	if task.Governance {
		fmt.Fprintf(out, "  Governance: %s\n", dpGreen("injected"))
	} else if task.Metadata != nil && task.Metadata["governance"] == "disabled" {
		fmt.Fprintf(out, "  Governance: %s\n", dpDim("disabled"))
	}
	if task.TraceID != "" {
		fmt.Fprintf(out, "  Trace ID:   %s\n", dpCyan(task.TraceID))
	}
	if task.Prompt != "" {
		fmt.Fprintf(out, "  Prompt:     %s\n", task.Prompt)
	}
	if task.WorktreeID != "" {
		fmt.Fprintf(out, "  Worktree:   %s\n", task.WorktreeID)
		fmt.Fprintf(out, "  Branch:     %s\n", task.Branch)
		fmt.Fprintf(out, "  Path:       %s\n", task.WorktreePath)
		if task.BaseCommit != "" {
			fmt.Fprintf(out, "  Base Commit:%s\n", task.BaseCommit)
		}
		if task.HeadCommit != "" {
			fmt.Fprintf(out, "  Head Commit:%s\n", task.HeadCommit)
		}
	}
	fmt.Fprintf(out, "  Session ID: %s (PID: %d)\n", task.SessionID, task.PID)
	fmt.Fprintf(out, "  Exit Code:  %d\n", task.ExitCode)
	if task.Verify != "" {
		fmt.Fprintf(out, "  Verify Cmd: %s\n", task.Verify)
		if task.VerificationExitCode != 0 {
			fmt.Fprintf(out, "  Verify Exit: %d\n", task.VerificationExitCode)
		}
		if task.VerificationOutput != "" {
			fmt.Fprintf(out, "  Verify Out: %s\n", strings.TrimSpace(task.VerificationOutput))
		}
	}
	if task.Retry > 0 {
		fmt.Fprintf(out, "  Retry:      %d (used: %d)\n", task.Retry, task.RetryCount)
	}
	fmt.Fprintf(out, "  Created At: %s\n", task.CreatedAt.Local().Format(time.RFC3339))
	if task.EndedAt != nil {
		fmt.Fprintf(out, "  Ended At:   %s\n", task.EndedAt.Local().Format(time.RFC3339))
		fmt.Fprintf(out, "  Duration:   %.2fs\n", task.DurationSeconds)
	}
	if task.LogPath != "" {
		fmt.Fprintf(out, "  Log File:   %s\n", task.LogPath)
	}
	if task.Error != "" {
		fmt.Fprintf(out, "  Error:      %s\n", dpRed(task.Error))
	}

	return nil
}

func runDispatchLogs(cmd *cobra.Command, args []string) error {
	defer func() {
		dpTail = 100
		dpFollow = false
		dpJSON = false
	}()

	taskID := args[0]
	mgr := dispatch.GetDefaultTaskManager()
	task, err := mgr.GetTask("", taskID)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	if dpFollow && task.Status == dispatch.StatusRunning && task.SessionID != "" {
		// Output initial tail
		if logBytes, err := mgr.GetTaskLogs("", taskID, dpTail*80); err == nil && len(logBytes) > 0 {
			out.Write(logBytes)
		}

		agentMgr := agent.GetDefaultManager()
		ch, unsub, err := agentMgr.SubscribeSession(task.SessionID)
		if err == nil {
			defer unsub()
			for chunk := range ch {
				out.Write([]byte(chunk.Data))
			}
		}
		return nil
	}

	logBytes, err := mgr.GetTaskLogs("", taskID, dpTail*80)
	if err != nil {
		return err
	}

	if dpJSON {
		res := map[string]interface{}{
			"id":   taskID,
			"logs": string(logBytes),
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	out.Write(logBytes)
	if len(logBytes) > 0 && logBytes[len(logBytes)-1] != '\n' {
		fmt.Fprintln(out)
	}
	return nil
}

func runDispatchDiff(cmd *cobra.Command, args []string) error {
	defer func() {
		dpStatOnly = false
		dpStructured = false
		dpWeb = false
		dpJSON = false
	}()

	taskID := args[0]
	mgr := dispatch.GetDefaultTaskManager()
	task, err := mgr.GetTask("", taskID)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	if dpWeb {
		port := 8989
		if envPort := os.Getenv("MULTIGRAVITY_PORT"); envPort != "" {
			if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
				port = p
			}
		}
		url := fmt.Sprintf("http://127.0.0.1:%d/ui/tasks/%s/diff", port, taskID)
		fmt.Fprintf(out, "Visual Diff Viewer: %s\n", url)
		return nil
	}

	if task.WorktreePath == "" {
		if dpJSON {
			res := map[string]interface{}{
				"id":          taskID,
				"worktree_id": "",
				"branch":      "",
				"base_commit": "",
				"diff":        "",
				"structured":  &dispatch.StructuredDiff{Files: []dispatch.DiffFile{}},
			}
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		}
		return fmt.Errorf("task %q does not have an ephemeral worktree", taskID)
	}

	diff, err := mgr.GetTaskDiff("", taskID, dpStatOnly)
	if err != nil {
		return fmt.Errorf("failed to compute git diff: %w", err)
	}

	if dpJSON {
		res := map[string]interface{}{
			"id":          taskID,
			"worktree_id": task.WorktreeID,
			"branch":      task.Branch,
			"base_commit": task.BaseCommit,
			"diff":        diff,
		}
		if dpStructured || dpJSON {
			res["structured"] = dispatch.ParseUnifiedDiff(diff)
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	if dpStructured {
		sd := dispatch.ParseUnifiedDiff(diff)
		fmt.Fprintf(out, "Diff Summary for %s:\n", dpBold(taskID))
		fmt.Fprintf(out, "Files Changed: %d | Additions: %s | Deletions: %s\n\n",
			sd.Summary.FilesChanged, dpGreen(fmt.Sprintf("+%d", sd.Summary.Additions)), dpRed(fmt.Sprintf("-%d", sd.Summary.Deletions)))
		for _, f := range sd.Files {
			p := f.NewPath
			if p == "" {
				p = f.OldPath
			}
			if f.Binary {
				fmt.Fprintf(out, "  [%s] %s (binary)\n", strings.ToUpper(string(f.Status)), p)
			} else {
				fmt.Fprintf(out, "  [%s] %s (+%d, -%d)\n", strings.ToUpper(string(f.Status)), p, f.Additions, f.Deletions)
			}
		}
		return nil
	}

	if strings.TrimSpace(diff) == "" {
		fmt.Fprintln(out, "No modifications detected in worktree.")
		return nil
	}

	fmt.Fprint(out, diff)
	if !strings.HasSuffix(diff, "\n") {
		fmt.Fprintln(out)
	}
	return nil
}

func runDispatchDashboard(cmd *cobra.Command, args []string) error {
	defer func() {
		dpJSON = false
	}()

	mgr := dispatch.GetDefaultTaskManager()
	summary, err := mgr.GetDashboardSummary("")
	if err != nil {
		return fmt.Errorf("failed to get dashboard summary: %w", err)
	}

	out := cmd.OutOrStdout()
	if dpJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}

	fmt.Fprintf(out, "%s\n", dpBold("Task Execution Dashboard"))
	if summary.AuditFailed > 0 {
		fmt.Fprintf(out, "Total: %d | Running: %s | Completed: %s | Failed: %s | Audit Failed: %s | Cancelled: %s\n\n",
			summary.Total,
			dpCyan(fmt.Sprintf("%d", summary.Running)),
			dpGreen(fmt.Sprintf("%d", summary.Completed)),
			dpRed(fmt.Sprintf("%d", summary.Failed)),
			dpRed(fmt.Sprintf("%d", summary.AuditFailed)),
			dpDim(fmt.Sprintf("%d", summary.Cancelled)),
		)
	} else {
		fmt.Fprintf(out, "Total: %d | Running: %s | Completed: %s | Failed: %s | Cancelled: %s\n\n",
			summary.Total,
			dpCyan(fmt.Sprintf("%d", summary.Running)),
			dpGreen(fmt.Sprintf("%d", summary.Completed)),
			dpRed(fmt.Sprintf("%d", summary.Failed)),
			dpDim(fmt.Sprintf("%d", summary.Cancelled)),
		)
	}

	if len(summary.RecentTasks) == 0 {
		fmt.Fprintln(out, "No dispatched tasks found.")
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", dpBold("TASK ID"), dpBold("PROFILE"), dpBold("STATUS"), dpBold("DURATION"), dpBold("COMMAND/PROMPT"))
	for _, t := range summary.RecentTasks {
		dur := "-"
		if t.DurationSeconds > 0 {
			dur = fmt.Sprintf("%.1fs", t.DurationSeconds)
		} else if t.Status == dispatch.StatusRunning {
			dur = "active"
		}
		cmdOrPrompt := t.Prompt
		if cmdOrPrompt == "" {
			cmdOrPrompt = t.Command
		}
		if len(cmdOrPrompt) > 40 {
			cmdOrPrompt = cmdOrPrompt[:37] + "..."
		}

		statusStr := string(t.Status)
		switch t.Status {
		case dispatch.StatusRunning:
			statusStr = dpCyan("● running")
		case dispatch.StatusCompleted:
			statusStr = dpGreen("✓ completed")
		case dispatch.StatusFailed:
			statusStr = dpRed("✗ failed")
		case dispatch.StatusCancelled:
			statusStr = dpDim("! cancelled")
		case dispatch.StatusAuditFailed:
			statusStr = dpRed("✗ audit_failed")
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", t.ID, t.Profile, statusStr, dur, cmdOrPrompt)
	}
	w.Flush()
	return nil
}

func runDispatchCancel(cmd *cobra.Command, args []string) error {
	defer func() {
		dpForce = false
		dpJSON = false
	}()

	taskID := args[0]
	mgr := dispatch.GetDefaultTaskManager()
	if err := mgr.CancelTask("", taskID, dpForce); err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	if dpJSON {
		res := map[string]interface{}{
			"id":     taskID,
			"status": "cancelled",
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Fprintf(out, "%s Task %s cancelled.\n", dpGreen("✓"), dpBold(taskID))
	return nil
}

func runDispatchDelete(cmd *cobra.Command, args []string) error {
	defer func() {
		dpRmWorktree = false
		dpForceDelete = false
		dpJSON = false
	}()

	taskID := args[0]
	mgr := dispatch.GetDefaultTaskManager()
	removeWorktree := dpRmWorktree || dpForceDelete
	if err := mgr.DeleteTask("", taskID, removeWorktree); err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	if dpJSON {
		res := map[string]interface{}{
			"id":      taskID,
			"deleted": true,
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Fprintf(out, "%s Task %s deleted.\n", dpGreen("✓"), dpBold(taskID))
	return nil
}

func runDispatchPrune(cmd *cobra.Command, args []string) error {
	defer func() {
		dpMaxAgeStr = "24h"
		dpJSON = false
	}()

	maxAge, err := time.ParseDuration(dpMaxAgeStr)
	if err != nil {
		return fmt.Errorf("invalid --max-age duration %q: %w", dpMaxAgeStr, err)
	}

	mgr := dispatch.GetDefaultTaskManager()
	pruned, err := mgr.PruneTasks("", maxAge)
	if err != nil {
		return fmt.Errorf("failed to prune tasks: %w", err)
	}

	out := cmd.OutOrStdout()

	if dpJSON {
		res := map[string]interface{}{
			"pruned":  pruned,
			"max_age": dpMaxAgeStr,
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Fprintf(out, "%s Pruned %d task(s) older than %s.\n", dpGreen("✓"), pruned, dpMaxAgeStr)
	return nil
}
