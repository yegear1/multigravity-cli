package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/worktree"
)

var (
	wtRepo         string
	wtBranch       string
	wtBase         string
	wtDir          string
	wtProfile      string
	wtTaskID       string
	wtForce        bool
	wtDeleteBranch bool
	wtStatOnly     bool
	wtCached       bool
	wtJSON         bool

	wtGreen  = color.New(color.FgGreen).SprintFunc()
	wtYellow = color.New(color.FgYellow).SprintFunc()
	wtRed    = color.New(color.FgRed).SprintFunc()
	wtCyan   = color.New(color.FgCyan).SprintFunc()
	wtBold   = color.New(color.Bold).SprintFunc()
	wtDim    = color.New(color.Faint).SprintFunc()
)

func worktreeIDsCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		wts, err := worktree.ListWorktrees(wtRepo)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var ids []string
		for _, wt := range wts {
			ids = append(ids, wt.ID)
		}
		return ids, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

func newWorktreeCmd() *cobra.Command {
	wtCmd := &cobra.Command{
		Use:     "worktree",
		Aliases: []string{"wt"},
		Short:   "Manage isolated, ephemeral Git worktrees for agents and tasks",
		Long: `Manage ephemeral Git worktrees to allow AI agents, CLI runners, and profiles
to execute tasks in true parallelism without working directory conflicts or dirty commits.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorktreeList(cmd, args)
		},
	}

	wtCmd.PersistentFlags().StringVar(&wtRepo, "repo", "", "Path to git repository (defaults to current working directory)")
	wtCmd.PersistentFlags().BoolVar(&wtJSON, "json", false, "Output results in machine-readable JSON format")

	// List
	listSubCmd := &cobra.Command{
		Use:   "list",
		Short: "List tracked worktrees in the repository",
		Args:  cobra.NoArgs,
		RunE:  runWorktreeList,
	}

	// Create
	createSubCmd := &cobra.Command{
		Use:   "create <id>",
		Short: "Create a new isolated Git worktree for an agent or task",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorktreeCreate,
	}
	createSubCmd.Flags().StringVarP(&wtBranch, "branch", "b", "", "Branch name to create or checkout (defaults to multigravity/<id>)")
	createSubCmd.Flags().StringVar(&wtBase, "base", "", "Base commit or branch to branch off (defaults to HEAD)")
	createSubCmd.Flags().StringVar(&wtDir, "dir", "", "Target directory for worktree (defaults to <repo>/.multigravity/worktrees/<id>)")
	createSubCmd.Flags().StringVar(&wtProfile, "profile", "", "Associated Multigravity profile")
	createSubCmd.Flags().StringVar(&wtTaskID, "task", "", "Associated task ID")
	_ = createSubCmd.RegisterFlagCompletionFunc("profile", profileArgsCompletion)

	// Status
	statusSubCmd := &cobra.Command{
		Use:               "status <id>",
		Short:             "Show Git status, modified files, and commit tracking for a worktree",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: worktreeIDsCompletion,
		RunE:              runWorktreeStatus,
	}

	// Diff
	diffSubCmd := &cobra.Command{
		Use:               "diff <id>",
		Short:             "Show diff between worktree and its base commit",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: worktreeIDsCompletion,
		RunE:              runWorktreeDiff,
	}
	diffSubCmd.Flags().StringVar(&wtBase, "base", "", "Base commit or ref to compare against (defaults to worktree base commit)")
	diffSubCmd.Flags().BoolVar(&wtStatOnly, "stat", false, "Show only diffstat summary")
	diffSubCmd.Flags().BoolVar(&wtCached, "cached", false, "Show staged changes only")

	// Remove
	removeSubCmd := &cobra.Command{
		Use:               "remove <id>",
		Aliases:           []string{"rm", "delete"},
		Short:             "Remove a worktree directory and prune references",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: worktreeIDsCompletion,
		RunE:              runWorktreeRemove,
	}
	removeSubCmd.Flags().BoolVarP(&wtForce, "force", "f", false, "Force removal even if worktree has uncommitted changes")
	removeSubCmd.Flags().BoolVarP(&wtDeleteBranch, "delete-branch", "D", false, "Also delete the associated branch in the repository")

	// Prune
	pruneSubCmd := &cobra.Command{
		Use:   "prune",
		Short: "Prune stale worktree entries from Git and Multigravity manifest",
		Args:  cobra.NoArgs,
		RunE:  runWorktreePrune,
	}

	wtCmd.AddCommand(listSubCmd)
	wtCmd.AddCommand(createSubCmd)
	wtCmd.AddCommand(statusSubCmd)
	wtCmd.AddCommand(diffSubCmd)
	wtCmd.AddCommand(removeSubCmd)
	wtCmd.AddCommand(pruneSubCmd)

	return wtCmd
}

var worktreeCmd = newWorktreeCmd()

func resetWorktreeFlags(cmd *cobra.Command) {
	wtRepo = ""
	wtBranch = ""
	wtBase = ""
	wtDir = ""
	wtProfile = ""
	wtTaskID = ""
	wtForce = false
	wtDeleteBranch = false
	wtStatOnly = false
	wtCached = false
	wtJSON = false
	if cmd != nil {
		_ = cmd.Flags().Set("json", "false")
	}
}

func runWorktreeList(cmd *cobra.Command, args []string) error {
	defer resetWorktreeFlags(cmd)

	list, err := worktree.ListWorktrees(wtRepo)
	if err != nil {
		return err
	}
	if list == nil {
		list = []worktree.Worktree{}
	}

	if wtJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(list)
	}

	if len(list) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No active agent worktrees found in repository.")
		return nil
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s\n", wtBold("MANAGED AGENT WORKTREES:"))
	fmt.Fprintf(out, "%-16s %-10s %-24s %-14s %s\n", "ID", "STATUS", "BRANCH", "PROFILE", "PATH")
	fmt.Fprintf(out, "%s\n", strings.Repeat("─", 80))

	for _, wt := range list {
		statusStr := wt.Status
		switch wt.Status {
		case "active":
			statusStr = wtGreen("active")
		case "dirty":
			statusStr = wtYellow("dirty")
		case "missing":
			statusStr = wtRed("missing")
		}

		profileStr := wt.Profile
		if profileStr == "" {
			profileStr = "—"
		}

		// Show relative path if inside cwd or repo
		displayPath := wt.Path
		if cwd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(cwd, wt.Path); err == nil && !strings.HasPrefix(rel, "..") {
				displayPath = "./" + rel
			}
		}

		fmt.Fprintf(out, "%-16s %-10s %-24s %-14s %s\n",
			wt.ID,
			statusStr,
			wtCyan(wt.Branch),
			profileStr,
			displayPath,
		)
	}
	return nil
}

func runWorktreeCreate(cmd *cobra.Command, args []string) error {
	defer resetWorktreeFlags(cmd)
	id := args[0]

	opts := worktree.CreateOptions{
		ID:         id,
		RepoPath:   wtRepo,
		Branch:     wtBranch,
		BaseCommit: wtBase,
		TargetDir:  wtDir,
		Profile:    wtProfile,
		TaskID:     wtTaskID,
	}

	wt, err := worktree.CreateWorktree(opts)
	if err != nil {
		return err
	}

	if wtJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(wt)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s Created worktree %s\n", wtGreen("✓"), wtBold(wt.ID))
	fmt.Fprintf(out, "  %s %s\n", wtDim("Branch:"), wtCyan(wt.Branch))
	fmt.Fprintf(out, "  %s %s\n", wtDim("Base:  "), wt.BaseCommit)
	fmt.Fprintf(out, "  %s %s\n", wtDim("Path:  "), wt.Path)
	if wt.Profile != "" {
		fmt.Fprintf(out, "  %s %s\n", wtDim("Profile:"), wt.Profile)
	}
	return nil
}

func runWorktreeStatus(cmd *cobra.Command, args []string) error {
	defer resetWorktreeFlags(cmd)
	id := args[0]

	status, err := worktree.GetWorktreeStatus(wtRepo, id)
	if err != nil {
		return err
	}

	if wtJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s %s\n", wtBold("WORKTREE STATUS:"), wtBold(status.Worktree.ID))
	fmt.Fprintf(out, "  %s %s\n", wtDim("Path:        "), status.Worktree.Path)
	fmt.Fprintf(out, "  %s %s\n", wtDim("Branch:      "), wtCyan(status.Worktree.Branch))
	fmt.Fprintf(out, "  %s %s\n", wtDim("Base Commit: "), status.Worktree.BaseCommit)
	if status.Worktree.HeadCommit != "" {
		fmt.Fprintf(out, "  %s %s\n", wtDim("HEAD Commit: "), status.Worktree.HeadCommit)
	}
	if status.Worktree.Profile != "" {
		fmt.Fprintf(out, "  %s %s\n", wtDim("Profile:     "), status.Worktree.Profile)
	}

	cleanStr := wtGreen("clean")
	if !status.IsClean {
		cleanStr = wtYellow("dirty (uncommitted changes)")
	}
	fmt.Fprintf(out, "  %s %s\n", wtDim("Working Tree:"), cleanStr)
	fmt.Fprintf(out, "  %s +%d / -%d\n", wtDim("Commits:     "), status.CommitsAhead, status.CommitsBehind)

	if len(status.ModifiedFiles) > 0 {
		fmt.Fprintf(out, "\n%s (%d):\n", wtYellow("Modified Files"), len(status.ModifiedFiles))
		for _, f := range status.ModifiedFiles {
			fmt.Fprintf(out, "  %s %s\n", wtYellow("~"), f)
		}
	}

	if len(status.UntrackedFiles) > 0 {
		fmt.Fprintf(out, "\n%s (%d):\n", wtRed("Untracked Files"), len(status.UntrackedFiles))
		for _, f := range status.UntrackedFiles {
			fmt.Fprintf(out, "  %s %s\n", wtRed("?"), f)
		}
	}
	return nil
}

func runWorktreeDiff(cmd *cobra.Command, args []string) error {
	defer resetWorktreeFlags(cmd)
	id := args[0]

	opts := worktree.DiffOptions{
		Base:     wtBase,
		StatOnly: wtStatOnly,
		Cached:   wtCached,
	}

	diff, err := worktree.GetWorktreeDiff(wtRepo, id, opts)
	if err != nil {
		return err
	}

	if diff == "" {
		fmt.Fprintln(cmd.OutOrStdout(), "No changes detected.")
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), diff)
	return nil
}

func runWorktreeRemove(cmd *cobra.Command, args []string) error {
	defer resetWorktreeFlags(cmd)
	id := args[0]

	opts := worktree.RemoveOptions{
		Force:        wtForce,
		DeleteBranch: wtDeleteBranch,
	}

	if err := worktree.RemoveWorktree(wtRepo, id, opts); err != nil {
		return err
	}

	if wtJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"success": true,
			"id":      id,
			"removed": true,
		})
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s Removed worktree %s\n", wtGreen("✓"), wtBold(id))
	return nil
}

func runWorktreePrune(cmd *cobra.Command, args []string) error {
	defer resetWorktreeFlags(cmd)

	if err := worktree.PruneWorktrees(wtRepo); err != nil {
		return err
	}

	if wtJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"success": true,
			"pruned":  true,
		})
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s Pruned stale worktree entries\n", wtGreen("✓"))
	return nil
}
