package cmd

import (
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var cloneCmd = &cobra.Command{
	Use:   "clone <src> <dest>",
	Short: "Copy an existing profile",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		src := args[0]
		dest := args[1]

		cmd.Printf("Cloning profile %q to %q...\n", src, dest)
		if err := profile.CloneProfile(src, dest); err != nil {
			return err
		}

		cmd.Printf("Successfully cloned %q to %q\n", src, dest)
		return nil
	},
}
