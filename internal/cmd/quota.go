package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func newQuotaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "quota [profile]",
		Short: "Show AI token limits, usage percentage, and reset time",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetProf := ""
			if len(args) > 0 {
				targetProf = args[0]
				if err := config.ValidateProfileName(targetProf); err != nil {
					return err
				}
				if !profile.ProfileExists(targetProf) {
					return fmt.Errorf("profile '%s' does not exist", targetProf)
				}
			}

			servers, err := quota.FindActiveServers(targetProf)
			if err != nil {
				return err
			}

			if len(servers) == 0 {
				if targetProf != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Profile '%s' is not running.\n", targetProf)
					fmt.Fprintf(cmd.OutOrStdout(), "To check quota and token consumption, launch it first: multigravity %s\n", targetProf)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No active Antigravity profile or Language Server found in execution.")
					fmt.Fprintln(cmd.OutOrStdout(), "Launch a profile to monitor quota: multigravity <profile>")
				}
				return nil
			}

			quota.RenderQuotaStatus(cmd.OutOrStdout(), servers)
			return nil
		},
	}
}

var quotaCmd = newQuotaCmd()

func init() {
	rootCmd.AddCommand(quotaCmd)
}
