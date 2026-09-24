package cmd

import (
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var restartCmd = &cobra.Command{
	Use:   "restart <name> [args...]",
	Short: "Restart a profile gracefully",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		forwardArgs := args[1:]
		return profile.RestartProfile(name, forwardArgs)
	},
}

func init() {
	restartCmd.Flags().SetInterspersed(false)
}
