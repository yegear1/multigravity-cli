package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var mcpJSON bool

var mcpCmd = &cobra.Command{
	Use:   "mcp <status|share|isolate> <profile>",
	Short: "Manage MCP server configuration and schemas for a profile",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		action := args[0]
		profName := args[1]

		switch action {
		case "status":
			if mcpJSON {
				st, err := profile.GetMcpStatus(profName)
				if err != nil {
					return err
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(st)
			}
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

func init() {
	mcpCmd.Flags().BoolVar(&mcpJSON, "json", false, "Output MCP status in JSON format")
}



