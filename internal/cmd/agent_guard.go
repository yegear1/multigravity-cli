package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/agent"
)

var (
	guardRole     string
	guardRepo     string
	guardReadOnly bool
)

func newAgentGuardCmd() *cobra.Command {
	guardCmd := &cobra.Command{
		Use:   "agent-guard",
		Short: "Process sandboxing and filesystem boundary supervisor for agent runners",
		Long: `agent-guard supervises agent executions within defined repository boundaries,
intercepting and preventing destructive mutations outside the target repository root.`,
	}

	wrapCmd := &cobra.Command{
		Use:   "wrap [flags] -- <command> [args...]",
		Short: "Wrap a command with repo filesystem boundaries and subshell mutation interception",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runAgentGuardWrap,
	}

	wrapCmd.Flags().StringVar(&guardRole, "role", "developer", "Execution role boundary (developer, researcher, verifier, reviewer, auditor)")
	wrapCmd.Flags().StringVar(&guardRepo, "repo", ".", "Target repository root boundary")
	wrapCmd.Flags().BoolVar(&guardReadOnly, "read-only", false, "Enforce strict read-only execution prohibiting any filesystem mutation")

	guardCmd.AddCommand(wrapCmd)
	return guardCmd
}

func runAgentGuardWrap(cmd *cobra.Command, args []string) error {
	defer func() {
		guardRole = "developer"
		guardRepo = "."
		guardReadOnly = false
	}()

	if len(args) == 0 {
		return fmt.Errorf("command is required after -- in agent-guard wrap")
	}

	repoPath := guardRepo
	if repoPath == "" {
		repoPath = "."
	}

	absRepo, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		return fmt.Errorf("invalid repository directory %q: %w", repoPath, err)
	}

	fi, err := os.Stat(absRepo)
	if err != nil || !fi.IsDir() {
		return fmt.Errorf("repository directory does not exist or is not a directory: %s", absRepo)
	}

	command := args[0]
	cmdArgs := args[1:]

	// 1. Inspect subshell and direct command for mutation violations outside target repo or role violations
	fullCmdLine := strings.Join(args, " ")
	if err := agent.InterceptSubshellMutationWithRole(absRepo, fullCmdLine, guardRole, guardReadOnly); err != nil {
		return err
	}

	// 2. If running via sh -c or bash -c, inspect the subshell argument specifically
	baseCmd := strings.ToLower(filepath.Base(command))
	baseCmd = strings.TrimSuffix(baseCmd, ".exe")
	if (baseCmd == "sh" || baseCmd == "bash") && len(cmdArgs) >= 2 && cmdArgs[0] == "-c" {
		if err := agent.InterceptSubshellMutationWithRole(absRepo, cmdArgs[1], guardRole, guardReadOnly); err != nil {
			return err
		}
	}

	// 3. Execute supervised child process
	child := exec.Command(command, cmdArgs...)
	child.Dir = absRepo
	child.Stdin = cmd.InOrStdin()
	child.Stdout = cmd.OutOrStdout()
	child.Stderr = cmd.ErrOrStderr()

	return child.Run()
}
