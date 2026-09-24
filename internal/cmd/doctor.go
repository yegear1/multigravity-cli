package cmd

import (
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/doctor"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run a system diagnosis",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := doctor.RunDoctor(cmd.OutOrStdout())
		return err
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
