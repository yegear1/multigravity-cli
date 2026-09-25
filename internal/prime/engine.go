package prime

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/profile"
)

type PrimeOptions struct {
	Profile          string
	Force            bool
	Check            bool
	Status           bool
	NoJitter         bool
	MaxJitter        float64
	Include5h        bool
	Warm5h           bool
	Quiet            bool
	JSON             bool
	Out              io.Writer
	InstallCron      bool
	UninstallCron    bool
	InstallSystemd   bool
	UninstallSystemd bool
	InstallTask      bool
	UninstallTask    bool
}

type BucketConfig struct {
	Key        string
	BucketID   string
	Name       string
	Period     string
	ParentKey  string
	Model      string
	ModelLabel string
}

var BucketConfigs = []BucketConfig{
	{
		Key:        "gemini",
		BucketID:   "gemini-weekly",
		Name:       "Gemini Models (Weekly)",
		Period:     "weekly",
		ParentKey:  "",
		Model:      "MODEL_PLACEHOLDER_M73",
		ModelLabel: "gemini-3.6-flash-low",
	},
	{
		Key:        "gemini_5h",
		BucketID:   "gemini-5h",
		Name:       "Gemini Models (5-Hour Window)",
		Period:     "5h",
		ParentKey:  "gemini",
		Model:      "MODEL_PLACEHOLDER_M73",
		ModelLabel: "gemini-3.6-flash-low",
	},
	{
		Key:        "3p",
		BucketID:   "3p-weekly",
		Name:       "Claude & GPT models (Weekly)",
		Period:     "weekly",
		ParentKey:  "",
		Model:      "MODEL_PLACEHOLDER_M35",
		ModelLabel: "claude-sonnet-4-6",
	},
	{
		Key:        "3p_5h",
		BucketID:   "3p-5h",
		Name:       "Claude & GPT models (5-Hour Window)",
		Period:     "5h",
		ParentKey:  "3p",
		Model:      "MODEL_PLACEHOLDER_M35",
		ModelLabel: "claude-sonnet-4-6",
	},
}

