package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var ghCmd = &cobra.Command{
	Use:   "gh <status|share|isolate> <profile>",
	Short: "Manage GitHub CLI authentication sharing for a profile",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		action := args[0]
		profName := args[1]

		switch action {
		case "status":
			msg, err := profile.GhStatus(profName)
			if err != nil {
				return err
			}
			cmd.Println(msg)
			return nil
		case "share":
			msgs, err := profile.GhShare(profName)
			if err != nil {
				return err
			}
			for _, m := range msgs {
				cmd.Println(m)
			}
			return nil
		case "isolate":
			msgs, err := profile.GhIsolate(profName)
			if err != nil {
				return err
			}
			for _, m := range msgs {
				cmd.Println(m)
			}
			return nil
		default:
			return fmt.Errorf("usage: multigravity gh <status|share|isolate> <profile>")
		}
	},
}


