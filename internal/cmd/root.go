package cmd

import (
	"fmt"

	"os"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/tui"
)

var isInteractiveTerminal = func() bool {
	return tui.IsInteractive(os.Stdin.Fd(), os.Stdout.Fd())
}

func profileArgsCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		profiles, err := profile.ListProfiles()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return profiles, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

var rootCmd = &cobra.Command{
	Use:   "multigravity [command|profile]",
	Short: "Multigravity CLI — Isolated profile manager for Antigravity IDE",
	Long: `Multigravity manages fully isolated profiles for the Google Antigravity IDE.
It preserves isolation of workspaces, settings, AI chats, and tokens, while allowing
granular sharing of MCP servers, skills, read-only permissions, and developer credentials.`,
	Version: config.Version,
	Args:    cobra.ArbitraryArgs,
	FParseErrWhitelist: cobra.FParseErrWhitelist{
		UnknownFlags: true,
	},
	SilenceUsage: true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			profiles, _ := profile.ListProfiles()
			return profiles, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveDefault
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			if isInteractiveTerminal() {
				return tui.RunMenu(cmd.InOrStdin(), cmd.OutOrStdout(), nil)
			}
			_ = cmd.Help()
			return fmt.Errorf("no command or profile specified")
		}

		firstArg := args[0]
		// Invariant: If first argument is not an internal command, interpret as profile name
		if err := config.ValidateProfileName(firstArg); err == nil {
			return profile.LaunchProfile(firstArg, args[1:])
		}

		return fmt.Errorf("unknown command or invalid profile name: %s", firstArg)
	},
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(renameCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(colorCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(allowReadonlyCmd)
	rootCmd.AddCommand(ghCmd)
	rootCmd.AddCommand(cloneCmd)
	rootCmd.AddCommand(templateCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(serveCmd)

	// Shell completion dynamic profile args
	stopCmd.ValidArgsFunction = profileArgsCompletion
	restartCmd.ValidArgsFunction = profileArgsCompletion
	deleteCmd.ValidArgsFunction = profileArgsCompletion
	renameCmd.ValidArgsFunction = profileArgsCompletion
	cloneCmd.ValidArgsFunction = profileArgsCompletion
	exportCmd.ValidArgsFunction = profileArgsCompletion
	colorCmd.ValidArgsFunction = profileArgsCompletion
	quotaCmd.ValidArgsFunction = profileArgsCompletion
	primeCmd.ValidArgsFunction = profileArgsCompletion

	cleanCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			profiles, _ := profile.ListProfiles()
			return append(profiles, "--all"), cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	mcpCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{"status", "share", "isolate"}, cobra.ShellCompDirectiveNoFileComp
		}
		if len(args) == 1 {
			profiles, _ := profile.ListProfiles()
			return profiles, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	skillsCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{"status", "share", "isolate"}, cobra.ShellCompDirectiveNoFileComp
		}
		if len(args) == 1 {
			profiles, _ := profile.ListProfiles()
			return profiles, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	configCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{"status", "share", "isolate", "seed"}, cobra.ShellCompDirectiveNoFileComp
		}
		if len(args) == 1 {
			profiles, _ := profile.ListProfiles()
			return append(profiles, "--all", "--host"), cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ghCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{"status", "share", "isolate"}, cobra.ShellCompDirectiveNoFileComp
		}
		if len(args) == 1 {
			profiles, _ := profile.ListProfiles()
			return profiles, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	rootCmd.Flags().SetInterspersed(false)
}
