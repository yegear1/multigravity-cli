package main

import (
	"os"

	"github.com/ye-dev/multigravity-cli/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
