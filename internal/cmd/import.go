package cmd

import (
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var importCmd = &cobra.Command{
	Use:   "import <archive> [name]",
	Short: "Import a profile from an archive",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		archivePath := args[0]
		name := ""
		if len(args) > 1 {
			name = args[1]
		}

		importedName, err := profile.ImportProfile(archivePath, name)
		if err != nil {
			return err
		}

		cmd.Printf("Imported profile %q\n", importedName)
		return nil
	},
}
