package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/catalog"
)

func newCatalogCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "catalog [profile]",
		Short: "Inventory and health of MCP servers and skills",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			report, err := catalog.Inspect(name)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			catalog.Run(cmd.OutOrStdout(), report)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output the catalog in JSON format")
	cmd.ValidArgsFunction = profileArgsCompletion
	return cmd
}

func init() {
	rootCmd.AddCommand(newCatalogCmd())
}
