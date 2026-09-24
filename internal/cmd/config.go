package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var configCmd = &cobra.Command{
	Use:   "config <status|share|isolate|seed> <profile|--all|--host>",
	Short: "Manage config.json permissions and synchronization",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		action := args[0]
		targetArg := ""
		if len(args) == 2 {
			targetArg = args[1]
		}

		switch action {
		case "seed", "allow-readonly":
			msgs, err := profile.ConfigSeed(targetArg)
			if err != nil {
				return err
			}
			for _, m := range msgs {
				cmd.Println(m)
			}
			return nil
		case "status":
			if targetArg == "" {
				return fmt.Errorf("usage: multigravity config status <profile>")
			}
			msg, err := profile.ConfigStatus(targetArg)
			if err != nil {
				return err
			}
			cmd.Println(msg)
			return nil
		case "share":
			if targetArg == "" {
				return fmt.Errorf("usage: multigravity config share <profile>")
			}
			msgs, err := profile.ConfigShare(targetArg)
			if err != nil {
				return err
			}
			for _, m := range msgs {
				cmd.Println(m)
			}
			return nil
		case "isolate":
			if targetArg == "" {
				return fmt.Errorf("usage: multigravity config isolate <profile>")
			}
			msgs, err := profile.ConfigIsolate(targetArg)
			if err != nil {
				return err
			}
			for _, m := range msgs {
				cmd.Println(m)
			}
			return nil
		default:
			return fmt.Errorf("usage: multigravity config <status|share|isolate|seed> <profile|--all|--host>")
		}
	},
}

var (
	allowReadonlyHost bool
	allowReadonlyAll  bool
)

var allowReadonlyCmd = &cobra.Command{
	Use:   "allow-readonly [profile|--all|--host]",
	Short: "Seed default read-only permissions in config.json (alias for config seed)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := ""
		if allowReadonlyHost {
			target = "--host"
		} else if allowReadonlyAll {
			target = "--all"
		} else if len(args) == 1 {
			target = args[0]
		}
		msgs, err := profile.ConfigSeed(target)
		if err != nil {
			return err
		}
		for _, m := range msgs {
			cmd.Println(m)
		}
		return nil
	},
}

func init() {
	allowReadonlyCmd.Flags().BoolVar(&allowReadonlyHost, "host", false, "Seed default read-only permissions in host config.json")
	allowReadonlyCmd.Flags().BoolVar(&allowReadonlyAll, "all", false, "Seed default read-only permissions in host and all profile configs")
}


