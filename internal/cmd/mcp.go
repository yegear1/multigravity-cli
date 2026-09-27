package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/mcp"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var (
	mcpJSON       bool
	mcpServeStdio bool
	mcpServeHTTP  bool
	mcpServeHost  string
	mcpServePort  int
)

var mcpCmd = &cobra.Command{
	Use:   "mcp <status|share|isolate|serve> [profile]",
	Short: "Manage MCP configuration for profiles or launch the native MCP server",
	Long: `Manage MCP server configuration and schemas for profiles, or launch
the native Multigravity Model Context Protocol (MCP) server over stdio or HTTP.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		action := args[0]
		if action == "serve" || action == "server" || action == "stdio" {
			return runMCPServer(cmd, args[1:])
		}

		if len(args) != 2 {
			return fmt.Errorf("usage: multigravity mcp <status|share|isolate> <profile> or multigravity mcp serve")
		}

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
			return fmt.Errorf("usage: multigravity mcp <status|share|isolate> <profile> or multigravity mcp serve")
		}
	},
}

func newMCPServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "serve",
		Aliases: []string{"server", "stdio"},
		Short:   "Start the native Multigravity MCP server (stdio or HTTP)",
		Long: `Start the native Multigravity Model Context Protocol (MCP) server.
By default, the server runs over stdio (JSON-RPC line-delimited), suitable for
configuring in Antigravity or Claude Desktop mcp_config.json:

  "multigravity": {
    "command": "multigravity",
    "args": ["mcp", "serve"]
  }

To run as a standalone HTTP server, provide --http or --port.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPServer(cmd, args)
		},
	}
	cmd.Flags().BoolVar(&mcpServeStdio, "stdio", false, "Force stdio transport (default)")
	cmd.Flags().BoolVar(&mcpServeHTTP, "http", false, "Run standalone HTTP server instead of stdio")
	cmd.Flags().StringVar(&mcpServeHost, "host", "127.0.0.1", "Host address to bind to in HTTP mode")
	cmd.Flags().IntVarP(&mcpServePort, "port", "p", 8990, "Port to listen on in HTTP mode")
	return cmd
}

func newMCPServerRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mcp-server",
		Aliases: []string{"mcp-stdio"},
		Short:   "Start the native Multigravity MCP server (alias for 'multigravity mcp serve')",
		Long: `Start the native Multigravity Model Context Protocol (MCP) server over stdio or HTTP.
This is a direct alias for 'multigravity mcp serve'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPServer(cmd, args)
		},
	}
	cmd.Flags().BoolVar(&mcpServeStdio, "stdio", false, "Force stdio transport (default)")
	cmd.Flags().BoolVar(&mcpServeHTTP, "http", false, "Run standalone HTTP server instead of stdio")
	cmd.Flags().StringVar(&mcpServeHost, "host", "127.0.0.1", "Host address to bind to in HTTP mode")
	cmd.Flags().IntVarP(&mcpServePort, "port", "p", 8990, "Port to listen on in HTTP mode")
	return cmd
}

func runMCPServer(cmd *cobra.Command, args []string) error {
	ctx, stopCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopCancel()

	srv := mcp.NewDefaultServer()

	// If --http was explicitly passed or custom port without --stdio
	if mcpServeHTTP || (mcpServePort != 8990 && !mcpServeStdio) {
		fmt.Fprintf(os.Stderr, "🚀 Multigravity MCP HTTP Server listening at http://%s:%d/mcp\n", mcpServeHost, mcpServePort)
		fmt.Fprintf(os.Stderr, "   SSE endpoint: http://%s:%d/mcp/sse\n", mcpServeHost, mcpServePort)
		return srv.StartHTTPServer(ctx, mcpServeHost, mcpServePort)
	}

	// Default stdio transport
	return srv.ServeStdio(ctx, cmd.InOrStdin(), cmd.OutOrStdout())
}

func init() {
	mcpCmd.Flags().BoolVar(&mcpJSON, "json", false, "Output MCP status in JSON format")
	mcpCmd.AddCommand(newMCPServeCmd())
}
