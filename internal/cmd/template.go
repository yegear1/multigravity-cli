package cmd

import (
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var templateCmd = &cobra.Command{
	Use:   "template <save|list|delete>",
	Short: "Manage reusable profile templates",
}

var templateSaveCmd = &cobra.Command{
	Use:   "save <profile> <template-name>",
	Short: "Save an existing profile as a reusable template",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]
		tplName := args[1]

		cmd.Printf("Saving %q as template %q...\n", profileName, tplName)
		if err := profile.SaveTemplate(profileName, tplName); err != nil {
			return err
		}

		cmd.Printf("Saved template %q\n", tplName)
		return nil
	},
}

var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved profile templates",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		templates, err := profile.ListTemplates()
		if err != nil {
			return err
		}

		cmd.Println("Templates:")
		if len(templates) == 0 {
			cmd.Println("  (none)")
			return nil
		}

		for _, tpl := range templates {
			cmd.Printf("  %-20s %s\n", tpl.Name, tpl.Size)
		}
		return nil
	},
}

var templateDeleteCmd = &cobra.Command{
	Use:   "delete <template-name>",
	Short: "Delete a saved profile template",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tplName := args[0]

		if err := profile.DeleteTemplate(tplName); err != nil {
			return err
		}

		cmd.Printf("Deleted template %q\n", tplName)
		return nil
	},
}

func init() {
	templateCmd.AddCommand(templateSaveCmd)
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateDeleteCmd)
}
