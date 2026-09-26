package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/headless"
)

var (
	hlJSON            bool
	hlPort            int
	hlTimeout         time.Duration
	hlForce           bool
	hlTail            int
	hlFollow          bool
	hlSkipPermissions bool

	hlGreen  = color.New(color.FgGreen).SprintFunc()
	hlYellow = color.New(color.FgYellow).SprintFunc()
	hlRed    = color.New(color.FgRed).SprintFunc()
	hlCyan   = color.New(color.FgCyan).SprintFunc()
	hlBold   = color.New(color.Bold).SprintFunc()
	hlDim    = color.New(color.Faint).SprintFunc()
)

func newHeadlessCmd() *cobra.Command {
	headlessCmd := &cobra.Command{
		Use:     "headless",
		Aliases: []string{"hl"},
		Short:   "Manage background headless language servers and agent invocations",
		Long: `Manage persistent background headless language server instances per profile
with strict identity isolation, and execute headless agent prompts.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHeadlessList(cmd, args)
		},
	}

	headlessCmd.PersistentFlags().BoolVar(&hlJSON, "json", false, "Output results in machine-readable JSON format")

	// List Subcommand
	listSubCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List active background headless language server instances",
		Args:    cobra.NoArgs,
		RunE:    runHeadlessList,
	}

	// Status Subcommand
	statusSubCmd := &cobra.Command{
		Use:   "status <profile>",
		Short: "Display status and health of a profile's headless server",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return profileArgsCompletion(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: runHeadlessStatus,
	}

	// Start Subcommand
	startSubCmd := &cobra.Command{
		Use:   "start <profile>",
		Short: "Start a background headless language server for a profile",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return profileArgsCompletion(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: runHeadlessStart,
	}
	startSubCmd.Flags().IntVarP(&hlPort, "port", "p", 0, "Specific HTTPS port to bind (default 0: auto)")
	startSubCmd.Flags().DurationVarP(&hlTimeout, "timeout", "t", 10*time.Second, "Startup timeout")
	startSubCmd.Flags().BoolVarP(&hlForce, "force", "f", false, "Force restart if an instance is already running")

	// Stop Subcommand
	stopSubCmd := &cobra.Command{
		Use:   "stop <profile>",
		Short: "Stop a running background headless language server",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return profileArgsCompletion(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: runHeadlessStop,
	}

	// Restart Subcommand
	restartSubCmd := &cobra.Command{
		Use:   "restart <profile>",
		Short: "Restart a profile's background headless language server",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return profileArgsCompletion(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: runHeadlessRestart,
	}
	restartSubCmd.Flags().IntVarP(&hlPort, "port", "p", 0, "Specific HTTPS port to bind (default 0: auto)")
	restartSubCmd.Flags().DurationVarP(&hlTimeout, "timeout", "t", 10*time.Second, "Startup timeout")

	// Logs Subcommand
	logsSubCmd := &cobra.Command{
		Use:   "logs <profile>",
		Short: "View logs from a profile's background headless server",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return profileArgsCompletion(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: runHeadlessLogs,
	}
	logsSubCmd.Flags().IntVarP(&hlTail, "tail", "n", 50, "Number of lines to show from the end")
	logsSubCmd.Flags().BoolVarP(&hlFollow, "follow", "f", false, "Stream logs continuously")

	// Run Subcommand
	runSubCmd := &cobra.Command{
		Use:   "run <profile> <prompt>",
		Short: "Execute a headless agent prompt with strict identity isolation",
		Args:  cobra.MinimumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return profileArgsCompletion(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: runHeadlessRunPrompt,
	}
	runSubCmd.Flags().DurationVarP(&hlTimeout, "timeout", "t", 120*time.Second, "Execution timeout")
	runSubCmd.Flags().BoolVar(&hlSkipPermissions, "dangerously-skip-permissions", true, "Skip interactive sandbox/permission confirmations")

	headlessCmd.AddCommand(listSubCmd)
	headlessCmd.AddCommand(statusSubCmd)
	headlessCmd.AddCommand(startSubCmd)
	headlessCmd.AddCommand(stopSubCmd)
	headlessCmd.AddCommand(restartSubCmd)
	headlessCmd.AddCommand(logsSubCmd)
	headlessCmd.AddCommand(runSubCmd)

	return headlessCmd
}

func runHeadlessList(cmd *cobra.Command, args []string) error {
	defer func() {
		hlJSON = false
	}()

	mgr := headless.GetDefaultManager()
	list, err := mgr.List()
	if err != nil {
		return err
	}

	if hlJSON {
		if list == nil {
			list = []*headless.InstanceInfo{}
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(list)
	}

	if len(list) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), hlDim("No active background headless instances found."))
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, hlBold("PROFILE\tPID\tPORT\tSTATUS\tSTARTED"))
	for _, inst := range list {
		statusStr := string(inst.Status)
		switch inst.Status {
		case headless.StateRunning:
			statusStr = hlGreen("running")
		case headless.StateUnhealthy:
			statusStr = hlYellow("unhealthy")
		case headless.StateFailed:
			statusStr = hlRed("failed")
		default:
			statusStr = hlDim(statusStr)
		}

		startedStr := inst.StartedAt.Format("15:04:05 02/01/2006")
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%s\n", inst.Profile, inst.PID, inst.Port, statusStr, startedStr)
	}
	return w.Flush()
}

func runHeadlessStatus(cmd *cobra.Command, args []string) error {
	defer func() {
		hlJSON = false
	}()

	profileName := args[0]
	mgr := headless.GetDefaultManager()
	st, err := mgr.GetStatus(profileName)
	if err != nil {
		return err
	}

	if hlJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(st)
	}

	statusStr := string(st.Status)
	switch st.Status {
	case headless.StateRunning:
		statusStr = hlGreen("● RUNNING")
	case headless.StateUnhealthy:
		statusStr = hlYellow("▲ UNHEALTHY: " + st.HealthError)
	case headless.StateFailed:
		statusStr = hlRed("✖ FAILED")
	default:
		statusStr = hlDim("○ STOPPED")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", hlBold("Profile:"), st.Profile)
	fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", hlBold("Status:"), statusStr)
	if st.Status != headless.StateStopped {
		fmt.Fprintf(cmd.OutOrStdout(), "%s     %d\n", hlBold("PID:"), st.PID)
		fmt.Fprintf(cmd.OutOrStdout(), "%s    %d\n", hlBold("Port:"), st.Port)
		fmt.Fprintf(cmd.OutOrStdout(), "%s    %s\n", hlBold("CSRF:"), st.CSRFToken)
		if !st.StartedAt.IsZero() {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", hlBold("Started:"), st.StartedAt.Format(time.RFC3339))
		}
		if st.LogFile != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%s     %s\n", hlBold("Log:"), hlDim(st.LogFile))
		}
	}
	return nil
}

func runHeadlessStart(cmd *cobra.Command, args []string) error {
	defer func() {
		hlJSON = false
		hlPort = 0
		hlTimeout = 10 * time.Second
		hlForce = false
	}()

	profileName := args[0]
	mgr := headless.GetDefaultManager()
	inst, err := mgr.Start(profileName, headless.StartOptions{
		Profile:      profileName,
		Port:         hlPort,
		Timeout:      hlTimeout,
		ForceRestart: hlForce,
	})
	if err != nil {
		return err
	}

	if hlJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(inst)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s Started background headless language server for profile %s (PID: %d, Port: %d)\n",
		hlGreen("✓"), hlBold(profileName), inst.PID, inst.Port)
	return nil
}

func runHeadlessStop(cmd *cobra.Command, args []string) error {
	defer func() {
		hlJSON = false
	}()

	profileName := args[0]
	mgr := headless.GetDefaultManager()
	if err := mgr.Stop(profileName); err != nil {
		return err
	}

	if hlJSON {
		res := map[string]interface{}{
			"profile": profileName,
			"status":  "stopped",
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s Headless server for profile %s stopped.\n", hlGreen("✓"), hlBold(profileName))
	return nil
}

func runHeadlessRestart(cmd *cobra.Command, args []string) error {
	defer func() {
		hlJSON = false
		hlPort = 0
		hlTimeout = 10 * time.Second
	}()

	profileName := args[0]
	mgr := headless.GetDefaultManager()
	inst, err := mgr.Restart(profileName, headless.StartOptions{
		Profile: profileName,
		Port:    hlPort,
		Timeout: hlTimeout,
	})
	if err != nil {
		return err
	}

	if hlJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(inst)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s Restarted background headless server for profile %s (PID: %d, Port: %d)\n",
		hlGreen("✓"), hlBold(profileName), inst.PID, inst.Port)
	return nil
}

func runHeadlessLogs(cmd *cobra.Command, args []string) error {
	profileName := args[0]
	mgr := headless.GetDefaultManager()

	if !hlFollow {
		logs, err := mgr.GetLogs(profileName, hlTail)
		if err != nil {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), logs)
		return nil
	}

	// Follow loop
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastLen int
	for range ticker.C {
		logs, err := mgr.GetLogs(profileName, 0)
		if err != nil {
			return err
		}
		if len(logs) > lastLen {
			newChunk := logs[lastLen:]
			fmt.Fprint(cmd.OutOrStdout(), newChunk)
			lastLen = len(logs)
		}
	}
	return nil
}

func runHeadlessRunPrompt(cmd *cobra.Command, args []string) error {
	defer func() {
		hlJSON = false
		hlTimeout = 120 * time.Second
		hlSkipPermissions = true
	}()

	profileName := args[0]
	prompt := strings.Join(args[1:], " ")

	mgr := headless.GetDefaultManager()
	result, err := mgr.RunAgentPrompt(headless.AgentRunOptions{
		Profile:                    profileName,
		Prompt:                     prompt,
		Timeout:                    hlTimeout,
		DangerouslySkipPermissions: hlSkipPermissions,
	})
	if err != nil {
		return err
	}

	if hlJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if result.ExitCode != 0 {
		fmt.Fprintf(cmd.OutOrStderr(), "%s Agent run exited with code %d: %s\n", hlRed("✖"), result.ExitCode, result.Error)
		return fmt.Errorf("agent run failed")
	}

	fmt.Fprintln(cmd.OutOrStdout(), result.Response)
	if result.TotalTokens > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "%s Tokens: %d | Duration: %.2fs\n", hlDim("---"), result.TotalTokens, result.DurationSeconds)
	}
	return nil
}
