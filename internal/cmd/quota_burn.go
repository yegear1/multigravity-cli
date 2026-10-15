package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func newQuotaBurnCmd() *cobra.Command {
	var (
		asJSON bool
		window string
	)

	cmd := &cobra.Command{
		Use:   "burn [profile]",
		Short: "Show token burn rates and velocity over sliding windows",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				asJSON = false
				window = "1h"
			}()
			winDur := time.Hour
			if window != "" {
				d, err := time.ParseDuration(window)
				if err != nil {
					return fmt.Errorf("invalid window duration %q: %w", window, err)
				}
				if d <= 0 {
					return fmt.Errorf("window duration must be greater than zero")
				}
				winDur = d
			}

			now := time.Now()
			out := cmd.OutOrStdout()

			if len(args) == 1 {
				target := args[0]
				if err := config.ValidateProfileName(target); err != nil {
					return err
				}
				if !profile.ProfileExists(target) {
					return fmt.Errorf("profile %q does not exist", target)
				}
				rate, err := quota.CalculateTokenBurnRate(target, winDur, now)
				if err != nil {
					return err
				}
				if asJSON {
					enc := json.NewEncoder(out)
					enc.SetIndent("", "  ")
					return enc.Encode(rate)
				}
				fmt.Fprintf(out, "Token Burn Rate for Profile %q (window: %s):\n", target, winDur)
				fmt.Fprintf(out, "  Tokens Burned:     %d tokens\n", rate.TokensBurned)
				fmt.Fprintf(out, "  Burn Velocity:     %.2f tokens/min (%.0f tokens/hr)\n", rate.TokensPerMinute, rate.TokensPerHour)
				fmt.Fprintf(out, "  Recorded Samples:  %d events\n", rate.SampleCount)
				if rate.FirstSample != "" {
					fmt.Fprintf(out, "  Active Period:     %s -> %s\n", rate.FirstSample, rate.LastSample)
				}
				return nil
			}

			fleet, err := quota.CalculateFleetBurnRates(winDur, now)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(fleet)
			}

			if len(fleet) == 0 {
				fmt.Fprintln(out, "No profiles found.")
				return nil
			}

			var names []string
			for name := range fleet {
				names = append(names, name)
			}
			sort.Strings(names)

			fmt.Fprintf(out, "Fleet Token Burn Rates (window: %s):\n", winDur)
			fmt.Fprintf(out, "%-20s %12s %14s %14s %8s\n", "PROFILE", "BURNED", "TOKENS/MIN", "TOKENS/HR", "SAMPLES")
			for _, name := range names {
				r := fleet[name]
				fmt.Fprintf(out, "%-20s %12d %14.1f %14.0f %8d\n", r.Profile, r.TokensBurned, r.TokensPerMinute, r.TokensPerHour, r.SampleCount)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output burn rates in JSON format")
	cmd.Flags().StringVar(&window, "window", "1h", "Sliding window duration (e.g. 15m, 1h, 24h)")
	return cmd
}

func newQuotaThrottleCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "throttle [profile]",
		Short: "Show or inspect dynamic token quota throttle policies and limits",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				asJSON = false
			}()
			cfg, err := quota.LoadThrottleConfig()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			if len(args) == 1 {
				target := args[0]
				if err := config.ValidateProfileName(target); err != nil {
					return err
				}
				if !profile.ProfileExists(target) {
					return fmt.Errorf("profile %q does not exist", target)
				}
				policy := cfg.PolicyForProfile(target)
				if asJSON {
					enc := json.NewEncoder(out)
					enc.SetIndent("", "  ")
					return enc.Encode(policy)
				}
				fmt.Fprintf(out, "Throttle Policy for Profile %q:\n", target)
				fmt.Fprintf(out, "  Auto-Throttle:        %t\n", policy.AutoThrottleEnabled)
				fmt.Fprintf(out, "  Depletion Threshold:  %.0f%%\n", policy.EffectiveDepletionThreshold()*100)
				fmt.Fprintf(out, "  Throttle Pacing:      %v\n", policy.ThrottleDelay())
				fmt.Fprintf(out, "  Max Tokens / Hour:    %d\n", policy.MaxTokensPerHour)
				fmt.Fprintf(out, "  Max Tokens / Minute:  %d\n", policy.MaxTokensPerMinute)
				return nil
			}

			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(cfg)
			}

			fmt.Fprintln(out, "Default Fleet Throttle Policy:")
			fmt.Fprintf(out, "  Auto-Throttle:        %t\n", cfg.DefaultPolicy.AutoThrottleEnabled)
			fmt.Fprintf(out, "  Depletion Threshold:  %.0f%%\n", cfg.DefaultPolicy.EffectiveDepletionThreshold()*100)
			fmt.Fprintf(out, "  Throttle Pacing:      %v\n", cfg.DefaultPolicy.ThrottleDelay())
			fmt.Fprintf(out, "  Max Tokens / Hour:    %d\n", cfg.DefaultPolicy.MaxTokensPerHour)
			fmt.Fprintf(out, "  Max Tokens / Minute:  %d\n", cfg.DefaultPolicy.MaxTokensPerMinute)

			if len(cfg.Profiles) > 0 {
				fmt.Fprintln(out, "\nProfile Specific Overrides:")
				var profNames []string
				for p := range cfg.Profiles {
					profNames = append(profNames, p)
				}
				sort.Strings(profNames)
				for _, p := range profNames {
					pol := cfg.Profiles[p]
					fmt.Fprintf(out, "  - %-18s threshold=%.0f%% delay=%v max/hr=%d max/min=%d auto=%t\n",
						p, pol.EffectiveDepletionThreshold()*100, pol.ThrottleDelay(), pol.MaxTokensPerHour, pol.MaxTokensPerMinute, pol.AutoThrottleEnabled)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output throttle configuration in JSON format")
	return cmd
}
