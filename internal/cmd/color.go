package cmd

import (
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var colorClear bool

var colorCmd = &cobra.Command{
	Use:   "color <profile> [color|--clear]",
	Short: "View or change profile theme color",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		profName := args[0]
		if err := config.ValidateProfileName(profName); err != nil {
			return err
		}

		colorArg := ""
		if len(args) == 2 {
			colorArg = args[1]
		}

		if colorClear || colorArg == "--clear" {
			if err := profile.ApplyProfileColor(profName, "", true); err != nil {
				return err
			}
			cmd.Printf("Cleared color customizations for profile '%s'.\n", profName)
			return nil
		}

		if colorArg == "" {
			currentColor, err := profile.GetProfileColor(profName)
			if err != nil {
				return err
			}
			if currentColor != "" {
				cmd.Printf("Profile '%s' color: %s\n", profName, currentColor)
				return nil
			}
			cmd.Printf("Profile '%s' has no custom color set.\n", profName)
			return nil
		}

		if err := profile.ApplyProfileColor(profName, colorArg, false); err != nil {
			return err
		}
		cmd.Printf("Set theme color for profile '%s' to %s.\n", profName, colorArg)
		return nil
	},
}

func init() {
	colorCmd.Flags().BoolVar(&colorClear, "clear", false, "Clear window theme color customizations")
}
