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
		inUse, _ := cmd.Flags().GetBool("in-use")
		openFlag, _ := cmd.Flags().GetBool("open")
		if openFlag {
			inUse = true
		}
		jsonOut, _ := cmd.Flags().GetBool("json")

		filter := chat.FilterAll
		if inUse {
			filter = chat.FilterInUse
		} else if active && !archived {
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
		_ = cmd.Flags().Set("in-use", "false")
		_ = cmd.Flags().Set("open", "false")
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
		skipInUse, _ := cmd.Flags().GetBool("skip-in-use")
		safeOnly, _ := cmd.Flags().GetBool("safe-only")
		return chat.ExportConversationsWithOptions(profile, out, chat.ExportOptions{
			SkipInUse: skipInUse || safeOnly,
		})
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		_ = cmd.Flags().Set("skip-in-use", "false")
		_ = cmd.Flags().Set("safe-only", "false")
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
		skipInUse, _ := cmd.Flags().GetBool("skip-in-use")
		safeOnly, _ := cmd.Flags().GetBool("safe-only")
		return chat.SyncConversationsWithOptions(args[0], args[1], chat.SyncOptions{
			SkipInUse: skipInUse || safeOnly,
		})
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		_ = cmd.Flags().Set("skip-in-use", "false")
		_ = cmd.Flags().Set("safe-only", "false")
	},
}

func init() {
	aiListCmd.Flags().Bool("json", false, "Output conversations in JSON format")
	aiListCmd.Flags().Bool("active", false, "Show only active conversations")
	aiListCmd.Flags().Bool("archived", false, "Show only archived conversations")
	aiListCmd.Flags().Bool("in-use", false, "Show only conversations currently open / in use")
	aiListCmd.Flags().Bool("open", false, "Alias for --in-use")

	aiExportCmd.Flags().Bool("skip-in-use", false, "Skip in-use conversations to allow safe export while profile is running")
	aiExportCmd.Flags().Bool("safe-only", false, "Alias for --skip-in-use")

	aiSyncCmd.Flags().Bool("skip-in-use", false, "Skip in-use conversations to allow safe sync while profile is running")
	aiSyncCmd.Flags().Bool("safe-only", false, "Alias for --skip-in-use")

	aiCmd.AddCommand(aiListCmd)
	aiCmd.AddCommand(aiExportCmd)
	aiCmd.AddCommand(aiImportCmd)
	aiCmd.AddCommand(aiSyncCmd)

	// Reuse quota and prime subcommands under ai
	aiCmd.AddCommand(newQuotaCmd())
	aiCmd.AddCommand(newPrimeCmd())

	rootCmd.AddCommand(aiCmd)
}
