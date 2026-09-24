package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp <status|share|isolate> <profile>",
	Short: "Manage MCP server configuration and schemas for a profile",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		action := args[0]
		profName := args[1]

		switch action {
		case "status":
			msg, err := profile.McpStatus(profName)
			if err != nil {
				return err
			}
			cmd.Println(msg)
			return nil
		case "share":
			msgs, err := profile.McpShare(profName)
			if err != nil {
				return err
			}
			for _, m := range msgs {
				cmd.Println(m)
			}
			return nil
		case "isolate":
			msgs, err := profile.McpIsolate(profName)
			if err != nil {
				return err
			}
			for _, m := range msgs {
				cmd.Println(m)
			}
			return nil
		default:
			return fmt.Errorf("usage: multigravity mcp <status|share|isolate> <profile>")
		}
	},
}


