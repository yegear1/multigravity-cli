package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var skillsCmd = &cobra.Command{
	Use:   "skills <status|share|isolate> <profile>",
	Short: "Manage AI skills and plugins for a profile",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		action := args[0]
		profName := args[1]

		switch action {
		case "status":
			msg, err := profile.SkillsStatus(profName)
			if err != nil {
				return err
			}
			cmd.Println(msg)
			return nil
		case "share":
			msgs, err := profile.SkillsShare(profName)
			if err != nil {
				return err
			}
			for _, m := range msgs {
				cmd.Println(m)
			}
			return nil
		case "isolate":
			msgs, err := profile.SkillsIsolate(profName)
			if err != nil {
				return err
			}
			for _, m := range msgs {
				cmd.Println(m)
			}
			return nil
		default:
			return fmt.Errorf("usage: multigravity skills <status|share|isolate> <profile>")
		}
	},
}


