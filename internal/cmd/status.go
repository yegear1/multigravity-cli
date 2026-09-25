package cmd

import (
	"encoding/json"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var statusJSON bool

var statusCmd = &cobra.Command{
	Use:   "status [profile]",
	Short: "Show status, running state, and disk usage of profiles",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		defer func() {
			_ = cmd.Flags().Set("json", "false")
			statusJSON = false
		}()

		if len(args) == 1 {
			p, err := profile.GetProfile(args[0])
			if err != nil {
				return err
			}

			if statusJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(p)
			}

			cmd.Printf("%-18s %-10s %-12s %-20s %s\n", "PROFILE", "RUNNING", "TYPE", "LAST USED", "SIZE")
			cmd.Printf("%-18s %-10s %-12s %-20s %s\n", "-------", "-------", "----", "---------", "----")

			green := color.New(color.FgGreen).SprintFunc()
			runningStr := "no"
			if p.IsRunning {
				runningStr = green("yes")
			}

			lastUsedStr := "unknown"
			if !p.LastUsed.IsZero() {
				lastUsedStr = p.LastUsed.Format("2006-01-02 15:04")
			}

			cmd.Printf("%-18s %-10s %-12s %-20s %s\n", p.Name, runningStr, p.Type, lastUsedStr, p.Size)
			return nil
		}

		if statusJSON {
			profiles, err := profile.GetProfiles()
			if err != nil {
				return err
			}
			if profiles == nil {
				profiles = []profile.ProfileInfo{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(profiles)
		}

		profiles, err := profile.GetProfiles()
		if err != nil {
			return err
		}

		if len(profiles) == 0 {
			cmd.Println("No profiles found.")
			return nil
		}

		cmd.Printf("%-18s %-10s %-12s %-20s %s\n", "PROFILE", "RUNNING", "TYPE", "LAST USED", "SIZE")
		cmd.Printf("%-18s %-10s %-12s %-20s %s\n", "-------", "-------", "----", "---------", "----")

		green := color.New(color.FgGreen).SprintFunc()

		for _, p := range profiles {
			runningStr := "no"
			if p.IsRunning {
				runningStr = green("yes")
			}

			lastUsedStr := "unknown"
			if !p.LastUsed.IsZero() {
				lastUsedStr = p.LastUsed.Format("2006-01-02 15:04")
			}

			cmd.Printf("%-18s %-10s %-12s %-20s %s\n", p.Name, runningStr, p.Type, lastUsedStr, p.Size)
		}

		return nil
	},
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output status in JSON format")
}

