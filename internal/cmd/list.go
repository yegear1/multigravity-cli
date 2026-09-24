package cmd

import (
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var listRaw bool

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List existing profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
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
}
