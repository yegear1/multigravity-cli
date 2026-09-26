package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/workspace"
)

var (
	workspaceJSON   bool
	workspaceActive bool
)

var workspaceCmd = &cobra.Command{
	Use:     "workspace",
	Aliases: []string{"ws", "workspaces"},
	Short:   "Detect and map workspaces and active Git repositories per profile",
	Long: `Inspect, map and list workspaces and projects configured across Antigravity profiles.
Identifies active repositories in running profiles and correlates current working directories.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return workspaceListCmd.RunE(cmd, args)
	},
}

var workspaceListCmd = &cobra.Command{
	Use:     "list [profile]",
	Aliases: []string{"ls"},
	Short:   "List workspaces across all profiles or a specific profile",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		defer func() {
			_ = cmd.Flags().Set("json", "false")
			_ = cmd.Flags().Set("active", "false")
			workspaceJSON = false
			workspaceActive = false
		}()

		var list []workspace.Workspace
		var err error

		if len(args) == 1 {
			targetProfile := args[0]
			list, err = workspace.GetProfileWorkspaces(targetProfile)
			if err != nil {
				return err
			}
		} else {
			list, err = workspace.GetAllWorkspaces()
			if err != nil {
				return err
			}
		}

		if workspaceActive {
			var filtered []workspace.Workspace
			for _, ws := range list {
				if ws.IsActive {
					filtered = append(filtered, ws)
				}
			}
			list = filtered
		}

		if workspaceJSON {
			if list == nil {
				list = []workspace.Workspace{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(list)
		}

		if len(list) == 0 {
			if workspaceActive {
				cmd.Println("No active workspaces found.")
			} else {
				cmd.Println("No workspaces found.")
			}
			return nil
		}

		cmd.Printf("%-16s %-24s %-16s %-20s %-8s %s\n", "PROFILE", "WORKSPACE", "BRANCH", "GIT STATUS", "ACTIVE", "PATH")
		cmd.Printf("%-16s %-24s %-16s %-20s %-8s %s\n", "-------", "---------", "------", "----------", "------", "----")

		green := color.New(color.FgGreen).SprintFunc()
		yellow := color.New(color.FgYellow).SprintFunc()

		for _, ws := range list {
			activeStr := "-"
			if ws.IsActive {
				activeStr = green("yes")
			}

			branchStr := "-"
			gitStatusStr := "-"
			if ws.Git != nil && ws.Git.IsGitRepo {
				if ws.Git.Branch != "" {
					branchStr = ws.Git.Branch
				}
				if ws.Git.IsClean {
					gitStatusStr = green("clean")
				} else {
					gitStatusStr = yellow(fmt.Sprintf("dirty (+%d ~%d)", ws.Git.UntrackedFiles, ws.Git.ModifiedFiles))
				}
			} else if ws.DefaultBranch != "" {
				branchStr = ws.DefaultBranch
			}

			pathStr := ws.Path
			if pathStr == "" {
				pathStr = ws.URI
			}

			cmd.Printf("%-16s %-24s %-16s %-20s %-8s %s\n",
				ws.Profile, ws.Name, branchStr, gitStatusStr, activeStr, pathStr)
		}

		return nil
	},
}

var workspaceActiveCmd = &cobra.Command{
	Use:   "active",
	Short: "Show currently active workspaces in running profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		defer func() {
			_ = cmd.Flags().Set("json", "false")
			workspaceJSON = false
		}()

		list, err := workspace.GetActiveWorkspaces()
		if err != nil {
			return err
		}

		if workspaceJSON {
			if list == nil {
				list = []workspace.Workspace{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(list)
		}

		if len(list) == 0 {
			cmd.Println("No active workspaces currently in use by running profiles.")
			return nil
		}

		cmd.Printf("%-16s %-24s %-16s %-20s %s\n", "PROFILE", "WORKSPACE", "BRANCH", "GIT STATUS", "PATH")
		cmd.Printf("%-16s %-24s %-16s %-20s %s\n", "-------", "---------", "------", "----------", "----")

		green := color.New(color.FgGreen).SprintFunc()
		yellow := color.New(color.FgYellow).SprintFunc()

		for _, ws := range list {
			branchStr := "-"
			gitStatusStr := "-"
			if ws.Git != nil && ws.Git.IsGitRepo {
				if ws.Git.Branch != "" {
					branchStr = ws.Git.Branch
				}
				if ws.Git.IsClean {
					gitStatusStr = green("clean")
				} else {
					gitStatusStr = yellow(fmt.Sprintf("dirty (+%d ~%d)", ws.Git.UntrackedFiles, ws.Git.ModifiedFiles))
				}
			}

			pathStr := ws.Path
			if pathStr == "" {
				pathStr = ws.URI
			}

			cmd.Printf("%-16s %-24s %-16s %-20s %s\n",
				ws.Profile, ws.Name, branchStr, gitStatusStr, pathStr)
		}

		return nil
	},
}

var workspaceCurrentCmd = &cobra.Command{
	Use:     "current",
	Aliases: []string{"here", "pwd"},
	Short:   "Detect which profile and workspace own the current working directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		defer func() {
			_ = cmd.Flags().Set("json", "false")
			workspaceJSON = false
		}()

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		matches, err := workspace.GetWorkspaceByPath(cwd)
		if err != nil {
			return err
		}

		if workspaceJSON {
			if matches == nil {
				matches = []workspace.Workspace{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(matches)
		}

		if len(matches) == 0 {
			cmd.Printf("No profile workspace configured for: %s\n", cwd)
			return nil
		}

		green := color.New(color.FgGreen).SprintFunc()
		cyan := color.New(color.FgCyan).SprintFunc()

		cmd.Printf("Current Directory: %s\n\n", cwd)
		for _, m := range matches {
			activeBadge := ""
			if m.IsActive {
				activeBadge = " " + green("[ACTIVE]")
			}
			cmd.Printf("● Profile: %s%s\n", cyan(m.Profile), activeBadge)
			cmd.Printf("  Workspace: %s (ID: %s)\n", m.Name, m.ID)
			cmd.Printf("  Mapped Path: %s\n", m.Path)
			if m.Git != nil && m.Git.IsGitRepo {
				statusStr := "clean"
				if !m.Git.IsClean {
					statusStr = fmt.Sprintf("dirty (+%d untracked, ~%d modified)", m.Git.UntrackedFiles, m.Git.ModifiedFiles)
				}
				cmd.Printf("  Git: branch=%s, status=%s\n", m.Git.Branch, statusStr)
				if m.Git.RemoteURL != "" {
					cmd.Printf("  Remote: %s\n", m.Git.RemoteURL)
				}
			}
			cmd.Println()
		}

		return nil
	},
}

var workspaceShowCmd = &cobra.Command{
	Use:     "show <profile> <workspace-name-or-id>",
	Aliases: []string{"get", "info"},
	Short:   "Inspect detailed configuration and Git telemetry of a workspace",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		defer func() {
			_ = cmd.Flags().Set("json", "false")
			workspaceJSON = false
		}()

		targetProfile := args[0]
		targetWorkspace := args[1]

		list, err := workspace.GetProfileWorkspaces(targetProfile)
		if err != nil {
			return err
		}

		var matched *workspace.Workspace
		for i := range list {
			if strings.EqualFold(list[i].Name, targetWorkspace) || list[i].ID == targetWorkspace {
				matched = &list[i]
				break
			}
		}

		if matched == nil {
			return fmt.Errorf("workspace %q not found in profile %q", targetWorkspace, targetProfile)
		}

		if workspaceJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(matched)
		}

		green := color.New(color.FgGreen).SprintFunc()
		cyan := color.New(color.FgCyan).SprintFunc()

		cmd.Printf("Workspace Details:\n")
		cmd.Printf("  Name:            %s\n", cyan(matched.Name))
		cmd.Printf("  ID:              %s\n", matched.ID)
		cmd.Printf("  Profile:         %s\n", matched.Profile)
		activeStr := "no"
		if matched.IsActive {
			activeStr = green("yes (currently open in running Antigravity instance)")
		}
		cmd.Printf("  Active:          %s\n", activeStr)
		cmd.Printf("  Workspace Only:  %v\n", matched.IsWorkspaceOnly)
		cmd.Printf("  Path:            %s\n", matched.Path)
		cmd.Printf("  URI:             %s\n", matched.URI)
		if matched.DefaultBranch != "" {
			cmd.Printf("  Default Branch:  %s\n", matched.DefaultBranch)
		}

		if matched.Git != nil && matched.Git.IsGitRepo {
			cmd.Printf("\nGit Telemetry:\n")
			cmd.Printf("  Is Git Repo:     true\n")
			cmd.Printf("  Branch:          %s\n", matched.Git.Branch)
			if matched.Git.RemoteURL != "" {
				cmd.Printf("  Remote URL:      %s\n", matched.Git.RemoteURL)
			}
			if matched.Git.CommitHash != "" {
				cmd.Printf("  Latest Commit:   %s - %s\n", matched.Git.CommitHash, matched.Git.CommitMessage)
			}
			cleanStr := green("clean (no unstaged or untracked changes)")
			if !matched.Git.IsClean {
				cleanStr = color.YellowString("dirty (%d modified, %d untracked)", matched.Git.ModifiedFiles, matched.Git.UntrackedFiles)
			}
			cmd.Printf("  Status:          %s\n", cleanStr)
		}

		cmd.Printf("\nAgent Settings:\n")
		cmd.Printf("  Sandbox Mode:    %v\n", matched.Settings.SandboxMode)
		if matched.Settings.FileAccessPolicy != "" {
			cmd.Printf("  File Access:     %s\n", matched.Settings.FileAccessPolicy)
		}
		if matched.Settings.AutoExecutionPolicy != "" {
			cmd.Printf("  Auto Execution:  %s\n", matched.Settings.AutoExecutionPolicy)
		}

		return nil
	},
}

func init() {
	workspaceListCmd.Flags().BoolVar(&workspaceJSON, "json", false, "Output in JSON format")
	workspaceListCmd.Flags().BoolVar(&workspaceActive, "active", false, "Show only active workspaces")

	workspaceActiveCmd.Flags().BoolVar(&workspaceJSON, "json", false, "Output in JSON format")
	workspaceCurrentCmd.Flags().BoolVar(&workspaceJSON, "json", false, "Output in JSON format")
	workspaceShowCmd.Flags().BoolVar(&workspaceJSON, "json", false, "Output in JSON format")

	workspaceCmd.AddCommand(workspaceListCmd)
	workspaceCmd.AddCommand(workspaceActiveCmd)
	workspaceCmd.AddCommand(workspaceCurrentCmd)
	workspaceCmd.AddCommand(workspaceShowCmd)

	// Shell autocompletion for profile names
	_ = workspaceListCmd.RegisterFlagCompletionFunc("profile", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		names, _ := profile.ListProfiles()
		return names, cobra.ShellCompDirectiveNoFileComp
	})
	workspaceShowCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			names, _ := profile.ListProfiles()
			return names, cobra.ShellCompDirectiveNoFileComp
		}
		if len(args) == 1 {
			wsList, _ := workspace.GetProfileWorkspaces(args[0])
			var wsNames []string
			for _, ws := range wsList {
				wsNames = append(wsNames, ws.Name)
			}
			return wsNames, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}
