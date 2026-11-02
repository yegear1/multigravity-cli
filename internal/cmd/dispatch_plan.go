package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ye-dev/multigravity-cli/internal/dispatch"
)

var (
	dpPlanWorkers     int
	dpPlanTimeout     string
	dpPlanRepo        string
	dpPlanApplyAtomic bool
	dpPlanOmitDiff    bool
	dpPlanFull        bool
	dpPlanRole        string
	dpPlanReadOnly    bool
	dpPlanWorkspace   string
)

func newDispatchPlanCmd() *cobra.Command {
	planSubCmd := &cobra.Command{
		Use:   "plan <plan.json>",
		Short: "Dispatch a multi-subtask plan concurrently across ephemeral worktrees",
		Long: `Read a JSON plan specification and execute its subtasks concurrently across isolated
git worktrees, returning a unified summary of execution status and diffs. Use '-' to read from stdin.`,
		Args: cobra.ExactArgs(1),
		RunE: runDispatchPlan,
	}
	planSubCmd.Flags().IntVar(&dpPlanWorkers, "workers", 0, "Override worker pool concurrency limit")
	planSubCmd.Flags().StringVar(&dpPlanTimeout, "timeout", "", "Execution timeout duration (e.g. 5m, 300s)")
	planSubCmd.Flags().StringVar(&dpPlanRepo, "repo", "", "Target git repository directory")
	planSubCmd.Flags().BoolVar(&dpPlanApplyAtomic, "apply-atomic", false, "Atomically apply diffs to base repo after completion")
	planSubCmd.Flags().BoolVar(&dpPlanOmitDiff, "omit-diff", false, "Omit diff bodies in output for token efficiency")
	planSubCmd.Flags().BoolVar(&dpPlanFull, "full", false, "Output full verbose plan result including prompts and diffs")
	planSubCmd.Flags().StringVar(&dpPlanRole, "role", "", "Execution role boundary for plan subtasks (e.g. developer, researcher, verifier)")
	planSubCmd.Flags().BoolVar(&dpPlanReadOnly, "read-only", false, "Enforce strict read-only execution across plan subtasks")
	planSubCmd.Flags().StringVar(&dpPlanWorkspace, "workspace", "", "Workspace isolation mode for plan subtasks (worktree, mirror, inherit)")
	return planSubCmd
}

func runDispatchPlan(cmd *cobra.Command, args []string) error {
	defer func() {
		dpPlanWorkers = 0
		dpPlanTimeout = ""
		dpPlanRepo = ""
		dpPlanApplyAtomic = false
		dpPlanOmitDiff = false
		dpPlanFull = false
		dpPlanRole = ""
		dpPlanReadOnly = false
		dpPlanWorkspace = ""
		dpJSON = false
	}()

	planArg := args[0]
	var planData []byte
	var err error

	if planArg == "-" {
		planData, err = io.ReadAll(cmd.InOrStdin())
	} else {
		planData, err = os.ReadFile(planArg)
	}
	if err != nil {
		return fmt.Errorf("failed to read plan file: %w", err)
	}

	var req dispatch.PlanRequest
	if err := json.Unmarshal(planData, &req); err != nil {
		return fmt.Errorf("invalid plan JSON: %w", err)
	}

	if dpPlanWorkers > 0 {
		req.Workers = dpPlanWorkers
	}
	if dpPlanTimeout != "" {
		req.Timeout = dpPlanTimeout
	}
	if dpPlanRepo != "" {
		req.RepoPath = dpPlanRepo
	}
	if dpPlanApplyAtomic {
		req.ApplyAtomic = true
	}
	if dpPlanOmitDiff {
		req.OmitDiff = true
	}
	if dpPlanRole != "" {
		req.Role = dpPlanRole
	}
	if dpPlanReadOnly {
		req.ReadOnly = true
	}
	if dpPlanWorkspace != "" {
		req.Workspace = dpPlanWorkspace
	}

	out := cmd.OutOrStdout()
	if !dpJSON {
		fmt.Fprintf(out, "%s Dispatching plan across isolated worktrees...\n", dpCyan("▶"))
	}

	mgr := dispatch.GetDefaultTaskManager()
	result, err := mgr.ExecutePlan(cmd.Context(), req)
	if err != nil {
		return err
	}

	if dpJSON {
		outResult := result
		if !dpPlanFull {
			outResult = result.ToCompact()
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(outResult)
	}

	statusStr := dpGreen(result.Status)
	if result.Status == "failed" {
		statusStr = dpRed(result.Status)
	} else if result.Status == "partial" || result.Status == "timeout" {
		statusStr = dpYellow(result.Status)
	}

	fmt.Fprintf(out, "\n%s Plan Execution: %s [%s] (%.2fs)\n", dpBold("Plan:"), dpBold(result.PlanID), statusStr, result.DurationSeconds)
	fmt.Fprintf(out, "  Subtasks: %d total, %s succeeded, %s failed\n",
		result.TotalSubtasks,
		dpGreen(fmt.Sprintf("%d", result.Succeeded)),
		dpRed(fmt.Sprintf("%d", result.Failed)),
	)

	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "\nSUBTASK\tPROFILE\tSTATUS\tEXIT\tFILES CHANGED\tBRANCH")
	fmt.Fprintln(w, "-------\t-------\t------\t----\t-------------\t------")
	for _, st := range result.Subtasks {
		stStatus := dpGreen(string(st.Status))
		if st.Status == dispatch.StatusFailed {
			stStatus = dpRed(string(st.Status))
		}
		files := strings.Join(st.FilesChanged, ", ")
		if files == "" {
			files = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
			st.ID, st.Profile, stStatus, st.ExitCode, files, st.Branch)
	}
	_ = w.Flush()

	summary := result.UnifiedSummary
	fmt.Fprintf(out, "\n%s\n", dpBold("Unified Diff Summary:"))
	fmt.Fprintf(out, "  Files Modified: %d\n", summary.TotalFilesChanged)
	fmt.Fprintf(out, "  Additions:      %s\n", dpGreen(fmt.Sprintf("+%d", summary.TotalAdditions)))
	fmt.Fprintf(out, "  Deletions:      %s\n", dpRed(fmt.Sprintf("-%d", summary.TotalDeletions)))

	if summary.DisjointScopesClean {
		fmt.Fprintf(out, "  Disjoint Scope: %s Clean (zero overlapping files across worktrees)\n", dpGreen("✓"))
	} else {
		fmt.Fprintf(out, "  Disjoint Scope: %s Collision detected in %v\n", dpYellow("⚠"), summary.ConflictingFiles)
	}

	if result.Status != "completed" {
		return fmt.Errorf("plan finished with status: %s", result.Status)
	}
	return nil
}
