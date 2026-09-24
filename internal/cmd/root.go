package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

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
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
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
	rootCmd.Flags().SetInterspersed(false)
}
