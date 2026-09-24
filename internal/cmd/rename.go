package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var renameCmd = &cobra.Command{
	Use:   "rename <old> <new>",
	Short: "Rename an existing profile",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName := args[0]
		newName := args[1]

		if err := profile.RenameProfile(oldName, newName); err != nil {
			return err
		}

		fmt.Printf("Renamed profile %q to %q\n", oldName, newName)
		return nil
	},
}
