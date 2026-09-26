package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/headless"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var (
	execJSON            bool
	execAll             bool
	execTimeout         time.Duration
	execWorkers         int
	execSkipPermissions bool

	execGreen = color.New(color.FgGreen).SprintFunc()
	execRed   = color.New(color.FgRed).SprintFunc()
	execBold  = color.New(color.Bold).SprintFunc()
	execDim   = color.New(color.Faint).SprintFunc()
)

func newExecCmd() *cobra.Command {
	execCmd := &cobra.Command{
		Use:   "exec [profile|--all] <prompt>",
		Short: "Fan out a headless prompt across one profile or every profile",
		Long: `Run the same prompt on one profile, or on every profile with --all.

Each profile is executed by the existing headless runner (agy, or the language
server cascade when agy is absent), with that profile's own HOME and credential
vault. A worker pool caps how many runs are in flight. This command does not
start a separate agent engine.`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 && !execAll {
				profiles, err := profile.ListProfiles()
				if err != nil {
					return nil, cobra.ShellCompDirectiveNoFileComp
				}
				return append(profiles, "--all"), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: runExec,
	}

	execCmd.Flags().BoolVar(&execJSON, "json", false, "Output the aggregated report as JSON")
	execCmd.Flags().BoolVar(&execAll, "all", false, "Run the prompt on every profile")
	execCmd.Flags().DurationVarP(&execTimeout, "timeout", "t", 120*time.Second, "Per-profile execution timeout")
	execCmd.Flags().IntVarP(&execWorkers, "workers", "w", 0, "Maximum in-flight runs (default: one per selected profile)")
	execCmd.Flags().BoolVar(&execSkipPermissions, "dangerously-skip-permissions", true, "Skip interactive sandbox/permission confirmations")
	return execCmd
}

func runExec(cmd *cobra.Command, args []string) error {
	defer func() {
		execJSON = false
		execAll = false
		execTimeout = 120 * time.Second
		execWorkers = 0
		execSkipPermissions = true
	}()

	var profileName string
	var prompt string
	if execAll {
		prompt = strings.Join(args, " ")
	} else {
		if len(args) < 2 {
			return fmt.Errorf("usage: multigravity exec [profile|--all] \"<prompt>\"")
		}
		profileName = args[0]
		if profileName == "--all" {
			execAll = true
			prompt = strings.Join(args[1:], " ")
		} else {
			prompt = strings.Join(args[1:], " ")
		}
	}

	mgr := headless.GetDefaultManager()
	report, err := mgr.Exec(headless.ExecOptions{
		Profile:                    profileName,
		All:                        execAll,
		Prompt:                     prompt,
		Timeout:                    execTimeout,
		Workers:                    execWorkers,
		DangerouslySkipPermissions: execSkipPermissions,
	})
	if err != nil {
		return err
	}

	if execJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	} else if err := writeExecTable(cmd, report); err != nil {
		return err
	}

	if report.Failed > 0 {
		return fmt.Errorf("%d of %d profile runs failed", report.Failed, len(report.Results))
	}
	return nil
}

func writeExecTable(cmd *cobra.Command, report *headless.ExecReport) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, execBold("PROFILE\tEXIT\tTOKENS\tDURATION\tRESPONSE"))
	for _, res := range report.Results {
		exitStr := execGreen("0")
		if res.ExitCode != 0 || res.Error != "" {
			exitStr = execRed(fmt.Sprintf("%d", res.ExitCode))
		}
		response := res.Response
		if res.Error != "" {
			response = res.Error
		}
		response = compactExecText(response, 72)
		fmt.Fprintf(w, "%s\t%s\t%d\t%.2fs\t%s\n", res.Profile, exitStr, res.TotalTokens, res.DurationSeconds, response)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %d profiles | %d succeeded | %d failed | tokens: %d | %.2fs\n",
		execDim("---"), len(report.Results), report.Succeeded, report.Failed, report.TotalTokens, report.DurationSeconds)
	return nil
}

func compactExecText(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if max > 0 && len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}
