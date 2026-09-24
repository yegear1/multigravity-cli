package cmd

import (
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var stopForce bool

var stopCmd = &cobra.Command{
	Use:   "stop <name>",
	Short: "Stop a running profile gracefully",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		return profile.StopProfile(name, stopForce)
	},
}

func init() {
	stopCmd.Flags().BoolVarP(&stopForce, "force", "f", false, "Force stop processes immediately (SIGKILL)")
}
