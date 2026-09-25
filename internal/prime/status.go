package prime

import (
	"fmt"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

// GetProfilePrimeStatus queries the prime and cycle status of a single profile
func GetProfilePrimeStatus(profileName string) (*ProfilePrimeStatus, error) {
	profKey := profileName
	if profKey == "" {
		profiles, err := profile.ListProfiles()
		if err == nil && len(profiles) == 1 {
			profKey = profiles[0]
		} else if len(profiles) > 1 {
			return nil, fmt.Errorf("multiple profiles exist; please specify profile name")
		} else {
			profKey = "default"
		}
	} else if !profile.ProfileExists(profKey) {
		return nil, fmt.Errorf("profile '%s' does not exist", profKey)
	}

	state, err := LoadState()
	if err != nil {
		state = &PrimeState{Profiles: make(map[string]ProfileState)}
	}
	prompts := LoadPrompts()

	profState := state.Profiles[profKey]
	if profState == nil {
		profState = make(ProfileState)
	}

	now := time.Now().UTC()

	var port int
	var csrf string
	var pid int
	var headless *quota.HeadlessInstance
	var quotaData *quota.QuotaSummaryResponse
	serverMode := "offline"
	var serverErr string

	client := newQuotaClientFn(10 * time.Second)
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
			serverErr = hErr.Error()
		} else {
			defer headless.Close()
			port = headless.Port
			csrf = headless.CSRF
			pid = headless.PID
			serverMode = "Headless (Standby)"

			var qErr error
			quotaData, qErr = client.RetrieveUserQuotaSummary(port, csrf)
			if qErr != nil {
				serverErr = qErr.Error()
			}
		}
	} else if quotaData == nil {
		quotaData, _ = client.RetrieveUserQuotaSummary(port, csrf)
	}

	buckets := make(map[string]quota.QuotaBucket)
	if quotaData != nil && quotaData.Response.Groups != nil {
		for _, g := range quotaData.Response.Groups {
			for _, b := range g.Buckets {
				if b.BucketID == "gemini-weekly" || b.BucketID == "3p-weekly" || b.BucketID == "gemini-5h" || b.BucketID == "3p-5h" {
					buckets[b.BucketID] = b
				}
			}
		}
	}

	cronDesc, sysDesc := CheckWatchdogStatus(profKey)

	status := &ProfilePrimeStatus{
		Profile:        profKey,
		ServerMode:     serverMode,
		PID:            pid,
		Port:           port,
		CrontabDesc:    cronDesc,
		SystemdDesc:    sysDesc,
		PromptsCatalog: GetPromptsFilePath(),
		PromptsCount:   len(prompts),
		Error:          serverErr,
		Buckets:        make([]BucketStatusReport, 0, len(BucketConfigs)),
	}

	for _, cfg := range BucketConfigs {
		b, hasBucket := buckets[cfg.BucketID]
		bState := profState[cfg.Key]

		var parentB *quota.QuotaBucket
		if cfg.ParentKey != "" {
			if pb, ok := buckets[cfg.ParentKey+"-weekly"]; ok {
				parentB = &pb
			}
		}

		wType := quota.ClassifyWindow(quota.QuotaBucket{
			BucketID:    cfg.BucketID,
			DisplayName: cfg.Name,
			ResetTime:   b.ResetTime,
		}, now)

		canWarm := false
		if hasBucket {
			canWarm = quota.CanWarm5hWindow(b, parentB, now)
		}

		rep := BucketStatusReport{
			Key:             cfg.Key,
			BucketID:        cfg.BucketID,
			Name:            cfg.Name,
			Period:          cfg.Period,
			Model:           cfg.Model,
			ModelLabel:      cfg.ModelLabel,
			Available:       hasBucket,
			WindowType:      wType,
			CanWarm:         canWarm,
			LastPrimedAt:    bState.LastPrimedAt,
			LastModel:       bState.LastModel,
			LastPrompt:      bState.LastPrompt,
			LastResetTime:   bState.LastResetTime,
			TargetPrimeTime: bState.TargetPrimeTime,
		}

		if hasBucket {
			rep.RemainingFraction = b.RemainingFraction
			rep.RemainingPercent = b.RemainingFraction * 100.0
			rep.ResetTime = b.ResetTime

			_, dSec := quota.FormatResetCountdown(b.ResetTime, now)
			rep.SecondsUntilReset = int64(dSec)
			if dSec > 0 {
				rep.TimeLeft = fmt.Sprintf("%dh %dm", dSec/3600, (dSec%3600)/60)
			} else {
				rep.TimeLeft = "Refreshed!"
			}

			isRef := (b.RemainingFraction >= 0.999) || (dSec <= 0)
			rep.IsRefreshed = isRef

			alrPrimed := (bState.LastResetTime == b.ResetTime && !isRef)
			if alrPrimed {
				rep.CycleStatus = fmt.Sprintf("Active (%s countdown is running)", cfg.Period)
			} else if isRef {
				if bState.TargetPrimeTime != "" {
					rep.CycleStatus = fmt.Sprintf("Pending Prime with Jitter (Scheduled at: %s)", bState.TargetPrimeTime)
				} else if canWarm {
					rep.CycleStatus = "Ready to Warm (Refreshed / Proactive 5h warm-up available)"
				} else {
					rep.CycleStatus = "Ready to Prime (Reset occurred / New cycle waiting to start)"
				}
			} else {
				rep.CycleStatus = "Active (Countdown running)"
			}
		}

		status.Buckets = append(status.Buckets, rep)
	}

	return status, nil
}

