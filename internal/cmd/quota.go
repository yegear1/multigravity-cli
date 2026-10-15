package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func newQuotaCmd() *cobra.Command {
	var quotaJSON bool

	cmd := &cobra.Command{
		Use:   "quota [profile]",
		Short: "Show AI token limits, usage percentage, and reset time",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetProf := ""
			if len(args) > 0 {
				targetProf = args[0]
				if err := config.ValidateProfileName(targetProf); err != nil {
					return err
				}
				if !profile.ProfileExists(targetProf) {
					return fmt.Errorf("profile '%s' does not exist", targetProf)
				}
			}

			servers, err := quota.FindActiveServers(targetProf)
			if err != nil {
				return err
			}
			quota.RecordLiveSnapshots(servers)

			if quotaJSON {
				summary, err := quota.BuildQuotaSummary(targetProf, servers)
				if err != nil {
					return err
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(summary)
			}

			if len(servers) == 0 {
				if targetProf != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Profile '%s' is not running.\n", targetProf)
					fmt.Fprintf(cmd.OutOrStdout(), "To check quota and token consumption, launch it first: multigravity %s\n", targetProf)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No active Antigravity profile or Language Server found in execution.")
					fmt.Fprintln(cmd.OutOrStdout(), "Launch a profile to monitor quota: multigravity <profile>")
				}
				return nil
			}

			quota.RenderQuotaStatus(cmd.OutOrStdout(), servers)
			return nil
		},
	}

	cmd.Flags().BoolVar(&quotaJSON, "json", false, "Output quota summary in JSON format")
	cmd.AddCommand(newQuotaHistoryCmd())
	cmd.AddCommand(newQuotaCalibrationCmd())
	cmd.AddCommand(newQuotaBurnCmd())
	cmd.AddCommand(newQuotaThrottleCmd())
	return cmd
}

func newQuotaHistoryCmd() *cobra.Command {
	var (
		asJSON bool
		since  string
		until  string
		source string
		limit  int
	)

	cmd := &cobra.Command{
		Use:   "history [profile]",
		Short: "Show the stored quota and token time series",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			limitRaw := ""
			if cmd.Flags().Changed("limit") {
				limitRaw = strconv.Itoa(limit)
			}
			q, err := quota.ParseHistoryQuery(since, until, source, limitRaw)
			if err != nil {
				return err
			}

			var payload any
			if len(args) == 1 {
				name := args[0]
				if err := config.ValidateProfileName(name); err != nil {
					return err
				}
				series, err := quota.LoadSeries(name, q)
				if err != nil {
					return err
				}
				payload = series
				if !asJSON {
					writeQuotaHistoryText(cmd, series)
					return nil
				}
			} else {
				fleet, err := quota.LoadFleetSeries(q)
				if err != nil {
					return err
				}
				payload = fleet
				if !asJSON {
					if len(fleet.Profiles) == 0 {
						fmt.Fprintln(cmd.OutOrStdout(), "No quota history recorded yet.")
						return nil
					}
					for i := range fleet.Profiles {
						if i > 0 {
							fmt.Fprintln(cmd.OutOrStdout())
						}
						writeQuotaHistoryText(cmd, &fleet.Profiles[i])
					}
					return nil
				}
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(payload)
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the time series in JSON format")
	cmd.Flags().StringVar(&since, "since", "", "Start of the window (duration such as 24h or 7d, or RFC3339)")
	cmd.Flags().StringVar(&until, "until", "", "End of the window (duration or RFC3339)")
	cmd.Flags().StringVar(&source, "source", "", "Filter by source: quota, gateway, or headless")
	cmd.Flags().IntVar(&limit, "limit", 500, "Maximum samples to return")
	return cmd
}

func writeQuotaHistoryText(cmd *cobra.Command, series *quota.Series) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s  samples=%d  tokens=%d\n", series.Profile, series.Summary.Samples, series.Summary.TotalTokens)
	if len(series.Samples) == 0 {
		fmt.Fprintln(out, "  (no samples in this window)")
		return
	}
	for _, sample := range series.Samples {
		ts := sample.Timestamp.UTC().Format("2006-01-02T15:04:05Z")
		switch {
		case sample.Tokens != nil:
			fmt.Fprintf(out, "  %s  %-8s  tokens=%d", ts, sample.Source, sample.Tokens.TotalTokens)
			if sample.Tokens.Model != "" {
				fmt.Fprintf(out, "  model=%s", sample.Tokens.Model)
			}
			if sample.Tokens.Estimated {
				fmt.Fprint(out, "  estimated")
			}
			fmt.Fprintln(out)
		case len(sample.Buckets) > 0:
			fmt.Fprintf(out, "  %s  %-8s", ts, sample.Source)
			for _, bucket := range sample.Buckets {
				fmt.Fprintf(out, "  %s=%.0f%%", bucket.BucketID, bucket.RemainingFraction*100)
			}
			fmt.Fprintln(out)
		default:
			fmt.Fprintf(out, "  %s  %s\n", ts, sample.Source)
		}
	}
}

func newQuotaCalibrationCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "calibration",
		Short: "Show automated token estimation calibration heuristics and metrics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			stats := quota.GetCalibrationStats()
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(stats)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Automated Token Estimation Calibration:")
			fmt.Fprintf(out, "  Observations:     %d samples\n", stats.Samples)
			fmt.Fprintf(out, "  Effective Ratio:  %.2f runes/token (baseline: %.2f)\n", stats.RunesPerToken, quota.DefaultRunesPerToken)
			if stats.PromptRunesPerToken > 0 {
				fmt.Fprintf(out, "  Prompt Ratio:     %.2f runes/token\n", stats.PromptRunesPerToken)
			}
			if stats.CompletionRunesPerToken > 0 {
				fmt.Fprintf(out, "  Completion Ratio: %.2f runes/token\n", stats.CompletionRunesPerToken)
			}
			fmt.Fprintf(out, "  Cumulative Data:  %d runes, %d tokens\n", stats.TotalRunes, stats.TotalTokens)
			if stats.LastCalibrated != "" {
				fmt.Fprintf(out, "  Last Calibrated:  %s\n", stats.LastCalibrated)
			}

			if len(stats.Models) > 0 {
				fmt.Fprintln(out, "\nPer-Model Calibrated Ratios:")
				for model, mc := range stats.Models {
					fmt.Fprintf(out, "  - %-24s %d samples, %.2f runes/token\n", model, mc.Samples, mc.RunesPerToken)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output calibration stats in JSON format")
	return cmd
}

var quotaCmd = newQuotaCmd()

func init() {
	rootCmd.AddCommand(quotaCmd)
}
