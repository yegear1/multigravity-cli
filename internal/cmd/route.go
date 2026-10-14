package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func newRouteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Recommend optimal profile and model based on measured quotas",
		Long: `Evaluate measured quotas, reset windows, and task complexity across active profiles
to provide deterministic profile and model route recommendations.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				_ = cmd.Flags().Set("json", "false")
				_ = cmd.Flags().Set("weight", "medium")
			}()

			weightStr, _ := cmd.Flags().GetString("weight")
			jsonOutput, _ := cmd.Flags().GetBool("json")

			weight, err := quota.ParseRouteWeight(weightStr)
			if err != nil {
				return err
			}

			servers, err := quota.FindActiveServers("")
			if err != nil {
				return err
			}
			quota.RecordLiveSnapshots(servers)

			plan := quota.RecommendRoute(servers, weight, time.Now())

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(plan)
			}

			if len(servers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No active Antigravity profile or Language Server found in execution.")
				fmt.Fprintln(cmd.OutOrStdout(), "Launch a profile to monitor quota: multigravity <profile>")
				return nil
			}

			renderRoutePlan(cmd.OutOrStdout(), plan)
			return nil
		},
	}

	cmd.Flags().StringP("weight", "w", "medium", "Task complexity weight (light, medium, heavy)")
	cmd.Flags().Bool("json", false, "Output route recommendation in JSON format")

	_ = cmd.RegisterFlagCompletionFunc("weight", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"light", "medium", "heavy"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func renderRoutePlan(out io.Writer, plan quota.RoutePlan) {
	if plan.Recommended != nil {
		rec := plan.Recommended
		fmt.Fprintln(out, "Recommended Route:")
		fmt.Fprintf(out, "  Profile:  %s\n", rec.Profile)
		fmt.Fprintf(out, "  Model:    %s\n", rec.Model)
		fmt.Fprintf(out, "  Weight:   %s\n", rec.Weight)
		if rec.WindowType != "" {
			fmt.Fprintf(out, "  Quota:    %.0f%% remaining (%s window)\n", rec.RemainingFraction*100, rec.WindowType)
		} else {
			fmt.Fprintf(out, "  Quota:    %.0f%% remaining\n", rec.RemainingFraction*100)
		}
		if rec.ResetTime != "" {
			countdown, _ := quota.FormatResetCountdown(rec.ResetTime, time.Now())
			countdown = strings.TrimPrefix(countdown, " | ")
			fmt.Fprintf(out, "  Reset:    %s (%s)\n", rec.ResetTime, countdown)
		}
		fmt.Fprintf(out, "  Reason:   %s\n", rec.Reason)
	} else {
		fmt.Fprintf(out, "No eligible profile found for '%s' weight workload.\n", plan.Weight)
	}

	if len(plan.Candidates) > 0 {
		fmt.Fprintln(out, "\nCandidate Profiles:")
		fmt.Fprintf(out, "  %-15s %-25s %-7s %-8s %-10s %s\n", "PROFILE", "MODEL", "QUOTA", "WINDOW", "STATUS", "REASON")
		fmt.Fprintf(out, "  %s\n", strings.Repeat("-", 80))
		for _, c := range plan.Candidates {
			status := "ineligible"
			if c.Eligible {
				status = "eligible"
			}
			window := c.WindowType
			if window == "" {
				window = "-"
			}
			quotaStr := fmt.Sprintf("%.0f%%", c.RemainingFraction*100)
			if c.Score <= -1000 && c.RemainingFraction == 0 {
				quotaStr = "-"
			}
			fmt.Fprintf(out, "  %-15s %-25s %-7s %-8s %-10s %s\n", c.Profile, c.Model, quotaStr, window, status, c.Reason)
		}
	}
}

var routeCmd = newRouteCmd()

func init() {
	rootCmd.AddCommand(routeCmd)
}
