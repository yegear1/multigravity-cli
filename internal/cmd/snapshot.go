package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

func newSnapshotCmd() *cobra.Command {
	var asJSON bool
	var note string

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Create and restore profile and conversation snapshots",
		Long: `Save a restore point of a profile and its AI conversations.

The archive omits auth tokens and credential trees. Creating a snapshot and
rolling one back both refuse while the profile is running.`,
	}

	createCmd := &cobra.Command{
		Use:   "create <profile>",
		Short: "Save a restore point of a profile and its conversations",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			snap, err := profile.CreateSnapshot(args[0], note)
			if err != nil {
				return err
			}
			if asJSON {
				return encodeSnapshotJSON(cmd, snap)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created snapshot %s for %q (%s)\n", snap.ID, snap.Profile, strings.Join(snap.Includes, ", "))
			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			note = ""
			asJSON = false
		},
	}
	createCmd.Flags().StringVar(&note, "note", "", "Short note stored with the restore point")

	listCmd := &cobra.Command{
		Use:   "list <profile>",
		Short: "List restore points for a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			snaps, err := profile.ListSnapshots(args[0])
			if err != nil {
				return err
			}
			if asJSON {
				return encodeSnapshotJSON(cmd, snaps)
			}
			if len(snaps) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No snapshots for %q.\n", args[0])
				return nil
			}
			for _, snap := range snaps {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s", snap.ID, snap.CreatedAt.Format("2006-01-02 15:04:05Z"), strings.Join(snap.Includes, ","))
				if snap.Note != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s", snap.Note)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			asJSON = false
		},
	}

	rollbackCmd := &cobra.Command{
		Use:   "rollback <profile> <id>",
		Short: "Restore a profile and its conversations to a snapshot",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			snap, err := profile.RollbackSnapshot(args[0], args[1])
			if err != nil {
				return err
			}
			if asJSON {
				return encodeSnapshotJSON(cmd, snap)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Restored profile %q to snapshot %s\n", snap.Profile, snap.ID)
			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			asJSON = false
		},
	}

	deleteCmd := &cobra.Command{
		Use:   "delete <profile> <id>",
		Short: "Remove a restore point without changing the profile",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := profile.DeleteSnapshot(args[0], args[1]); err != nil {
				return err
			}
			if asJSON {
				return encodeSnapshotJSON(cmd, map[string]string{
					"profile": args[0],
					"id":      args[1],
					"status":  "deleted",
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed snapshot %s for %q\n", args[1], args[0])
			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			asJSON = false
		},
	}

	cmd.PersistentFlags().BoolVar(&asJSON, "json", false, "Output in JSON format")
	createCmd.ValidArgsFunction = profileArgsCompletion
	listCmd.ValidArgsFunction = profileArgsCompletion
	rollbackCmd.ValidArgsFunction = snapshotArgsCompletion
	deleteCmd.ValidArgsFunction = snapshotArgsCompletion
	cmd.AddCommand(createCmd, listCmd, rollbackCmd, deleteCmd)
	return cmd
}

func snapshotArgsCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return profileArgsCompletion(cmd, args, toComplete)
	}
	if len(args) == 1 {
		snaps, err := profile.ListSnapshots(args[0])
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		ids := make([]string, 0, len(snaps))
		for _, snap := range snaps {
			ids = append(ids, snap.ID)
		}
		return ids, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

func encodeSnapshotJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

var snapshotCmd = newSnapshotCmd()

func init() {
	rootCmd.AddCommand(snapshotCmd)
}
