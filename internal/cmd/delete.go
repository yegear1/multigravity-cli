package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var deleteForce bool

var deleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a profile and all its data",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if !deleteForce {
			fmt.Printf("Delete profile %q and all its data? [y/N] ", name)
			reader := bufio.NewReader(os.Stdin)
			ans, _ := reader.ReadString('\n')
			ans = strings.TrimSpace(ans)
			if strings.ToLower(ans) != "y" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		if err := profile.DeleteProfile(name, deleteForce); err != nil {
			return err
		}

		fmt.Printf("Deleted profile %q\n", name)
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Force delete even if running, skipping confirmation")
}
