package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/ye-dev/multigravity-cli/internal/dispatch"
)

var (
	dpWaveTag     string
	dpWaveFile    string
	dpWaveRepo    string
	dpWaveTimeout string
	dpWaveNoRetry bool
	dpWaveLint    bool
	dpWaveFull    bool
)

func newDispatchWaveCmd() *cobra.Command {
	waveSubCmd := &cobra.Command{
		Use:   "wave",
		Short: "Dispatch a wave of approved tasks from backlog across ephemeral worktrees",
		Long: `Read task specifications from .agent/TASK.md for a given wave tag (e.g. W1)
and execute them in parallel or sequence with automatic profile and quota routing.`,
		Args: cobra.NoArgs,
		RunE: runDispatchWave,
	}
	waveSubCmd.Flags().StringVar(&dpWaveTag, "wave", "", "Wave tag to execute (defaults to first uncompleted approved wave)")
	waveSubCmd.Flags().StringVar(&dpWaveFile, "file", ".agent/TASK.md", "Path to task markdown file")
	waveSubCmd.Flags().StringVar(&dpWaveRepo, "repo", "", "Target git repository directory")
	waveSubCmd.Flags().StringVar(&dpWaveTimeout, "timeout", "4m", "Execution timeout duration (e.g. 4m, 300s)")
	waveSubCmd.Flags().BoolVar(&dpWaveNoRetry, "no-retry", false, "Disable automatic retry of failed subtasks")
	waveSubCmd.Flags().BoolVar(&dpWaveLint, "lint", false, "Validate task wave specifications against governance lint rules without executing")
	waveSubCmd.Flags().BoolVar(&dpWaveFull, "full", false, "Output full verbose wave execution result including subtask prompts and diffs")
	return waveSubCmd
}

func runDispatchWave(cmd *cobra.Command, args []string) error {
	defer func() {
		dpWaveTag = ""
		dpWaveFile = ""
		dpWaveRepo = ""
		dpWaveTimeout = ""
		dpWaveNoRetry = false
		dpWaveLint = false
		dpWaveFull = false
		dpJSON = false
	}()

	if dpWaveLint {
		repoRoot := dpWaveRepo
		if repoRoot == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}
			repoRoot = cwd
		}

		taskFile := dpWaveFile
		if taskFile == "" {
			taskFile = filepath.Join(repoRoot, ".agent", "TASK.md")
		} else if !filepath.IsAbs(taskFile) {
			taskFile = filepath.Join(repoRoot, taskFile)
		}

		waveSpec, err := dispatch.ParseTaskWave(taskFile, dpWaveTag)
		if err != nil {
			return err
		}

		violations := dispatch.LintTaskWave(waveSpec)
		if violations == nil {
			violations = []string{}
		}

		out := cmd.OutOrStdout()
		if dpJSON {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			if err := enc.Encode(violations); err != nil {
				return err
			}
		} else {
			if len(violations) == 0 {
				fmt.Fprintln(out, "Wave lint passed with zero violations.")
			} else {
				fmt.Fprintf(out, "%s Found %d wave lint violation(s):\n", dpRed("✗"), len(violations))
				for _, v := range violations {
					fmt.Fprintf(out, "  - %s\n", v)
				}
			}
		}

		if len(violations) > 0 {
			return fmt.Errorf("wave lint failed with %d violation(s)", len(violations))
		}
		return nil
	}

	var timeoutDur time.Duration
	if dpWaveTimeout != "" {
		d, err := time.ParseDuration(dpWaveTimeout)
		if err != nil {
			return fmt.Errorf("invalid timeout duration %q: %w", dpWaveTimeout, err)
		}
		timeoutDur = d
	}

	opts := dispatch.WaveOptions{
		RepoRoot: dpWaveRepo,
		TaskFile: dpWaveFile,
		WaveTag:  dpWaveTag,
		Timeout:  timeoutDur,
		NoRetry:  dpWaveNoRetry,
	}

	out := cmd.OutOrStdout()
	if !dpJSON {
		tagDesc := opts.WaveTag
		if tagDesc == "" {
			tagDesc = "auto (first uncompleted approved)"
		}
		fmt.Fprintf(out, "%s Dispatching task wave [%s]...\n", dpCyan("▶"), dpBold(tagDesc))
	}

	mgr := dispatch.GetDefaultTaskManager()
	result, err := mgr.ExecuteTaskWave(cmd.Context(), opts)
	if err != nil {
		if result != nil && dpJSON {
			outResult := result
			if !dpWaveFull {
				outResult = result.ToCompact()
			}
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			_ = enc.Encode(outResult)
		}
		return err
	}

	if dpJSON {
		outResult := result
		if !dpWaveFull {
			outResult = result.ToCompact()
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(outResult)
	}

	statusStr := dpGreen(result.WaveStatus)
	if result.WaveStatus == "failed" {
		statusStr = dpRed(result.WaveStatus)
	} else if result.WaveStatus == "partial" {
		statusStr = dpYellow(result.WaveStatus)
	}

	fmt.Fprintf(out, "\n%s Wave %s [%s]\n", dpBold("Wave Execution:"), dpBold(result.WaveTag), statusStr)

	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "\nTASK\tPROFILE\tSTATUS\tEXIT\tCOMMIT\tMODEL")
	fmt.Fprintln(w, "----\t-------\t------\t----\t------\t-----")
	for _, st := range result.Tasks {
		stStatus := dpGreen(string(st.Status))
		if st.Status == dispatch.StatusFailed {
			stStatus = dpRed(string(st.Status))
		}
		commitStr := st.Commit
		if commitStr == "" {
			commitStr = "-"
		}
		modelStr := st.Model
		if modelStr == "" {
			modelStr = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
			st.ID, st.Profile, stStatus, st.ExitCode, commitStr, modelStr)
	}
	_ = w.Flush()

	if result.TaskFileCommit != "" {
		fmt.Fprintf(out, "\n%s Task file updated and committed: %s\n", dpGreen("✓"), dpBold(result.TaskFileCommit))
	}

	if result.WaveStatus != "completed" {
		return fmt.Errorf("wave finished with status: %s", result.WaveStatus)
	}
	return nil
}
