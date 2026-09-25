package tui

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

// LauncherFunc defines the signature for launching a profile
type LauncherFunc func(name string, args []string) error

// IsInteractive checks if both stdin and stdout are interactive terminals
func IsInteractive(stdinFd, stdoutFd uintptr) bool {
	inTerm := isatty.IsTerminal(stdinFd) || isatty.IsCygwinTerminal(stdinFd)
	outTerm := isatty.IsTerminal(stdoutFd) || isatty.IsCygwinTerminal(stdoutFd)
	return inTerm && outTerm
}

// RunMenu displays the interactive TUI menu for selecting or managing profiles
func RunMenu(r io.Reader, w io.Writer, launcher LauncherFunc) error {
	if launcher == nil {
		launcher = profile.LaunchProfile
	}

	profiles, err := profile.GetProfiles()
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		fmt.Fprintln(w, "No profiles found.")
		fmt.Fprintln(w, "Create your first profile with: multigravity new <name>")
		return nil
	}

	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "╔══════════════════════════════════════════════════════════════════╗")
	fmt.Fprintln(w, "║                       MULTIGRAVITY PROFILES                      ║")
	fmt.Fprintln(w, "╚══════════════════════════════════════════════════════════════════╝")
	fmt.Fprintln(w, "")

	for i, p := range profiles {
		statusDot := "○ idle"
		if p.IsRunning {
			statusDot = green("● running")
		}

		typeStr := "isolated"
		if p.Type == "auth-only" || p.Type == "shared" {
			typeStr = "auth-only"
		}

		colorStr := ""
		if p.Color != "" {
			colorStr = fmt.Sprintf("[color: %s]", p.Color)
		}

		numStr := cyan(fmt.Sprintf("[%2d]", i+1))
		fmt.Fprintf(w, "  %s %-20s %-12s (%s) %s\n", numStr, p.Name, statusDot, typeStr, colorStr)
	}

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  [n]  Create a new profile")
	fmt.Fprintln(w, "  [q]  Quit")
	fmt.Fprintln(w, "")
	fmt.Fprint(w, "Select profile number or name: ")

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return nil
	}
	choice := strings.TrimSpace(scanner.Text())

	switch strings.ToLower(choice) {
	case "q", "":
		return nil
	case "n":
		fmt.Fprintln(w, "")
		fmt.Fprint(w, "Enter new profile name: ")
		if !scanner.Scan() {
			return nil
		}
		newName := strings.TrimSpace(scanner.Text())
		if newName != "" {
			if err := config.ValidateProfileName(newName); err != nil {
				return err
			}
			return profile.CreateProfile(profile.CreateOptions{Name: newName})
		}
		return nil
	default:
		// Check numeric index
		if idx, err := strconv.Atoi(choice); err == nil && idx >= 1 && idx <= len(profiles) {
			target := profiles[idx-1].Name
			fmt.Fprintln(w, "")
			fmt.Fprintf(w, "Launching profile: %s ...\n", target)
			return launcher(target, nil)
		}

		// Check if profile exists by name
		if profile.ProfileExists(choice) {
			fmt.Fprintln(w, "")
			fmt.Fprintf(w, "Launching profile: %s ...\n", choice)
			return launcher(choice, nil)
		}

		return fmt.Errorf("invalid selection: %s", choice)
	}
}
