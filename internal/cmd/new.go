package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var (
	newShared           bool
	newIsolatedDotfiles bool
	newIsolatedMCP      bool
	newIsolatedSkills   bool
	newIsolatedConfig   bool
	newIsolatedGH       bool
	newColor            string
	newFrom             string
)

var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new isolated Antigravity profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		opts := profile.CreateOptions{
			Name:             name,
			Shared:           newShared,
			IsolatedDotfiles: newIsolatedDotfiles,
			IsolatedMCP:      newIsolatedMCP,
			IsolatedSkills:   newIsolatedSkills,
			IsolatedConfig:   newIsolatedConfig,
			IsolatedGH:       newIsolatedGH,
			Color:            newColor,
			FromTemplate:     newFrom,
		}

		if err := profile.CreateProfile(opts); err != nil {
			return err
		}

		fmt.Printf("Created profile %q\n", name)
		return nil
	},
}

func init() {
	newCmd.Flags().BoolVar(&newShared, "shared", false, "Share base editor settings with system install")
	newCmd.Flags().BoolVar(&newIsolatedDotfiles, "isolated-dotfiles", false, "Do not link user .gitconfig or .ssh")
	newCmd.Flags().BoolVar(&newIsolatedMCP, "isolated-mcp", false, "Do not link user MCP servers")
	newCmd.Flags().BoolVar(&newIsolatedSkills, "isolated-skills", false, "Do not link user AI skills/plugins")
	newCmd.Flags().BoolVar(&newIsolatedConfig, "isolated-config", false, "Do not link user config.json and permission grants")
	newCmd.Flags().BoolVar(&newIsolatedGH, "isolated-gh", false, "Do not link user GitHub CLI authentication")
	newCmd.Flags().StringVar(&newColor, "color", "", "Visual accent color for profile window")
	newCmd.Flags().StringVar(&newFrom, "from", "", "Create from a saved template")
}
