package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/server"
)

var (
	serveHost string
	servePort int
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start local HTTP REST API server for Aggregator and UI clients",
	Long: `Start a local HTTP REST API server exposing profile management,
telemetry, doctor diagnostics, and AI conversation queries for Aggregator and UI clients.

By default, the server binds to 127.0.0.1:8989 for loopback security.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Environment variable fallback for port
		if servePort == 8989 {
			if envPort := os.Getenv("MULTIGRAVITY_PORT"); envPort != "" {
				if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
					servePort = p
				}
			}
		}

		srv := server.NewServer(server.Config{
			Host:      serveHost,
			Port:      servePort,
			Version:   config.Version,
			StartTime: time.Now(),
		})

		stopCtx, stopCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stopCancel()

		errCh := make(chan error, 1)
		go func() {
			errCh <- srv.Start()
		}()

		fmt.Fprintf(cmd.OutOrStdout(), "🚀 Multigravity API Server listening at http://%s\n", srv.Addr())
		fmt.Fprintf(cmd.OutOrStdout(), "   Ready for Aggregator, UI, and agent queries.\n")
		fmt.Fprintf(cmd.OutOrStdout(), "   Press Ctrl+C to stop.\n")

		select {
		case <-stopCtx.Done():
			fmt.Fprintln(cmd.OutOrStdout(), "\nShutting down server gracefully...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			return srv.Shutdown(shutdownCtx)
		case err := <-errCh:
			return err
		}
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveHost, "host", "127.0.0.1", "Host address to bind to")
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8989, "Port to listen on (or MULTIGRAVITY_PORT env)")
}
