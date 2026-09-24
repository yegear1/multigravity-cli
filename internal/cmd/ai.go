package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/chat"
)

var aiCmd = &cobra.Command{
	Use:   "ai <export|import|sync|list|quota|prime> [args...]",
	Short: "Manage AI conversations, quota telemetry, and priming",
}

var aiListCmd = &cobra.Command{
	Use:   "list <profile>",
	Short: "List AI conversations in a profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return chat.ListConversationsWriter(cmd.OutOrStdout(), args[0])
	},
}

var aiExportCmd = &cobra.Command{
	Use:   "export <profile> [path.tar.gz]",
	Short: "Export AI conversations and artifacts (credentials sanitized)",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		profile := args[0]
		out := ""
		if len(args) > 1 {
			out = args[1]
		}
		return chat.ExportConversations(profile, out)
	},
}

var aiImportCmd = &cobra.Command{
	Use:   "import <archive.tar.gz> <profile>",
	Short: "Import AI conversations into a profile",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return chat.ImportConversations(args[0], args[1])
	},
}

var aiSyncCmd = &cobra.Command{
	Use:   "sync <source_profile> <target_profile>",
	Short: "Synchronize AI conversations between two profiles",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] == args[1] {
			return fmt.Errorf("source and target profiles cannot be the same")
		}
		return chat.SyncConversations(args[0], args[1])
	},
}

func init() {
	aiCmd.AddCommand(aiListCmd)
	aiCmd.AddCommand(aiExportCmd)
	aiCmd.AddCommand(aiImportCmd)
	aiCmd.AddCommand(aiSyncCmd)

	// Reuse quota and prime subcommands under ai
	aiCmd.AddCommand(newQuotaCmd())
	aiCmd.AddCommand(newPrimeCmd())

	rootCmd.AddCommand(aiCmd)
}
