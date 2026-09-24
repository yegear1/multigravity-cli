package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/config"
)

var rootCmd = &cobra.Command{
	Use:   "multigravity [command|profile]",
	Short: "Multigravity CLI — Isolated profile manager for Antigravity IDE",
	Long: `Multigravity manages fully isolated profiles for the Google Antigravity IDE.
It preserves isolation of workspaces, settings, AI chats, and tokens, while allowing
granular sharing of MCP servers, skills, read-only permissions, and developer credentials.`,
	Version: config.Version,
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		firstArg := args[0]
		// Invariant: If first argument is not an internal command, interpret as profile name
		if err := config.ValidateProfileName(firstArg); err == nil {
			fmt.Printf("Launching profile %q with args: %v\n", firstArg, args[1:])
			return nil
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
}
