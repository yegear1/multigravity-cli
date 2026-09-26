package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ye-dev/multigravity-cli/internal/agent"
	"github.com/ye-dev/multigravity-cli/internal/dispatch"
)

var (
	dpPrompt      string
	dpAgentType   string
	dpNewWorktree bool
	dpWorktree    string
	dpBranch      string
	dpBaseCommit  string
	dpCwd         string
	dpGateway     string
	dpGatewayKey  string
	dpDetach      bool
	dpJSON        bool
	dpTail        int
	dpFollow      bool
	dpStatOnly    bool
	dpStructured  bool
	dpWeb         bool
	dpForce       bool
	dpRmWorktree  bool
	dpMaxAgeStr   string
	dpFilterProf  string
	dpFilterStat  string
	dpFilterType  string
	dpFilterWT    string

	dpGreen  = color.New(color.FgGreen).SprintFunc()
	dpYellow = color.New(color.FgYellow).SprintFunc()
	dpRed    = color.New(color.FgRed).SprintFunc()
	dpCyan   = color.New(color.FgCyan).SprintFunc()
	dpBold   = color.New(color.Bold).SprintFunc()
	dpDim    = color.New(color.Faint).SprintFunc()
)

func newDispatchCmd() *cobra.Command {
	dispatchCmd := &cobra.Command{
		Use:     "dispatch",
		Aliases: []string{"dp", "task"},
		Short:   "Orchestrate AI coding agent tasks with profile isolation and ephemeral worktrees",
		Long: `Dispatch and supervise autonomous coding agent tasks (Claude Code, Aider, OpenCode, Agy)
with profile-isolated credentials, ephemeral git worktrees, and persistent log capture.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDispatchList(cmd, args)
		},
	}

	dispatchCmd.PersistentFlags().BoolVar(&dpJSON, "json", false, "Output results in machine-readable JSON format")

	// Run Subcommand
	runSubCmd := &cobra.Command{
		Use:   "run <profile> [flags] [--] [command] [args...]",
		Short: "Dispatch an agent task with isolated profile credentials and worktree",
		Args:  cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return profileArgsCompletion(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: runDispatchRun,
	}
	runSubCmd.Flags().StringVarP(&dpPrompt, "prompt", "p", "", "Instruction prompt for the agent")
	runSubCmd.Flags().StringVar(&dpAgentType, "agent", "", "Agent type (claude, aider, opencode, agy, custom)")
	runSubCmd.Flags().BoolVarP(&dpNewWorktree, "new-worktree", "w", false, "Create a dedicated ephemeral git worktree for the task")
	runSubCmd.Flags().StringVar(&dpWorktree, "worktree", "", "Associate with an existing worktree ID")
	runSubCmd.Flags().StringVar(&dpBranch, "branch", "", "Git branch name for newly created worktree")
	runSubCmd.Flags().StringVar(&dpBaseCommit, "base", "", "Base commit or branch to branch off")
	runSubCmd.Flags().StringVar(&dpCwd, "cwd", "", "Custom working directory")
	runSubCmd.Flags().StringVar(&dpGateway, "gateway", "", "Gateway base URL (defaults to auto-detecting http://127.0.0.1:8080/v1)")
	runSubCmd.Flags().StringVar(&dpGatewayKey, "gateway-key", "", "Gateway authentication key")
	runSubCmd.Flags().BoolVarP(&dpDetach, "detach", "d", false, "Run task in background and print task ID immediately")

	// List Subcommand
	listSubCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List active and past dispatched tasks",
		Args:    cobra.NoArgs,
		RunE:    runDispatchList,
	}
	listSubCmd.Flags().StringVar(&dpFilterProf, "profile", "", "Filter tasks by profile name")
	listSubCmd.Flags().StringVar(&dpFilterStat, "status", "", "Filter tasks by status (running, completed, failed, cancelled)")
	listSubCmd.Flags().StringVar(&dpFilterType, "agent", "", "Filter tasks by agent type")
	listSubCmd.Flags().StringVar(&dpFilterWT, "worktree", "", "Filter tasks by worktree ID")

	// Status Subcommand
	statusSubCmd := &cobra.Command{
		Use:   "status <task-id>",
		Short: "Display details and telemetry of a dispatched task",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: taskArgsCompletion,
		RunE:  runDispatchStatus,
	}

	// Logs Subcommand
	logsSubCmd := &cobra.Command{
		Use:   "logs <task-id>",
		Short: "View or stream execution logs of a dispatched task",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: taskArgsCompletion,
		RunE:  runDispatchLogs,
	}
	logsSubCmd.Flags().IntVarP(&dpTail, "tail", "n", 100, "Number of lines/bytes to view from the end")
	logsSubCmd.Flags().BoolVarP(&dpFollow, "follow", "f", false, "Stream output continuously")

	// Diff Subcommand
	diffSubCmd := &cobra.Command{
		Use:   "diff <task-id>",
		Short: "Inspect git diff produced by a task in its ephemeral worktree",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: taskArgsCompletion,
		RunE:  runDispatchDiff,
	}
	diffSubCmd.Flags().BoolVar(&dpStatOnly, "stat", false, "Display diffstat summary only")
	diffSubCmd.Flags().BoolVar(&dpStructured, "structured", false, "Display structured file-by-file changes and stats")
	diffSubCmd.Flags().BoolVarP(&dpWeb, "web", "w", false, "Display URL or open in visual web diff viewer")

	// Dashboard Subcommand
	dashboardSubCmd := &cobra.Command{
		Use:     "dashboard",
		Aliases: []string{"dash"},
		Short:   "Display aggregated task execution statistics and status dashboard",
		Args:    cobra.NoArgs,
		RunE:    runDispatchDashboard,
	}

	// Cancel Subcommand
	cancelSubCmd := &cobra.Command{
		Use:     "cancel <task-id>",
		Aliases: []string{"stop"},
		Short:   "Cancel a running dispatched task",
		Args:    cobra.ExactArgs(1),
		ValidArgsFunction: taskArgsCompletion,
		RunE:    runDispatchCancel,
	}
	cancelSubCmd.Flags().BoolVarP(&dpForce, "force", "f", false, "Force kill task process immediately")

	// Delete Subcommand
	deleteSubCmd := &cobra.Command{
		Use:     "delete <task-id>",
		Aliases: []string{"rm"},
		Short:   "Delete a completed or cancelled task record and its logs",
		Args:    cobra.ExactArgs(1),
		ValidArgsFunction: taskArgsCompletion,
		RunE:    runDispatchDelete,
	}
	deleteSubCmd.Flags().BoolVarP(&dpRmWorktree, "worktree", "w", false, "Also remove the associated ephemeral git worktree and branch")

	// Prune Subcommand
	pruneSubCmd := &cobra.Command{
		Use:   "prune",
		Short: "Prune old finished tasks older than max age",
		Args:  cobra.NoArgs,
		RunE:  runDispatchPrune,
	}
	pruneSubCmd.Flags().StringVar(&dpMaxAgeStr, "max-age", "24h", "Maximum age of completed/cancelled tasks to preserve (e.g. 24h, 7d)")

	dispatchCmd.AddCommand(runSubCmd, listSubCmd, statusSubCmd, logsSubCmd, diffSubCmd, dashboardSubCmd, cancelSubCmd, deleteSubCmd, pruneSubCmd)
	return dispatchCmd
}

func taskArgsCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	mgr := dispatch.GetDefaultTaskManager()
	tasks, err := mgr.ListTasks("", dispatch.TaskFilter{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var matches []string
	for _, t := range tasks {
		if strings.HasPrefix(t.ID, toComplete) {
			desc := fmt.Sprintf("[%s] %s (%s)", t.Status, t.Profile, t.AgentType)
			matches = append(matches, fmt.Sprintf("%s\t%s", t.ID, desc))
		}
	}
	return matches, cobra.ShellCompDirectiveNoFileComp
}

func runDispatchRun(cmd *cobra.Command, args []string) error {
	defer func() {
		dpPrompt = ""
		dpAgentType = ""
		dpNewWorktree = false
		dpWorktree = ""
		dpBranch = ""
		dpBaseCommit = ""
		dpCwd = ""
		dpGateway = ""
		dpGatewayKey = ""
		dpDetach = false
		dpJSON = false
	}()

	profileName := args[0]
	var command string
	var cmdArgs []string

	if len(args) > 1 {
		command = args[1]
		if len(args) > 2 {
			cmdArgs = args[2:]
		}
	}

	gatewayURL := dpGateway
	if gatewayURL == "" {
		gatewayURL = probeDefaultGateway()
	}

	opts := dispatch.DispatchOptions{
		Profile:     profileName,
		Prompt:      dpPrompt,
		AgentType:   dpAgentType,
		Command:     command,
		Args:        cmdArgs,
		NewWorktree: dpNewWorktree,
		WorktreeID:  dpWorktree,
		Branch:      dpBranch,
		BaseCommit:  dpBaseCommit,
		Cwd:         dpCwd,
		GatewayURL:  gatewayURL,
		GatewayKey:  dpGatewayKey,
		Detached:    dpDetach,
	}

	mgr := dispatch.GetDefaultTaskManager()
	task, err := mgr.Dispatch(opts)
	if err != nil {
		return fmt.Errorf("failed to dispatch task: %w", err)
	}

	out := cmd.OutOrStdout()

	if dpDetach {
		if dpJSON {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(task)
		}

		fmt.Fprintf(out, "%s Task dispatched in background\n", dpGreen("✓"))
		fmt.Fprintf(out, "  Task ID:   %s\n", dpBold(task.ID))
		fmt.Fprintf(out, "  Profile:   %s\n", dpCyan(task.Profile))
		fmt.Fprintf(out, "  Agent:     %s\n", task.AgentType)
		if task.Prompt != "" {
			fmt.Fprintf(out, "  Prompt:    %s\n", dpDim(task.Prompt))
		}
		if task.WorktreeID != "" {
			fmt.Fprintf(out, "  Worktree:  %s (%s)\n", dpBold(task.WorktreeID), dpDim(task.Branch))
			fmt.Fprintf(out, "  Path:      %s\n", dpDim(task.WorktreePath))
		}
		if task.LogPath != "" {
			fmt.Fprintf(out, "  Log:       %s\n", dpDim(task.LogPath))
		}
		fmt.Fprintf(out, "\nTo inspect logs:   multigravity dispatch logs %s -f\n", task.ID)
		fmt.Fprintf(out, "To check diff:     multigravity dispatch diff %s\n", task.ID)
		fmt.Fprintf(out, "To check status:   multigravity dispatch status %s\n", task.ID)
		return nil
	}

	// Foreground mode: stream logs and wait for completion
	if !dpJSON {
		fmt.Fprintf(out, "%s Task %s started with profile %s\n", dpGreen("●"), dpBold(task.ID), dpCyan(task.Profile))
		if task.WorktreeID != "" {
			fmt.Fprintf(out, "  Worktree: %s (%s)\n\n", dpBold(task.WorktreeID), dpDim(task.WorktreePath))
		}
	}

	agentMgr := agent.GetDefaultManager()
	if task.SessionID != "" {
		if ch, unsub, err := agentMgr.SubscribeSession(task.SessionID); err == nil {
			defer unsub()
			for chunk := range ch {
				if !dpJSON {
					out.Write([]byte(chunk.Data))
				}
			}
		}
	}

	// Wait until task is no longer running
	var completedTask *dispatch.Task
	for {
		tCur, err := mgr.GetTask("", task.ID)
		if err == nil && tCur.Status != dispatch.StatusRunning && tCur.Status != dispatch.StatusPending {
			completedTask = tCur
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if dpJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(completedTask)
	}

	fmt.Fprintln(out)
	if completedTask.Status == dispatch.StatusCompleted {
		fmt.Fprintf(out, "%s Task %s completed successfully in %.2fs (exit code: %d)\n",
			dpGreen("✓"), dpBold(completedTask.ID), completedTask.DurationSeconds, completedTask.ExitCode)
	} else if completedTask.Status == dispatch.StatusCancelled {
		fmt.Fprintf(out, "%s Task %s was cancelled\n", dpYellow("!"), dpBold(completedTask.ID))
	} else {
		fmt.Fprintf(out, "%s Task %s failed with exit code %d\n", dpRed("✗"), dpBold(completedTask.ID), completedTask.ExitCode)
		if completedTask.Error != "" {
			fmt.Fprintf(out, "  Error: %s\n", dpRed(completedTask.Error))
		}
	}

	if completedTask.WorktreeID != "" {
		fmt.Fprintf(out, "To inspect changes: multigravity dispatch diff %s\n", completedTask.ID)
	}

	if completedTask.ExitCode != 0 {
		return fmt.Errorf("task exited with code %d", completedTask.ExitCode)
	}
	return nil
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
	fmt.Fprintf(out, "Total: %d | Running: %s | Completed: %s | Failed: %s | Cancelled: %s\n\n",
		summary.Total,
		dpCyan(fmt.Sprintf("%d", summary.Running)),
		dpGreen(fmt.Sprintf("%d", summary.Completed)),
		dpRed(fmt.Sprintf("%d", summary.Failed)),
		dpDim(fmt.Sprintf("%d", summary.Cancelled)),
	)

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
		dpJSON = false
	}()

	taskID := args[0]
	mgr := dispatch.GetDefaultTaskManager()
	if err := mgr.DeleteTask("", taskID, dpRmWorktree); err != nil {
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
