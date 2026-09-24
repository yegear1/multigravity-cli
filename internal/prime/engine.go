package prime

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

type PrimeOptions struct {
	Profile          string
	Force            bool
	Check            bool
	Status           bool
	NoJitter         bool
	MaxJitter        float64
	Include5h        bool
	Quiet            bool
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

	// 3. Load state and prompts
	state, err := LoadState()
	if err != nil {
		state = &PrimeState{Profiles: make(map[string]ProfileState)}
	}
	prompts := LoadPrompts()

	// 4. Discover active server or start ephemeral headless server
	client := quota.NewClient(10 * time.Second)
	var port int
	var csrf string
	var pid int
	var headless *quota.HeadlessInstance
	var quotaData *quota.QuotaSummaryResponse

	activeServers, _ := quota.FindActiveServers(opts.Profile)
	for _, s := range activeServers {
		if opts.Profile == "" || s.Profile == opts.Profile {
			port = s.Port
			csrf = s.CSRF
			pid = s.PID
			quotaData = s.Data
			break
		}
	}

	if port == 0 {
		var hErr error
		headless, hErr = quota.StartHeadlessServer(opts.Profile)
		if hErr != nil {
			if !opts.Quiet {
				fmt.Fprintf(os.Stderr, "Error: Language server is not running for profile '%s', and executable not found.\n", opts.Profile)
				fmt.Fprintf(os.Stderr, "Please launch the profile first: multigravity %s\n", opts.Profile)
			}
			return hErr
		}
		defer headless.Close()

		port = headless.Port
		csrf = headless.CSRF
		pid = headless.PID

		var qErr error
		quotaData, qErr = client.RetrieveUserQuotaSummary(port, csrf)
		if qErr != nil {
			if !opts.Quiet {
				fmt.Fprintf(os.Stderr, "Error querying quota summary: %v\n", qErr)
			}
			return qErr
		}
	}

	buckets := make(map[string]quota.QuotaBucket)
	for _, g := range quotaData.Response.Groups {
		for _, b := range g.Buckets {
			if b.BucketID == "gemini-weekly" || b.BucketID == "3p-weekly" || b.BucketID == "gemini-5h" || b.BucketID == "3p-5h" {
				buckets[b.BucketID] = b
			}
		}
	}

	if len(buckets) == 0 {
		if !opts.Quiet {
			fmt.Fprintln(os.Stderr, "Error: No quota buckets found in telemetry.")
		}
		return fmt.Errorf("no quota buckets found in telemetry")
	}

	profKey := opts.Profile
	if profKey == "" {
		profKey = "default"
	}
	profState := state.Profiles[profKey]
	if profState == nil {
		profState = make(ProfileState)
	}

	now := time.Now().UTC()

	// 5. Action: Status
	if opts.Status {
		serverMode := fmt.Sprintf("Active IDE Instance (PID %d, Port %d)", pid, port)
		if headless != nil {
			serverMode = "Headless (Standby)"
		}

		fmt.Printf("\nPrime Status — Profile: %s\n", profKey)
		fmt.Printf("%s\n", "============================================================")
		fmt.Printf("  Language Server:    %s\n", serverMode)

		for _, cfg := range BucketConfigs {
			b, hasBucket := buckets[cfg.BucketID]
			bState := profState[cfg.Key]

			fmt.Printf("\n  • %s (%s):\n", cfg.Name, cfg.BucketID)
			if !hasBucket {
				fmt.Println("    Status:           Not available in telemetry")
				continue
			}

			remFrac := b.RemainingFraction
			timeLeftStr, dSec := quota.FormatResetCountdown(b.ResetTime, now)
			tLeft := "Refreshed!"
			if dSec > 0 {
				tLeft = fmt.Sprintf("%dh %dm", dSec/3600, (dSec%3600)/60)
			}

			lastP := bState.LastPrimedAt
			if lastP == "" {
				lastP = "Never"
			}
			lastM := bState.LastModel
			if lastM == "" {
				lastM = "-"
			}
			promptInfo := ""
			if bState.LastPrompt != "" {
				promptInfo = fmt.Sprintf(`, prompt: "%s"`, bState.LastPrompt)
			}
			lastRst := bState.LastResetTime
			if lastRst == "" {
				lastRst = "-"
			}

			isRef := (remFrac >= 0.999) || (dSec <= 0)
			alrPrimed := (lastRst == b.ResetTime && !isRef)

			cStatus := "Active (Countdown running)"
			if alrPrimed {
				cStatus = fmt.Sprintf("Active (%s countdown is running)", cfg.Period)
			} else if isRef {
				if bState.TargetPrimeTime != "" {
					cStatus = fmt.Sprintf("Pending Prime with Jitter (Scheduled at: %s)", bState.TargetPrimeTime)
				} else {
					cStatus = "Ready to Prime (Reset occurred / New cycle waiting to start)"
				}
			}

			fmt.Printf("    Limit Quota:      %.1f%% remaining | Resets in: %s\n", remFrac*100, tLeft)
			fmt.Printf("    Reset Target:     %s\n", b.ResetTime)
			fmt.Printf("    Last Primed:      %s (model: %s%s)\n", lastP, lastM, promptInfo)
			fmt.Printf("    Cycle Status:     %s\n", cStatus)
			_ = timeLeftStr
		}

		cronDesc, sysDesc := CheckWatchdogStatus(profKey)
		fmt.Printf("\n  Scheduled Watchdog:\n")
		fmt.Printf("    Crontab:          %s\n", cronDesc)
		fmt.Printf("    Systemd Timer:    %s\n", sysDesc)
		fmt.Printf("    Prompts Catalog:  %s (%d prompts loaded)\n\n", GetPromptsFilePath(), len(prompts))
		return nil
	}

	// 6. Action: Prime
	usedPrompts := make(map[string]bool)
	primedProviders := make(map[string]bool)

	for _, cfg := range BucketConfigs {
		if cfg.Period == "5h" && !opts.Include5h && !opts.Force {
			continue
		}

		b, ok := buckets[cfg.BucketID]
		if !ok {
			continue
		}

		// Protection: if 5h bucket, check if parent weekly quota has remaining quota (> 5%)
		if cfg.ParentKey != "" {
			parentB, hasParent := buckets[cfg.ParentKey+"-weekly"]
			if hasParent {
				if parentB.RemainingFraction <= 0.05 && !opts.Force {
					if !opts.Quiet {
						fmt.Printf("Skipping 5h prime for %s: weekly quota is exhausted (%.1f%% remaining).\n", cfg.Name, parentB.RemainingFraction*100)
					}
					continue
				}
			}
		}

		remFrac := b.RemainingFraction
		rstStr := b.ResetTime
		_, dSec := quota.FormatResetCountdown(rstStr, now)
		tLeft := "Refreshed!"
		if dSec > 0 {
			tLeft = fmt.Sprintf("%dh %dm", dSec/3600, (dSec%3600)/60)
		}

		isRef := (remFrac >= 0.999) || (dSec <= 0)
		bState := profState[cfg.Key]
		alrPrimed := (bState.LastResetTime == rstStr && remFrac < 0.999)

		if !opts.Force {
			if alrPrimed {
				if !opts.Quiet {
					fmt.Printf("Quota for '%s' [%s] is already primed and active for this cycle.\n", profKey, cfg.Name)
					fmt.Printf("Current cycle resets in %s (%s).\n", tLeft, rstStr)
				}
				continue
			}

			if !isRef {
				if !opts.Quiet {
					fmt.Printf("Quota for '%s' [%s] has not reset yet (%.1f%% remaining, resets in %s).\n", profKey, cfg.Name, remFrac*100, tLeft)
				}
				continue
			}

			if opts.Check {
				if !opts.Quiet {
					fmt.Printf("Quota for '%s' [%s] is READY for priming (Reset occurred / New cycle waiting).\n", profKey, cfg.Name)
				}
				continue
			}

			// Check if this model provider was already primed in this session
			if primedProviders[cfg.Model] {
				bState.LastPrimedAt = time.Now().UTC().Format(time.RFC3339)
				bState.LastResetTime = rstStr
				bState.LastModel = cfg.ModelLabel
				bState.LastPrompt = fmt.Sprintf("(Linked with %s prime)", cfg.ParentKey)
				bState.TargetPrimeTime = ""
				bState.Status = "success"
				profState[cfg.Key] = bState
				state.Profiles[profKey] = profState
				_ = SaveState(state)
				if !opts.Quiet {
					fmt.Printf("✓ 5-Hour window for '%s' [%s] auto-linked with previous prime in this session.\n", profKey, cfg.Name)
				}
				continue
			}

			// Anti-bot jitter per bucket
			maxJitter := opts.MaxJitter
			if maxJitter <= 0 {
				maxJitter = 60.0
			}
			bucketMaxJitter := maxJitter
			if cfg.Period == "5h" {
				bucketMaxJitter = math.Min(maxJitter, 15.0)
			}

			if !opts.NoJitter && bucketMaxJitter > 0 {
				var targetTime time.Time
				if bState.TargetPrimeTime != "" {
					targetTime, _ = time.Parse(time.RFC3339, bState.TargetPrimeTime)
				}

				if targetTime.IsZero() || targetTime.Before(now.Add(-2*time.Hour)) {
					r := rand.New(rand.NewSource(time.Now().UnixNano()))
					jitterSec := r.Float64() * bucketMaxJitter * 60.0
					targetTime = now.Add(time.Duration(jitterSec) * time.Second)
					bState.TargetPrimeTime = targetTime.Format(time.RFC3339)
					profState[cfg.Key] = bState
					state.Profiles[profKey] = profState
					_ = SaveState(state)
				}

				if now.Before(targetTime) {
					waitSec := int(targetTime.Sub(now).Seconds())
					if opts.Quiet {
						continue
					}
					fmt.Printf("Anti-bot jitter for %s: waiting %dm %ds before priming (scheduled for %s UTC)...\n",
						cfg.Name, waitSec/60, waitSec%60, targetTime.Format("15:04:05"))
					time.Sleep(time.Duration(waitSec) * time.Second)
				}
			}
		}

		// Last-mile pre-prime check
		manualActivity := false
		if chkData, err := client.RetrieveUserQuotaSummary(port, csrf); err == nil {
			for _, cg := range chkData.Response.Groups {
				for _, cb := range cg.Buckets {
					if cb.BucketID == cfg.BucketID {
						freshRem := cb.RemainingFraction
						freshRst := cb.ResetTime
						if !opts.Force && freshRem < 0.999 && (freshRst != rstStr || freshRem < remFrac-0.005) {
							manualActivity = true
							if !opts.Quiet {
								fmt.Printf("Aborting prime for %s: manual user activity detected (quota now at %.1f%%).\n", cfg.Name, freshRem*100)
							}
							bState.TargetPrimeTime = ""
							profState[cfg.Key] = bState
							state.Profiles[profKey] = profState
							_ = SaveState(state)
							break
						}
					}
				}
				if manualActivity {
					break
				}
			}
		}

		if manualActivity {
			continue
		}

		// Select random prompt
		selectedPrompt := SelectRandomPrompt(prompts, usedPrompts)
		usedPrompts[selectedPrompt] = true

		// Despatch StartCascade + SendUserCascadeMessage
		cascadeID, err := client.StartCascade(port, csrf)
		if err != nil {
			if !opts.Quiet {
				fmt.Fprintf(os.Stderr, "Error priming %s (StartCascade): %v\n", cfg.Name, err)
			}
			continue
		}

		err = client.SendUserCascadeMessage(port, csrf, cascadeID, selectedPrompt, cfg.Model)
		if err != nil {
			if !opts.Quiet {
				fmt.Fprintf(os.Stderr, "Error priming %s (SendUserCascadeMessage): %v\n", cfg.Name, err)
			}
			continue
		}

		// Brief grace delay for response to register in SQLite
		time.Sleep(3 * time.Second)

		bState.LastPrimedAt = time.Now().UTC().Format(time.RFC3339)
		bState.LastResetTime = rstStr
		bState.LastCascadeID = cascadeID
		bState.LastModel = cfg.ModelLabel
		bState.LastPrompt = selectedPrompt
		bState.TargetPrimeTime = ""
		bState.Status = "success"
		profState[cfg.Key] = bState
		state.Profiles[profKey] = profState
		_ = SaveState(state)
		primedProviders[cfg.Model] = true

		if !opts.Quiet {
			fmt.Printf("\n✓ Successfully primed quota for profile '%s' [%s]!\n", profKey, cfg.Name)
			fmt.Printf("  Model: %s (minimum token cost)\n", cfg.ModelLabel)
			fmt.Printf("  Prompt: \"%s\"\n", selectedPrompt)
			fmt.Printf("  Cascade ID: %s\n", cascadeID)
			fmt.Println("  Reset countdown has officially started!")
		} else {
			fmt.Printf("[%s] Primed %s [%s] with %s: \"%s\"\n",
				time.Now().UTC().Format("2006-01-02 15:04:05 UTC"), profKey, cfg.Name, cfg.ModelLabel, selectedPrompt)
		}
	}

	return nil
}
