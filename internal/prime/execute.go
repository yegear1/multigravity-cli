package prime

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

// ExecutePrime evaluates and dispatches priming prompts with progress callbacks
func ExecutePrime(opts PrimeOptions, cb ProgressCallback) (*ProfilePrimeResult, error) {
	profKey := opts.Profile
	if profKey == "" {
		profiles, err := profile.ListProfiles()
		if err == nil && len(profiles) == 1 {
			profKey = profiles[0]
		} else {
			profKey = "default"
		}
	} else if !profile.ProfileExists(profKey) {
		return nil, fmt.Errorf("profile '%s' does not exist", profKey)
	}

	emit := func(stage, bucket, bucketID, msg string, details map[string]any) {
		if cb != nil {
			cb(PrimeProgressEvent{
				Profile:   profKey,
				Bucket:    bucket,
				BucketID:  bucketID,
				Stage:     stage,
				Message:   msg,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Details:   details,
			})
		}
	}

	emit("initializing", "", "", fmt.Sprintf("Starting prime check for profile '%s'", profKey), nil)

	state, err := LoadState()
	if err != nil {
		state = &PrimeState{Profiles: make(map[string]ProfileState)}
	}
	prompts := LoadPrompts()

	client := newQuotaClientFn(10 * time.Second)
	var port int
	var csrf string
	var pid int
	var headless *quota.HeadlessInstance
	var quotaData *quota.QuotaSummaryResponse
	serverMode := "active"

	activeServers, _ := findActiveServersFn(profKey)
	for _, s := range activeServers {
		if s.Profile == profKey || profKey == "" || profKey == "default" {
			port = s.Port
			csrf = s.CSRF
			pid = s.PID
			quotaData = s.Data
			serverMode = fmt.Sprintf("Active IDE Instance (PID %d, Port %d)", pid, port)
			break
		}
	}

	if port == 0 {
		var hErr error
		headless, hErr = startHeadlessFn(profKey)
		if hErr != nil {
			errMsg := fmt.Sprintf("Language server is not running for profile '%s', and executable not found.", profKey)
			emit("error", "", "", errMsg, nil)
			return nil, fmt.Errorf("language server is not running for profile '%s', and executable not found", profKey)
		}
		defer headless.Close()

		port = headless.Port
		csrf = headless.CSRF
		pid = headless.PID
		serverMode = "Headless (Standby)"

		var qErr error
		quotaData, qErr = client.RetrieveUserQuotaSummary(port, csrf)
		if qErr != nil {
			emit("error", "", "", fmt.Sprintf("Error querying quota summary: %v", qErr), nil)
			return nil, qErr
		}
	} else if quotaData == nil {
		var qErr error
		quotaData, qErr = client.RetrieveUserQuotaSummary(port, csrf)
		if qErr != nil {
			emit("error", "", "", fmt.Sprintf("Error querying quota summary: %v", qErr), nil)
			return nil, qErr
		}
	}

	emit("server_ready", "", "", fmt.Sprintf("Language server connected (%s)", serverMode), map[string]any{
		"port": port,
		"pid":  pid,
		"mode": serverMode,
	})

	buckets := make(map[string]quota.QuotaBucket)
	for _, g := range quotaData.Response.Groups {
		for _, b := range g.Buckets {
			if b.BucketID == "gemini-weekly" || b.BucketID == "3p-weekly" || b.BucketID == "gemini-5h" || b.BucketID == "3p-5h" {
				buckets[b.BucketID] = b
			}
		}
	}

	if len(buckets) == 0 {
		errMsg := "no quota buckets found in telemetry"
		emit("error", "", "", errMsg, nil)
		return nil, fmt.Errorf("%s", errMsg)
	}

	profState := state.Profiles[profKey]
	if profState == nil {
		profState = make(ProfileState)
	}

	now := time.Now().UTC()
	usedPrompts := make(map[string]bool)
	primedProviders := make(map[string]bool)

	result := &ProfilePrimeResult{
		Profile:    profKey,
		ServerMode: serverMode,
		PID:        pid,
		Port:       port,
		Buckets:    make([]BucketPrimeResult, 0, len(BucketConfigs)),
	}

	for _, cfg := range BucketConfigs {
		if cfg.Period == "5h" && !opts.Include5h && !opts.Warm5h && !opts.Force {
			continue
		}
		if opts.Warm5h && cfg.Period != "5h" {
			// If warm-5h was explicitly requested, target only 5-hour rolling windows
			continue
		}

		b, ok := buckets[cfg.BucketID]
		if !ok {
			continue
		}

		wType := quota.ClassifyWindow(quota.QuotaBucket{
			BucketID:    cfg.BucketID,
			DisplayName: cfg.Name,
			ResetTime:   b.ResetTime,
		}, now)

		warmType := "cycle_reset"
		if opts.Warm5h || cfg.Period == "5h" {
			warmType = "proactive_5h"
		}

		// Protection: if 5h bucket, check if parent weekly quota has remaining quota (> 5%)
		if cfg.ParentKey != "" {
			parentB, hasParent := buckets[cfg.ParentKey+"-weekly"]
			if hasParent {
				if parentB.RemainingFraction <= 0.05 && !opts.Force {
					msg := fmt.Sprintf("Skipping 5h prime for %s: weekly quota is exhausted (%.1f%% remaining).", cfg.Name, parentB.RemainingFraction*100)
					emit("skipped", cfg.Key, cfg.BucketID, msg, map[string]any{"reason": "weekly_exhausted", "window_type": wType})
					result.Buckets = append(result.Buckets, BucketPrimeResult{
						Key:        cfg.Key,
						BucketID:   cfg.BucketID,
						Name:       cfg.Name,
						Model:      cfg.Model,
						ModelLabel: cfg.ModelLabel,
						Status:     "skipped",
						Reason:     "weekly quota is exhausted",
						WindowType: wType,
						WarmType:   warmType,
					})
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
				msg := fmt.Sprintf("Quota for '%s' [%s] is already primed and active for this cycle.", profKey, cfg.Name)
				emit("skipped", cfg.Key, cfg.BucketID, msg, map[string]any{
					"reason":      "already_primed",
					"resets_in":   tLeft,
					"reset_time":  rstStr,
					"window_type": wType,
					"warm_type":   warmType,
				})
				result.Buckets = append(result.Buckets, BucketPrimeResult{
					Key:        cfg.Key,
					BucketID:   cfg.BucketID,
					Name:       cfg.Name,
					Model:      cfg.Model,
					ModelLabel: cfg.ModelLabel,
					Status:     "skipped",
					Reason:     "already primed and active for this cycle",
					ResetTime:  rstStr,
					WindowType: wType,
					WarmType:   warmType,
				})
				continue
			}

			if !isRef {
				msg := fmt.Sprintf("Quota for '%s' [%s] has not reset yet (%.1f%% remaining, resets in %s).", profKey, cfg.Name, remFrac*100, tLeft)
				emit("skipped", cfg.Key, cfg.BucketID, msg, map[string]any{
					"reason":      "not_reset",
					"remaining":   remFrac,
					"resets_in":   tLeft,
					"window_type": wType,
					"warm_type":   warmType,
				})
				result.Buckets = append(result.Buckets, BucketPrimeResult{
					Key:        cfg.Key,
					BucketID:   cfg.BucketID,
					Name:       cfg.Name,
					Model:      cfg.Model,
					ModelLabel: cfg.ModelLabel,
					Status:     "skipped",
					Reason:     fmt.Sprintf("quota has not reset yet (%.1f%% remaining, resets in %s)", remFrac*100, tLeft),
					ResetTime:  rstStr,
					WindowType: wType,
					WarmType:   warmType,
				})
				continue
			}

			if opts.Check {
				readyMsg := fmt.Sprintf("Quota for '%s' [%s] is READY for priming (Reset occurred / New cycle waiting).", profKey, cfg.Name)
				readyReason := "ready for priming (reset occurred)"
				if opts.Warm5h {
					readyMsg = fmt.Sprintf("Quota for '%s' [%s] is READY for proactive 5-hour warm-up.", profKey, cfg.Name)
					readyReason = "ready for proactive 5h warm-up"
				}
				emit("ready", cfg.Key, cfg.BucketID, readyMsg, map[string]any{
					"ready":       true,
					"window_type": wType,
					"warm_type":   warmType,
				})
				result.Buckets = append(result.Buckets, BucketPrimeResult{
					Key:        cfg.Key,
					BucketID:   cfg.BucketID,
					Name:       cfg.Name,
					Model:      cfg.Model,
					ModelLabel: cfg.ModelLabel,
					Status:     "ready",
					Reason:     readyReason,
					ResetTime:  rstStr,
					WindowType: wType,
					WarmType:   warmType,
				})
				continue
			}

			// Check if this model provider was already primed in this session
			if primedProviders[cfg.Model] {
				primedAt := time.Now().UTC().Format(time.RFC3339)
				bState.LastPrimedAt = primedAt
				bState.LastResetTime = rstStr
				bState.LastModel = cfg.ModelLabel
				bState.LastPrompt = fmt.Sprintf("(Linked with %s prime)", cfg.ParentKey)
				bState.TargetPrimeTime = ""
				bState.Status = "success"
				profState[cfg.Key] = bState
				state.Profiles[profKey] = profState
				_ = SaveState(state)

				msg := fmt.Sprintf("✓ 5-Hour window for '%s' [%s] auto-linked with previous prime in this session.", profKey, cfg.Name)
				emit("linked", cfg.Key, cfg.BucketID, msg, map[string]any{
					"linked":      true,
					"window_type": wType,
					"warm_type":   warmType,
				})
				result.Buckets = append(result.Buckets, BucketPrimeResult{
					Key:        cfg.Key,
					BucketID:   cfg.BucketID,
					Name:       cfg.Name,
					Model:      cfg.Model,
					ModelLabel: cfg.ModelLabel,
					Status:     "linked",
					Reason:     fmt.Sprintf("auto-linked with previous %s prime", cfg.ParentKey),
					Prompt:     fmt.Sprintf("(Linked with %s prime)", cfg.ParentKey),
					ResetTime:  rstStr,
					PrimedAt:   primedAt,
					WindowType: wType,
					WarmType:   warmType,
				})
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
					msg := fmt.Sprintf("Anti-bot jitter for %s: waiting %dm %ds before priming (scheduled for %s UTC)...",
						cfg.Name, waitSec/60, waitSec%60, targetTime.Format("15:04:05"))
					emit("jitter_waiting", cfg.Key, cfg.BucketID, msg, map[string]any{
						"wait_seconds": waitSec,
						"target_time":  targetTime.Format(time.RFC3339),
						"window_type":  wType,
						"warm_type":    warmType,
					})
					time.Sleep(time.Duration(waitSec) * time.Second)
				}
			}
		}

		// Last-mile pre-prime check
		manualActivity := false
		if chkData, err := client.RetrieveUserQuotaSummary(port, csrf); err == nil && chkData != nil {
			for _, cg := range chkData.Response.Groups {
				for _, cb := range cg.Buckets {
					if cb.BucketID == cfg.BucketID {
						freshRem := cb.RemainingFraction
						freshRst := cb.ResetTime
						if !opts.Force && freshRem < 0.999 && (freshRst != rstStr || freshRem < remFrac-0.005) {
							manualActivity = true
							msg := fmt.Sprintf("Aborting prime for %s: manual user activity detected (quota now at %.1f%%).", cfg.Name, freshRem*100)
							emit("aborted", cfg.Key, cfg.BucketID, msg, map[string]any{
								"reason":      "manual_activity",
								"window_type": wType,
								"warm_type":   warmType,
							})
							result.Buckets = append(result.Buckets, BucketPrimeResult{
								Key:        cfg.Key,
								BucketID:   cfg.BucketID,
								Name:       cfg.Name,
								Model:      cfg.Model,
								ModelLabel: cfg.ModelLabel,
								Status:     "aborted",
								Reason:     "manual user activity detected",
								ResetTime:  freshRst,
								WindowType: wType,
								WarmType:   warmType,
							})
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

		dispatchMsg := fmt.Sprintf("Dispatching prime prompt to %s for %s", cfg.ModelLabel, cfg.Name)
		if warmType == "proactive_5h" {
			dispatchMsg = fmt.Sprintf("Dispatching proactive 5h warm-up prompt to %s for %s", cfg.ModelLabel, cfg.Name)
		}
		emit("dispatching", cfg.Key, cfg.BucketID, dispatchMsg, map[string]any{
			"model":       cfg.ModelLabel,
			"prompt":      selectedPrompt,
			"window_type": wType,
			"warm_type":   warmType,
		})

		cascadeID, err := client.StartCascade(port, csrf)
		if err != nil {
			msg := fmt.Sprintf("Error priming %s (StartCascade): %v", cfg.Name, err)
			emit("error", cfg.Key, cfg.BucketID, msg, map[string]any{
				"window_type": wType,
				"warm_type":   warmType,
			})
			result.Buckets = append(result.Buckets, BucketPrimeResult{
				Key:        cfg.Key,
				BucketID:   cfg.BucketID,
				Name:       cfg.Name,
				Model:      cfg.Model,
				ModelLabel: cfg.ModelLabel,
				Status:     "error",
				Reason:     fmt.Sprintf("StartCascade failed: %v", err),
				WindowType: wType,
				WarmType:   warmType,
			})
			continue
		}

		err = client.SendUserCascadeMessage(port, csrf, cascadeID, selectedPrompt, cfg.Model)
		if err != nil {
			msg := fmt.Sprintf("Error priming %s (SendUserCascadeMessage): %v", cfg.Name, err)
			emit("error", cfg.Key, cfg.BucketID, msg, map[string]any{
				"window_type": wType,
				"warm_type":   warmType,
			})
			result.Buckets = append(result.Buckets, BucketPrimeResult{
				Key:        cfg.Key,
				BucketID:   cfg.BucketID,
				Name:       cfg.Name,
				Model:      cfg.Model,
				ModelLabel: cfg.ModelLabel,
				Status:     "error",
				Reason:     fmt.Sprintf("SendUserCascadeMessage failed: %v", err),
				WindowType: wType,
				WarmType:   warmType,
			})
			continue
		}

		// Brief grace delay for response to register in SQLite
		time.Sleep(graceDelay)

		primedAt := time.Now().UTC().Format(time.RFC3339)
		bState.LastPrimedAt = primedAt
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

		primeSuccessMsg := fmt.Sprintf("Successfully primed quota for profile '%s' [%s]!", profKey, cfg.Name)
		if warmType == "proactive_5h" {
			primeSuccessMsg = fmt.Sprintf("Successfully warmed 5-hour quota window for profile '%s' [%s]!", profKey, cfg.Name)
		}
		emit("primed", cfg.Key, cfg.BucketID, primeSuccessMsg, map[string]any{
			"model":       cfg.ModelLabel,
			"prompt":      selectedPrompt,
			"cascade_id":  cascadeID,
			"window_type": wType,
			"warm_type":   warmType,
		})

		result.Buckets = append(result.Buckets, BucketPrimeResult{
			Key:        cfg.Key,
			BucketID:   cfg.BucketID,
			Name:       cfg.Name,
			Model:      cfg.Model,
			ModelLabel: cfg.ModelLabel,
			Status:     "primed",
			Prompt:     selectedPrompt,
			CascadeID:  cascadeID,
			ResetTime:  rstStr,
			PrimedAt:   primedAt,
			WindowType: wType,
			WarmType:   warmType,
		})
	}

	emit("completed", "", "", fmt.Sprintf("Prime execution completed for profile '%s'", profKey), nil)
	return result, nil
}
