package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/idelog"
)

func newLogsCmd() *cobra.Command {
	var (
		tail   int
		follow bool
		asJSON bool
	)

	cmd := &cobra.Command{
		Use:   "logs <profile>",
		Short: "Show or follow the graphical IDE log for a profile",
		Long: `Show or follow the Antigravity graphical IDE log (Electron main process).

The file is <user-data-dir>/logs/main.log. Session secrets written by the IDE
are redacted. The managed language server stays on 'multigravity headless logs'
and task output stays on 'multigravity dispatch logs'.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: profileArgsCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				_ = cmd.Flags().Set("tail", "100")
				_ = cmd.Flags().Set("follow", "false")
				_ = cmd.Flags().Set("json", "false")
			}()

			profileName := args[0]
			out := cmd.OutOrStdout()
			if follow {
				ch, err := idelog.Follow(cmd.Context(), profileName, tail)
				if err != nil {
					return err
				}
				enc := json.NewEncoder(out)
				for chunk := range ch {
					if asJSON {
						if err := enc.Encode(map[string]string{"chunk": chunk}); err != nil {
							return err
						}
						continue
					}
					fmt.Fprint(out, chunk)
				}
				return nil
			}

			snap, err := idelog.Read(profileName, tail)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(snap)
			}
			fmt.Fprint(out, snap.Logs)
			return nil
		},
	}
	cmd.Flags().IntVarP(&tail, "tail", "n", 100, "Number of lines to show from the end")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Stream new IDE log lines until interrupted")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Emit a JSON snapshot, or NDJSON chunks with --follow")
	return cmd
}

var logsCmd = newLogsCmd()
