package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var exportIncludeCache bool

var exportCmd = &cobra.Command{
	Use:   "export <name> [path]",
	Short: "Export a profile to a compressed archive",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		outPath := ""
		if len(args) > 1 {
			outPath = args[1]
		}

		displayPath := outPath
		if displayPath == "" {
			displayPath = fmt.Sprintf("./%s.tar.gz", name)
		}
		cmd.Printf("Exporting %q to %s ...\n", name, displayPath)

		finalPath, err := profile.ExportProfile(name, outPath, exportIncludeCache)
		if err != nil {
			return err
		}

		_ = finalPath
		cmd.Println("Done.")
		return nil
	},
}

func init() {
	exportCmd.Flags().BoolVar(&exportIncludeCache, "include-cache", false, "Include volatile cache directories in export")
}
