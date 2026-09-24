package cmd

import (
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show storage usage and extension stats for all profiles",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		stats, total, err := profile.GetProfileStats()
		if err != nil {
			return err
		}

		if len(stats) == 0 {
			cmd.Println("No profiles found.")
			return nil
		}

		cmd.Println("Profile Storage Usage:")
		cmd.Printf("%-20s %-10s %-10s\n", "PROFILE", "SIZE", "EXTENSIONS")
		cmd.Printf("%-20s %-10s %-10s\n", "-------", "----", "----------")

		for _, s := range stats {
			cmd.Printf("%-20s %-10s %-10d\n", s.Name, s.Size, s.ExtensionCount)
		}

		cmd.Println()
		cmd.Printf("Total usage: %s\n", total)
		return nil
	},
}
