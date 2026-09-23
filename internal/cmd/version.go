package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/config"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print multigravity version and runtime information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("multigravity version %s (%s/%s)\n", config.Version, runtime.GOOS, runtime.GOARCH)
	},
}
