package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/ye-dev/multigravity-cli/internal/agent"
	"github.com/ye-dev/multigravity-cli/internal/worktree"
)

var (
	agAgentType  string
	agWorktree   string
	agCwd        string
	agGateway    string
	agGatewayKey string
	agDetach     bool
	agJSON       bool
	agTail       int
	agFollow     bool
	agFilterProf string
	agFilterStat string
	agFilterType string

	agGreen  = color.New(color.FgGreen).SprintFunc()
	agYellow = color.New(color.FgYellow).SprintFunc()
	agRed    = color.New(color.FgRed).SprintFunc()
	agCyan   = color.New(color.FgCyan).SprintFunc()
	agBold   = color.New(color.Bold).SprintFunc()
	agDim    = color.New(color.Faint).SprintFunc()
)

func newAgentCmd() *cobra.Command {
	agentCmd := &cobra.Command{
		Use:     "agent",
		Aliases: []string{"ag"},
		Short:   "PTY terminal multiplexer and headless runner for AI coding agents",
		Long: `Execute and multiplex AI coding agents (Claude Code, Aider, OpenCode, Agy)
in pseudo-terminal (PTY) environments with identity isolation and multi-account gateway integration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentList(cmd, args)
		},
	}

	agentCmd.PersistentFlags().BoolVar(&agJSON, "json", false, "Output results in machine-readable JSON format")

	// Run Subcommand
	runSubCmd := &cobra.Command{
		Use:   "run <profile> [--] <command> [args...]",
		Short: "Launch an agent CLI inside an isolated PTY environment",
		Args:  cobra.MinimumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return profileArgsCompletion(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: runAgentRun,
	}
	runSubCmd.Flags().StringVar(&agAgentType, "agent", "", "Agent type (claude, aider, opencode, agy, custom)")
	runSubCmd.Flags().StringVar(&agWorktree, "worktree", "", "Worktree ID to run inside")
	runSubCmd.Flags().StringVar(&agCwd, "cwd", "", "Working directory")
	runSubCmd.Flags().StringVar(&agGateway, "gateway", "", "Gateway base URL (defaults to auto-detecting multigravity serve at http://127.0.0.1:8080/v1)")
	runSubCmd.Flags().StringVar(&agGatewayKey, "gateway-key", "", "Gateway authentication key")
	runSubCmd.Flags().BoolVarP(&agDetach, "detach", "d", false, "Run in background and print session ID")

	// List Subcommand
	listSubCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List active and recent agent sessions",
		Args:    cobra.NoArgs,
		RunE:    runAgentList,
	}
	listSubCmd.Flags().StringVar(&agFilterProf, "profile", "", "Filter by profile name")
	listSubCmd.Flags().StringVar(&agFilterStat, "status", "", "Filter by status (running, exited, stopped, failed)")
	listSubCmd.Flags().StringVar(&agFilterType, "type", "", "Filter by agent type")

	// Status Subcommand
	statusSubCmd := &cobra.Command{
		Use:   "status <session-id>",
		Short: "Display details and telemetry of an agent session",
		Args:  cobra.ExactArgs(1),
		RunE:  runAgentStatus,
	}

	// Logs Subcommand
	logsSubCmd := &cobra.Command{
		Use:   "logs <session-id>",
		Short: "View buffered terminal output for an agent session",
		Args:  cobra.ExactArgs(1),
		RunE:  runAgentLogs,
	}
	logsSubCmd.Flags().IntVarP(&agTail, "tail", "n", 100, "Number of lines/bytes to view from the end")
	logsSubCmd.Flags().BoolVarP(&agFollow, "follow", "f", false, "Stream output continuously")

	// Stop Subcommand
	stopSubCmd := &cobra.Command{
		Use:   "stop <session-id>",
		Short: "Gracefully stop a running agent session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := agent.GetDefaultManager()
			if err := mgr.StopSession(args[0]); err != nil {
				return err
			}
			if agJSON {
				res := map[string]interface{}{"id": args[0], "status": "stopped"}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}
			fmt.Printf("%s Session %s stopped.\n", agGreen("✓"), agBold(args[0]))
			return nil
		},
	}

	// Kill Subcommand
	killSubCmd := &cobra.Command{
		Use:   "kill <session-id>",
		Short: "Force kill a running agent session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := agent.GetDefaultManager()
			if err := mgr.KillSession(args[0]); err != nil {
				return err
			}
			if agJSON {
				res := map[string]interface{}{"id": args[0], "status": "killed"}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}
			fmt.Printf("%s Session %s killed.\n", agRed("✓"), agBold(args[0]))
			return nil
		},
	}

	// Attach Subcommand
	attachSubCmd := &cobra.Command{
		Use:   "attach <session-id>",
		Short: "Attach interactive terminal to a running agent session",
		Args:  cobra.ExactArgs(1),
		RunE:  runAgentAttach,
	}

	agentCmd.AddCommand(runSubCmd, listSubCmd, statusSubCmd, logsSubCmd, stopSubCmd, killSubCmd, attachSubCmd)
	return agentCmd
}

func probeDefaultGateway() string {
	client := http.Client{Timeout: 60 * time.Millisecond}
	resp, err := client.Get("http://127.0.0.1:8080/health")
	if err == nil {
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return "http://127.0.0.1:8080/v1"
		}
	}
	return ""
}

func runAgentRun(cmd *cobra.Command, args []string) error {
	profileName := args[0]
	command := args[1]
	var cmdArgs []string
	if len(args) > 2 {
		cmdArgs = args[2:]
	}

	// Resolve worktree directory if specified and cwd is empty
	cwd := agCwd
	if agWorktree != "" && cwd == "" {
		if wt, err := worktree.GetWorktree(".", agWorktree); err == nil {
			cwd = wt.Path
		}
	}

	// Auto-detect gateway if empty
	gatewayURL := agGateway
	if gatewayURL == "" {
		gatewayURL = probeDefaultGateway()
	}

	// Terminal dimensions
	rows := uint16(24)
	cols := uint16(80)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		if w, h, err := term.GetSize(int(os.Stdin.Fd())); err == nil && w > 0 && h > 0 {
			cols = uint16(w)
			rows = uint16(h)
		}
	}

	opts := agent.CreateSessionOptions{
		Profile:    profileName,
		AgentType:  agAgentType,
		Command:    command,
		Args:       cmdArgs,
		Cwd:        cwd,
		WorktreeID: agWorktree,
		GatewayURL: gatewayURL,
		GatewayKey: agGatewayKey,
		Rows:       rows,
		Cols:       cols,
		Detached:   agDetach,
	}

	mgr := agent.GetDefaultManager()
	inst, err := mgr.StartSession(opts)
	if err != nil {
		return fmt.Errorf("failed to start agent session: %w", err)
	}

	info := inst.GetInfo()

	if agDetach {
		if agJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(info)
		}

		fmt.Printf("%s Agent session started in background\n", agGreen("✓"))
		fmt.Printf("  ID:        %s\n", agBold(info.ID))
		fmt.Printf("  Profile:   %s\n", agCyan(info.Profile))
		fmt.Printf("  Agent:     %s\n", info.AgentType)
		fmt.Printf("  PID:       %d\n", info.PID)
		if info.GatewayURL != "" {
			fmt.Printf("  Gateway:   %s\n", agDim(info.GatewayURL))
		}
		if info.Cwd != "" {
			fmt.Printf("  Cwd:       %s\n", agDim(info.Cwd))
		}
		fmt.Printf("\nTo view logs:   multigravity agent logs %s -f\n", info.ID)
		fmt.Printf("To attach:      multigravity agent attach %s\n", info.ID)
		return nil
	}

	// Interactive Mode
	return attachInteractive(mgr, inst)
}

func attachInteractive(mgr *agent.Manager, inst *agent.SessionInstance) error {
	info := inst.GetInfo()
	sessionID := info.ID

	// Put terminal in raw mode if interactive
	if term.IsTerminal(int(os.Stdin.Fd())) {
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err == nil {
			defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()
		}
	}

	// Subscribe to output
	ch, unsub, err := mgr.SubscribeSession(sessionID)
	if err != nil {
		return err
	}
	defer unsub()

	// Forward terminal resize signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)
	defer signal.Stop(sigChan)

	go func() {
		for range sigChan {
			if w, h, err := term.GetSize(int(os.Stdin.Fd())); err == nil && w > 0 && h > 0 {
				_ = mgr.ResizeSession(sessionID, uint16(h), uint16(w))
			}
		}
	}()

	// Forward Stdin
	inputErrChan := make(chan error, 1)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				_ = mgr.WriteSessionInput(sessionID, buf[:n])
			}
			if err != nil {
				inputErrChan <- err
				return
			}
		}
	}()

	// Stream stdout
	for chunk := range ch {
		_, _ = os.Stdout.WriteString(chunk.Data)
	}

	// Wait for process exit
	inst.Wait(1 * time.Second)
	finalInfo := inst.GetInfo()
	if finalInfo.ExitCode != 0 {
		return fmt.Errorf("agent exited with code %d", finalInfo.ExitCode)
	}
	return nil
}

func runAgentList(cmd *cobra.Command, args []string) error {
	mgr := agent.GetDefaultManager()
	filter := agent.SessionFilter{
		Profile:   agFilterProf,
		Status:    agent.SessionStatus(agFilterStat),
		AgentType: agFilterType,
	}

	sessions := mgr.ListSessions(filter)

	if agJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if sessions == nil {
			sessions = []agent.Session{}
		}
		return enc.Encode(sessions)
	}

	if len(sessions) == 0 {
		fmt.Println("No active or recorded agent sessions found.")
		fmt.Println("Run an agent with: multigravity agent run <profile> -- <command>")
		return nil
	}

	fmt.Printf("%-24s %-14s %-10s %-8s %-10s %s\n",
		agBold("SESSION ID"), agBold("PROFILE"), agBold("AGENT"), agBold("STATUS"), agBold("PID"), agBold("RUNTIME"))
	fmt.Println(strings.Repeat("─", 80))

	for _, s := range sessions {
		statusStr := string(s.Status)
		switch s.Status {
		case agent.StatusRunning:
			statusStr = agGreen("● running")
		case agent.StatusExited:
			if s.ExitCode == 0 {
				statusStr = agDim("○ exited")
			} else {
				statusStr = agRed("✖ failed (" + strconv.Itoa(s.ExitCode) + ")")
			}
		case agent.StatusStopped:
			statusStr = agYellow("■ stopped")
		}

		duration := time.Since(s.StartedAt).Round(time.Second)
		if s.EndedAt != nil {
			duration = s.EndedAt.Sub(s.StartedAt).Round(time.Second)
		}

		fmt.Printf("%-24s %-14s %-10s %-8s %-10d %s\n",
			s.ID, s.Profile, s.AgentType, statusStr, s.PID, duration)
	}
	return nil
}

func runAgentStatus(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	mgr := agent.GetDefaultManager()

	inst, err := mgr.GetSession(sessionID)
	if err != nil {
		return err
	}

	info := inst.GetInfo()

	if agJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}

	fmt.Printf("%s Session Telemetry\n", agBold("Multigravity Agent"))
	fmt.Println(strings.Repeat("─", 40))
	fmt.Printf("  ID:          %s\n", agBold(info.ID))
	fmt.Printf("  Profile:     %s\n", agCyan(info.Profile))
	fmt.Printf("  Agent Type:  %s\n", info.AgentType)
	fmt.Printf("  Command:     %s %s\n", info.Command, strings.Join(info.Args, " "))
	fmt.Printf("  PID:         %d\n", info.PID)
	fmt.Printf("  Status:      %s\n", info.Status)
	fmt.Printf("  Exit Code:   %d\n", info.ExitCode)
	if info.GatewayURL != "" {
		fmt.Printf("  Gateway:     %s\n", info.GatewayURL)
	}
	if info.WorktreeID != "" {
		fmt.Printf("  Worktree:    %s\n", info.WorktreeID)
	}
	if info.Cwd != "" {
		fmt.Printf("  Cwd:         %s\n", info.Cwd)
	}
	fmt.Printf("  Dimensions:  %dx%d (rows x cols)\n", info.Rows, info.Cols)
	fmt.Printf("  Started:     %s\n", info.StartedAt.Format(time.RFC3339))
	if info.EndedAt != nil {
		fmt.Printf("  Ended:       %s\n", info.EndedAt.Format(time.RFC3339))
	}
	return nil
}

func runAgentLogs(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	mgr := agent.GetDefaultManager()

	inst, err := mgr.GetSession(sessionID)
	if err != nil {
		return err
	}

	// Fetch initial buffer
	tailBytes := agTail * 128 // rough estimate per line if positive
	if agTail <= 0 {
		tailBytes = 0
	}
	buf := inst.GetOutput(tailBytes)
	if len(buf) > 0 {
		_, _ = os.Stdout.Write(buf)
	}

	if !agFollow {
		return nil
	}

	// Live stream
	ch, unsub := inst.Subscribe()
	defer unsub()

	for chunk := range ch {
		_, _ = os.Stdout.WriteString(chunk.Data)
	}
	return nil
}

func runAgentAttach(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	mgr := agent.GetDefaultManager()

	inst, err := mgr.GetSession(sessionID)
	if err != nil {
		return err
	}

	info := inst.GetInfo()
	if info.Status != agent.StatusRunning {
		return fmt.Errorf("session %s is not running (status: %s)", sessionID, info.Status)
	}

	return attachInteractive(mgr, inst)
}
