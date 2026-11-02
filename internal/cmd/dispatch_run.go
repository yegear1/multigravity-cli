package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ye-dev/multigravity-cli/internal/agent"
	"github.com/ye-dev/multigravity-cli/internal/dispatch"
)

var (
	dpPrompt       string
	dpAgentType    string
	dpNewWorktree  bool
	dpWorktree     string
	dpBranch       string
	dpBaseCommit   string
	dpCwd          string
	dpGateway      string
	dpGatewayKey   string
	dpDetach       bool
	dpRunProfile   string
	dpRunRepo      string
	dpSandbox      bool
	dpRole         string
	dpReadOnly     bool
	dpWorkspace    string
	dpVerify       string
	dpRetry        int
	dpNoGovernance bool
	dpTraceID      string
)

func newDispatchRunCmd() *cobra.Command {
	runSubCmd := &cobra.Command{
		Use:   "run [profile] [flags] [--] [command] [args...]",
		Short: "Dispatch an agent task with isolated profile credentials and worktree",
		Args:  cobra.ArbitraryArgs,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return profileArgsCompletion(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: runDispatchRun,
	}
	runSubCmd.Flags().StringVar(&dpRunProfile, "profile", "", "Profile to use for task execution")
	runSubCmd.Flags().StringVar(&dpRunRepo, "repo", "", "Target git repository directory")
	runSubCmd.Flags().StringVarP(&dpPrompt, "prompt", "p", "", "Instruction prompt for the agent")
	runSubCmd.Flags().StringVar(&dpAgentType, "agent", "", "Agent type (claude, aider, opencode, agy, codex, custom)")
	runSubCmd.Flags().BoolVarP(&dpNewWorktree, "new-worktree", "w", false, "Create a dedicated ephemeral git worktree for the task")
	runSubCmd.Flags().StringVar(&dpWorktree, "worktree", "", "Associate with an existing worktree ID")
	runSubCmd.Flags().StringVar(&dpWorkspace, "workspace", "", "Workspace isolation mode (worktree, mirror, inherit)")
	runSubCmd.Flags().StringVar(&dpBranch, "branch", "", "Git branch name for newly created worktree")
	runSubCmd.Flags().StringVar(&dpBaseCommit, "base", "", "Base commit or branch to branch off")
	runSubCmd.Flags().StringVar(&dpCwd, "cwd", "", "Custom working directory")
	runSubCmd.Flags().StringVar(&dpGateway, "gateway", "", "Gateway base URL (defaults to auto-detecting http://127.0.0.1:8080/v1)")
	runSubCmd.Flags().StringVar(&dpGatewayKey, "gateway-key", "", "Gateway authentication key")
	runSubCmd.Flags().BoolVar(&dpSandbox, "sandbox", false, "Enforce filesystem and execution boundaries via agent-guard")
	runSubCmd.Flags().StringVar(&dpRole, "role", "developer", "Execution role boundary (developer, researcher, verifier, reviewer, auditor)")
	runSubCmd.Flags().BoolVar(&dpReadOnly, "read-only", false, "Enforce strict read-only execution prohibiting any filesystem mutation")
	runSubCmd.Flags().StringVar(&dpVerify, "verify", "", "Verification command to execute inside worktree upon agent completion (e.g. 'go test ./...')")
	runSubCmd.Flags().IntVar(&dpRetry, "retry", 0, "Maximum number of retries if verification fails")
	runSubCmd.Flags().StringVar(&dpTraceID, "trace-id", "", "Distributed trace identifier (or MAESTRI_TRACE_ID)")
	runSubCmd.Flags().BoolVar(&dpNoGovernance, "no-governance", false, "Disable automatic injection of project governance directives (AGENTS.md, TASK.md)")
	runSubCmd.Flags().BoolVarP(&dpDetach, "detach", "d", false, "Run task in background and print task ID immediately")
	return runSubCmd
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
		dpRunRepo = ""
		dpRunProfile = ""
		dpSandbox = false
		dpRole = ""
		dpReadOnly = false
		dpWorkspace = ""
		dpGateway = ""
		dpGatewayKey = ""
		dpVerify = ""
		dpRetry = 0
		dpNoGovernance = false
		dpDetach = false
		dpJSON = false
	}()

	var positionalProfile string
	var command string
	var cmdArgs []string

	dashAt := cmd.ArgsLenAtDash()
	if dashAt >= 0 {
		if dashAt > 0 {
			positionalProfile = args[0]
		}
		if len(args) > dashAt {
			command = args[dashAt]
			if len(args) > dashAt+1 {
				cmdArgs = args[dashAt+1:]
			}
		}
	} else if len(args) > 0 {
		positionalProfile = args[0]
		if len(args) > 1 {
			command = args[1]
			if len(args) > 2 {
				cmdArgs = args[2:]
			}
		}
	}

	if positionalProfile == "" && dpRunProfile == "" {
		_ = cmd.Usage()
		return fmt.Errorf("profile is required: provide as positional argument or via --profile")
	}

	if positionalProfile != "" && dpRunProfile != "" && positionalProfile != dpRunProfile {
		return fmt.Errorf("positional profile %q does not match --profile %q", positionalProfile, dpRunProfile)
	}

	profileName := dpRunProfile
	if profileName == "" {
		profileName = positionalProfile
	}

	gatewayURL := dpGateway
	if gatewayURL == "" {
		gatewayURL = probeDefaultGateway()
	}

	workspaceMode := strings.ToLower(strings.TrimSpace(dpWorkspace))
	if workspaceMode != "" {
		switch dispatch.WorkspaceMode(workspaceMode) {
		case dispatch.WorkspaceWorktree, dispatch.WorkspaceMirror, dispatch.WorkspaceInherit:
		default:
			return fmt.Errorf("invalid workspace mode %q: must be one of %q, %q, %q",
				dpWorkspace, dispatch.WorkspaceWorktree, dispatch.WorkspaceMirror, dispatch.WorkspaceInherit)
		}
	} else if dpNewWorktree {
		workspaceMode = string(dispatch.WorkspaceWorktree)
	}

	opts := dispatch.DispatchOptions{
		Profile:      profileName,
		RepoPath:     dpRunRepo,
		Prompt:       dpPrompt,
		AgentType:    dpAgentType,
		Command:      command,
		Args:         cmdArgs,
		Sandbox:      dpSandbox || dpReadOnly || (dpRole != "" && dpRole != "developer"),
		Role:         dpRole,
		ReadOnly:     dpReadOnly,
		Workspace:    workspaceMode,
		Verify:       dpVerify,
		Retry:        dpRetry,
		TraceID:      dpTraceID,
		NoGovernance: dpNoGovernance,
		NewWorktree:  dpNewWorktree || workspaceMode == string(dispatch.WorkspaceWorktree),
		WorktreeID:   dpWorktree,
		Branch:       dpBranch,
		BaseCommit:   dpBaseCommit,
		Cwd:          dpCwd,
		GatewayURL:   gatewayURL,
		GatewayKey:   dpGatewayKey,
		Detached:     dpDetach,
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
		fmt.Fprintf(out, "  Task ID:    %s\n", dpBold(task.ID))
		fmt.Fprintf(out, "  Profile:    %s\n", dpCyan(task.Profile))
		fmt.Fprintf(out, "  Agent:      %s\n", task.AgentType)
		if task.TraceID != "" {
			fmt.Fprintf(out, "  Trace ID:   %s\n", dpCyan(task.TraceID))
		}
		if task.Governance {
			fmt.Fprintf(out, "  Governance: %s\n", dpGreen("injected"))
		}
		if task.Role != "" {
			fmt.Fprintf(out, "  Role:       %s\n", dpCyan(task.Role))
		}
		if task.ReadOnly {
			fmt.Fprintf(out, "  Mode:       %s\n", dpYellow("read-only"))
		}
		if task.Workspace != "" {
			fmt.Fprintf(out, "  Workspace:  %s\n", dpCyan(task.Workspace))
		}
		if task.Prompt != "" {
			fmt.Fprintf(out, "  Prompt:     %s\n", dpDim(task.Prompt))
		}
		if task.Verify != "" {
			fmt.Fprintf(out, "  Verify:    %s\n", dpDim(task.Verify))
		}
		if task.Retry > 0 {
			fmt.Fprintf(out, "  Retry:     %d\n", task.Retry)
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
		if task.Workspace != "" {
			fmt.Fprintf(out, "  Workspace: %s\n", dpCyan(task.Workspace))
		}
		if task.WorktreeID != "" {
			fmt.Fprintf(out, "  Worktree: %s (%s)\n\n", dpBold(task.WorktreeID), dpDim(task.WorktreePath))
		}
	}

	agentMgr := agent.GetDefaultManager()
	subscribedSessions := make(map[string]bool)
	var completedTask *dispatch.Task
	for {
		tCur, err := mgr.GetTask("", task.ID)
		if err == nil {
			if tCur.SessionID != "" && !subscribedSessions[tCur.SessionID] {
				currSessionID := tCur.SessionID
				subscribedSessions[currSessionID] = true

				if tCur.RetryCount > 0 && !dpJSON {
					fmt.Fprintf(out, "\n%s Verification failed. Retrying task (attempt %d/%d)...\n\n",
						dpYellow("↻"), tCur.RetryCount, tCur.Retry)
				}

				if ch, unsub, err := agentMgr.SubscribeSession(currSessionID); err == nil {
					for chunk := range ch {
						if !dpJSON {
							out.Write([]byte(chunk.Data))
						}
					}
					unsub()
				}
			}

			if tCur.Status != dispatch.StatusRunning && tCur.Status != dispatch.StatusPending {
				completedTask = tCur
				break
			}
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
		if completedTask.Verify != "" {
			fmt.Fprintf(out, "  Verification passed: %s\n", dpDim(completedTask.Verify))
		}
	} else if completedTask.Status == dispatch.StatusCancelled {
		fmt.Fprintf(out, "%s Task %s was cancelled\n", dpYellow("!"), dpBold(completedTask.ID))
	} else if completedTask.Status == dispatch.StatusAuditFailed {
		fmt.Fprintf(out, "%s Task %s audit verification failed (exit code: %d)\n",
			dpRed("✗"), dpBold(completedTask.ID), completedTask.VerificationExitCode)
		if completedTask.Verify != "" {
			fmt.Fprintf(out, "  Command: %s\n", dpDim(completedTask.Verify))
		}
		if completedTask.VerificationOutput != "" {
			fmt.Fprintf(out, "  Output:\n%s\n", strings.TrimRight(completedTask.VerificationOutput, "\r\n"))
		} else if completedTask.Error != "" {
			fmt.Fprintf(out, "  Error: %s\n", dpRed(completedTask.Error))
		}
	} else {
		fmt.Fprintf(out, "%s Task %s failed with exit code %d\n", dpRed("✗"), dpBold(completedTask.ID), completedTask.ExitCode)
		if completedTask.Error != "" {
			fmt.Fprintf(out, "  Error: %s\n", dpRed(completedTask.Error))
		}
	}

	if completedTask.WorktreeID != "" {
		fmt.Fprintf(out, "To inspect changes: multigravity dispatch diff %s\n", completedTask.ID)
	}

	if completedTask.Status == dispatch.StatusAuditFailed {
		return fmt.Errorf("task audit verification failed with exit code %d", completedTask.VerificationExitCode)
	}
	if completedTask.ExitCode != 0 {
		return fmt.Errorf("task exited with code %d", completedTask.ExitCode)
	}
	return nil
}
