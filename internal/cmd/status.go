package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status, running state, and disk usage of profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := profile.GetProfiles()
		if err != nil {
			return err
		}

		if len(profiles) == 0 {
			fmt.Println("No profiles found.")
			return nil
		}

		fmt.Printf("%-18s %-10s %-12s %-20s %s\n", "PROFILE", "RUNNING", "TYPE", "LAST USED", "SIZE")
		fmt.Printf("%-18s %-10s %-12s %-20s %s\n", "-------", "-------", "----", "---------", "----")

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

			fmt.Printf("%-18s %-10s %-12s %-20s %s\n", p.Name, runningStr, p.Type, lastUsedStr, p.Size)
		}

		return nil
	},
}
