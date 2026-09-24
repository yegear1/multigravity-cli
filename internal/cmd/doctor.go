package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/doctor"
)

var doctorJSON bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run a system diagnosis",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if doctorJSON {
			report, err := doctor.Diagnose()
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}

		_, err := doctor.RunDoctor(cmd.OutOrStdout())
		return err
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "Output diagnosis in JSON format")
	rootCmd.AddCommand(doctorCmd)
}