// GetAllProfilesPrimeStatus queries the prime status across all known profiles
func GetAllProfilesPrimeStatus() ([]ProfilePrimeStatus, error) {
	profiles, err := profile.ListProfiles()
	if err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		st, err := GetProfilePrimeStatus("default")
		if err != nil {
			return []ProfilePrimeStatus{}, nil
		}
		return []ProfilePrimeStatus{*st}, nil
	}

	results := make([]ProfilePrimeStatus, 0, len(profiles))
	for _, p := range profiles {
		st, err := GetProfilePrimeStatus(p)
		if err != nil {
			continue
		}
		results = append(results, *st)
	}
	return results, nil
}

// RenderStatusText formats a ProfilePrimeStatus into the canonical human-readable CLI text
func RenderStatusText(st *ProfilePrimeStatus) {
	fmt.Printf("\nPrime Status — Profile: %s\n", st.Profile)
	fmt.Printf("%s\n", "============================================================")
	fmt.Printf("  Language Server:    %s\n", st.ServerMode)

	for _, b := range st.Buckets {
		fmt.Printf("\n  • %s (%s):\n", b.Name, b.BucketID)
		if !b.Available {
			fmt.Println("    Status:           Not available in telemetry")
			continue
		}

		lastP := b.LastPrimedAt
		if lastP == "" {
			lastP = "Never"
		}
		lastM := b.LastModel
		if lastM == "" {
			lastM = "-"
		}
		promptInfo := ""
		if b.LastPrompt != "" {
			promptInfo = fmt.Sprintf(`, prompt: "%s"`, b.LastPrompt)
		}

		fmt.Printf("    Limit Quota:      %.1f%% remaining | Resets in: %s\n", b.RemainingPercent, b.TimeLeft)
		fmt.Printf("    Reset Target:     %s\n", b.ResetTime)
		fmt.Printf("    Last Primed:      %s (model: %s%s)\n", lastP, lastM, promptInfo)
		fmt.Printf("    Cycle Status:     %s\n", b.CycleStatus)
		if b.CanWarm {
			fmt.Printf("    Proactive 5h:     Available (multigravity prime %s --warm-5h)\n", st.Profile)
		}
	}

	fmt.Printf("\n  Scheduled Watchdog:\n")
	fmt.Printf("    Crontab:          %s\n", st.CrontabDesc)
	fmt.Printf("    Systemd Timer:    %s\n", st.SystemdDesc)
	fmt.Printf("    Prompts Catalog:  %s (%d prompts loaded)\n\n", st.PromptsCatalog, st.PromptsCount)
}