// RunPrime provides backward-compatible execution for the CLI
func RunPrime(opts PrimeOptions) error {
	// 1. Resolve Profile if empty
	if opts.Profile == "" {
		profiles, err := profile.ListProfiles()
		if err == nil && len(profiles) == 1 {
			opts.Profile = profiles[0]
		}
	}

	// 2. Handle Watchdog commands
	if opts.InstallCron {
		if opts.Profile == "" {
			return fmt.Errorf("profile name is required for --install-cron")
		}
		if !profile.ProfileExists(opts.Profile) {
			return fmt.Errorf("profile '%s' does not exist", opts.Profile)
		}
		return InstallCron(opts.Profile, opts.Include5h)
	}

	if opts.UninstallCron {
		if opts.Profile == "" {
			return fmt.Errorf("profile name is required for --uninstall-cron")
		}
		return UninstallCron(opts.Profile)
	}

	if opts.InstallSystemd {
		if opts.Profile == "" {
			return fmt.Errorf("profile name is required for --install-systemd")
		}
		if !profile.ProfileExists(opts.Profile) {
			return fmt.Errorf("profile '%s' does not exist", opts.Profile)
		}
		return InstallSystemd(opts.Profile, opts.Include5h)
	}

	if opts.UninstallSystemd {
		if opts.Profile == "" {
			return fmt.Errorf("profile name is required for --uninstall-systemd")
		}
		return UninstallSystemd(opts.Profile)
	}

	if opts.InstallTask {
		if opts.Profile == "" {
			return fmt.Errorf("profile name is required for --install-task")
		}
		if !profile.ProfileExists(opts.Profile) {
			return fmt.Errorf("profile '%s' does not exist", opts.Profile)
		}
		return InstallCron(opts.Profile, opts.Include5h)
	}

	if opts.UninstallTask {
		if opts.Profile == "" {
			return fmt.Errorf("profile name is required for --uninstall-task")
		}
		return UninstallCron(opts.Profile)
	}

	if opts.Profile != "" && !profile.ProfileExists(opts.Profile) {
		return fmt.Errorf("profile '%s' does not exist", opts.Profile)
	}

	outWriter := opts.Out
	if outWriter == nil {
		outWriter = os.Stdout
	}

	// 3. Action: Status
	if opts.Status {
		if opts.Profile == "" {
			profiles, err := profile.ListProfiles()
			if err == nil && len(profiles) > 1 {
				all, err := GetAllProfilesPrimeStatus()
				if err != nil {
					return err
				}
				if opts.JSON {
					enc := json.NewEncoder(outWriter)
					enc.SetIndent("", "  ")
					return enc.Encode(all)
				}
				for _, st := range all {
					RenderStatusText(&st)
				}
				return nil
			}
		}

		st, err := GetProfilePrimeStatus(opts.Profile)
		if err != nil {
			return err
		}

		if opts.JSON {
			enc := json.NewEncoder(outWriter)
			enc.SetIndent("", "  ")
			return enc.Encode(st)
		}

		if st.ServerMode == "offline" {
			if !opts.Quiet {
				fmt.Fprintf(os.Stderr, "Error: Language server is not running for profile '%s', and executable not found.\n", opts.Profile)
				fmt.Fprintf(os.Stderr, "Please launch the profile first: multigravity %s\n", opts.Profile)
			}
			return fmt.Errorf("language server is not running for profile '%s', and executable not found", opts.Profile)
		}

		RenderStatusText(st)
		return nil
	}

	// 4. Action: Prime
	if opts.JSON {
		res, err := ExecutePrime(opts, nil)
		if err != nil {
			enc := json.NewEncoder(outWriter)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{"error": err.Error()})
			return err
		}
		enc := json.NewEncoder(outWriter)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	// Interactive CLI Progress Callback reproducing exact legacy terminal prints
	cliCallback := func(ev PrimeProgressEvent) {
		switch ev.Stage {
		case "skipped":
			if !opts.Quiet {
				fmt.Println(ev.Message)
				if rst, ok := ev.Details["reset_time"].(string); ok && rst != "" {
					if rIn, ok := ev.Details["resets_in"].(string); ok {
						fmt.Printf("Current cycle resets in %s (%s).\n", rIn, rst)
					}
				}
			}
		case "ready":
			if !opts.Quiet {
				fmt.Println(ev.Message)
			}
		case "jitter_waiting":
			if !opts.Quiet {
				fmt.Println(ev.Message)
			}
		case "aborted":
			if !opts.Quiet {
				fmt.Println(ev.Message)
			}
		case "primed":
			if !opts.Quiet {
				if linked, _ := ev.Details["linked"].(bool); linked {
					fmt.Println(ev.Message)
				} else {
					model, _ := ev.Details["model"].(string)
					prompt, _ := ev.Details["prompt"].(string)
					cascadeID, _ := ev.Details["cascade_id"].(string)
					fmt.Printf("\n✓ %s\n", ev.Message)
					fmt.Printf("  Model: %s (minimum token cost)\n", model)
					fmt.Printf("  Prompt: \"%s\"\n", prompt)
					fmt.Printf("  Cascade ID: %s\n", cascadeID)
					fmt.Println("  Reset countdown has officially started!")
				}
			} else {
				model, _ := ev.Details["model"].(string)
				prompt, _ := ev.Details["prompt"].(string)
				fmt.Printf("[%s] Primed %s [%s] with %s: \"%s\"\n",
					time.Now().UTC().Format("2006-01-02 15:04:05 UTC"), ev.Profile, ev.Bucket, model, prompt)
			}
		case "error":
			if !opts.Quiet {
				fmt.Fprintln(os.Stderr, ev.Message)
			}
		}
	}

	_, err := ExecutePrime(opts, cliCallback)
	if err != nil {
		if !opts.Quiet && opts.Profile != "" {
			fmt.Fprintf(os.Stderr, "Error: Language server is not running for profile '%s', and executable not found.\n", opts.Profile)
			fmt.Fprintf(os.Stderr, "Please launch the profile first: multigravity %s\n", opts.Profile)
		}
		return err
	}

	return nil
}
