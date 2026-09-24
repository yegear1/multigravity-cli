package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func showCompletionHelp(w io.Writer) {
	if runtime.GOOS == "windows" {
		fmt.Fprintln(w, "To enable autocompletion in PowerShell, add the following to your $PROFILE:")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "  Invoke-Expression (& multigravity completion powershell)")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Then reload your profile or run: . $PROFILE")
		return
	}

	shellPath := os.Getenv("SHELL")
	shellName := filepath.Base(shellPath)
	if shellName == "" || shellName == "." {
		shellName = "bash"
	}

	fmt.Fprintln(w, "To enable autocompletion in your shell, add the following to your config:")
	fmt.Fprintln(w, "")
	switch shellName {
	case "zsh":
		fmt.Fprintln(w, "  # Add to ~/.zshrc")
		fmt.Fprintln(w, "  source <(multigravity completion zsh)")
	case "fish":
		fmt.Fprintln(w, "  # For current session:")
		fmt.Fprintln(w, "  multigravity completion fish | source")
		fmt.Fprintln(w, "  # To load completions for each session:")
		fmt.Fprintln(w, "  multigravity completion fish > ~/.config/fish/completions/multigravity.fish")
	default: // bash and others
		fmt.Fprintln(w, "  # Add to ~/.bashrc")
		fmt.Fprintln(w, "  source <(multigravity completion bash)")
	}
	fmt.Fprintln(w, "")
	if shellName == "fish" {
		fmt.Fprintln(w, "Then restart your shell.")
	} else {
		fmt.Fprintf(w, "Then restart your terminal or run: source ~/.%src\n", shellName)
	}
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Setup shell completion",
	Long: `Generate shell completion scripts or display instructions to configure
autocompletion for bash, zsh, fish, or powershell.`,
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	Args:      cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			showCompletionHelp(cmd.OutOrStdout())
			return nil
		}

		shell := strings.ToLower(args[0])
		out := cmd.OutOrStdout()
		root := cmd.Root()

		switch shell {
		case "bash":
			return root.GenBashCompletion(out)
		case "zsh":
			return root.GenZshCompletion(out)
		case "fish":
			return root.GenFishCompletion(out, true)
		case "powershell":
			return root.GenPowerShellCompletionWithDesc(out)
		default:
			return fmt.Errorf("unsupported shell type %q (supported: bash, zsh, fish, powershell)", shell)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
