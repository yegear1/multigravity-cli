package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var listRaw bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List existing profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := profile.ListProfiles()
		if err != nil {
			return err
		}

		if len(profiles) == 0 {
			if !listRaw {
				fmt.Println("Existing profiles:")
				fmt.Println("(none)")
			}
			return nil
		}

		if !listRaw {
			fmt.Println("Existing profiles:")
		}

		for _, p := range profiles {
			fmt.Println(p)
		}
		return nil
	},
}

func init() {
	listCmd.Flags().BoolVar(&listRaw, "raw", false, "Output profile names only (one per line)")
}
