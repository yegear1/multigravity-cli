package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var cleanAll bool

var cleanCmd = &cobra.Command{
	Use:   "clean <profile|--all>",
	Short: "Clean caches and temporary files of a profile",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := ""
		if cleanAll {
			target = "--all"
		} else if len(args) > 0 {
			target = args[0]
		}

		if target == "" {
			return fmt.Errorf("usage: multigravity clean <profile|--all>")
		}

		return profile.CleanProfile(target)
	},
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanAll, "all", false, "Clean caches for all profiles")
}
