package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/alert"
)

func newAlertsCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:     "alerts [profile]",
		Short:   "Report critical quota, quota drops, and reaped headless processes",
		Aliases: []string{"alert"},
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			report, err := alert.Evaluate(name)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			if len(report.Alerts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No alerts.")
				return nil
			}
			for _, item := range report.Alerts {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s", item.Profile, item.Kind)
				if item.BucketID != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s", item.BucketID)
				}
				if item.PID != 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "  pid=%d", item.PID)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", item.Message)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output alerts in JSON format")
	return cmd
}

var alertsCmd = newAlertsCmd()

func init() {
	rootCmd.AddCommand(alertsCmd)
}
