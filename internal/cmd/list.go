package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var (
	listRaw  bool
	listJSON bool
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List existing profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		if listJSON {
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

		profiles, err := profile.ListProfiles()
		if err != nil {
			return err
		}

		if len(profiles) == 0 {
			if !listRaw {
				cmd.Println("Existing profiles:")
				cmd.Println("(none)")
			}
			return nil
		}

		if !listRaw {
			cmd.Println("Existing profiles:")
		}

		for _, p := range profiles {
			cmd.Println(p)
		}
		return nil
	},
}

func init() {
	listCmd.Flags().BoolVar(&listRaw, "raw", false, "Output profile names only (one per line)")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output profiles in JSON format")
}

