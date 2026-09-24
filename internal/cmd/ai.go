package cmd

import (
	"encoding/json"
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
		active, _ := cmd.Flags().GetBool("active")
		archived, _ := cmd.Flags().GetBool("archived")
		jsonOut, _ := cmd.Flags().GetBool("json")

		filter := chat.FilterAll
		if active && !archived {
			filter = chat.FilterActive
		} else if archived && !active {
			filter = chat.FilterArchived
		}

		if jsonOut {
			convs, err := chat.GetFilteredConversations(args[0], filter)
			if err != nil {
				return err
			}
			if convs == nil {
				convs = []chat.ConversationInfo{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(convs)
		}
		return chat.ListConversationsFilter(cmd.OutOrStdout(), args[0], filter)
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		_ = cmd.Flags().Set("active", "false")
		_ = cmd.Flags().Set("archived", "false")
		_ = cmd.Flags().Set("json", "false")
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
	aiListCmd.Flags().Bool("json", false, "Output conversations in JSON format")
	aiListCmd.Flags().Bool("active", false, "Show only active conversations")
	aiListCmd.Flags().Bool("archived", false, "Show only archived conversations")

	aiCmd.AddCommand(aiListCmd)
	aiCmd.AddCommand(aiExportCmd)
	aiCmd.AddCommand(aiImportCmd)
	aiCmd.AddCommand(aiSyncCmd)

	// Reuse quota and prime subcommands under ai
	aiCmd.AddCommand(newQuotaCmd())
	aiCmd.AddCommand(newPrimeCmd())

	rootCmd.AddCommand(aiCmd)
}
