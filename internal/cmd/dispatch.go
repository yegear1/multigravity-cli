package cmd

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ye-dev/multigravity-cli/internal/dispatch"
)

var (
	dpJSON bool

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
		Long: `Dispatch and supervise autonomous coding agent tasks (Claude Code, Aider, OpenCode, Agy, Codex)
with profile-isolated credentials, ephemeral git worktrees, and persistent log capture.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDispatchList(cmd, args)
		},
	}

	dispatchCmd.PersistentFlags().BoolVar(&dpJSON, "json", false, "Output results in machine-readable JSON format")

	dispatchCmd.AddCommand(
		newDispatchRunCmd(),
		newDispatchListCmd(),
		newDispatchStatusCmd(),
		newDispatchLogsCmd(),
		newDispatchDiffCmd(),
		newDispatchDashboardCmd(),
		newDispatchCancelCmd(),
		newDispatchDeleteCmd(),
		newDispatchPruneCmd(),
		newDispatchPlanCmd(),
		newDispatchWaveCmd(),
		newDispatchApplyCmd(),
	)
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
