package alert

import (
	"fmt"
	"sort"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/headless"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

const (
	KindQuotaCritical        = "quota_critical"
	KindQuotaNearingDepletion = "quota_nearing_depletion"
	KindQuotaDrop            = "quota_drop"
	KindTokenBudgetExceeded  = "token_budget_exceeded"
	KindTokenBurnRateHigh    = "token_burn_rate_high"
	KindOrphanProcess        = "orphan_process"
)

// Alert is one proactive notification. Quota kinds read the stored series
// and the same remaining-fraction line used by warm-up. Orphan kind is the
// headless manager reap of a dead PID. Token kinds track burn rate and budget.
type Alert struct {
	Kind              string   `json:"kind"`
	Profile           string   `json:"profile"`
	BucketID          string   `json:"bucket_id,omitempty"`
	Message           string   `json:"message"`
	RemainingFraction *float64 `json:"remaining_fraction,omitempty"`
	PreviousFraction  *float64 `json:"previous_fraction,omitempty"`
	PID               int      `json:"pid,omitempty"`
	TokensBurned      int      `json:"tokens_burned,omitempty"`
	TokenBurnRate     float64  `json:"token_burn_rate,omitempty"`
	BudgetLimit       int      `json:"budget_limit,omitempty"`
	ObservedAt        string   `json:"observed_at"`
}

// Report is the machine-readable alert set for one profile or the fleet.
type Report struct {
	Alerts []Alert `json:"alerts"`
}

// Evaluate reads quota history and reaps stale headless state.
// An empty profileName scans every profile.
func Evaluate(profileName string) (*Report, error) {
	names, err := profileNames(profileName)
	if err != nil {
		return nil, err
	}
	throttleCfg, _ := quota.LoadThrottleConfig()
	if throttleCfg == nil {
		throttleCfg = quota.DefaultThrottleConfig()
	}

	report := &Report{Alerts: []Alert{}}
	mgr := headless.NewManager()
	now := time.Now()
	for _, name := range names {
		policy := throttleCfg.PolicyForProfile(name)

		quotaAlerts, err := quotaAlertsFor(name, policy)
		if err != nil {
			return nil, err
		}
		report.Alerts = append(report.Alerts, quotaAlerts...)

		tokenAlerts, err := tokenAlertsFor(name, policy, now)
		if err != nil {
			return nil, err
		}
		report.Alerts = append(report.Alerts, tokenAlerts...)

		reaped, err := mgr.ReapStale(name)
		if err != nil {
			return nil, err
		}
		if reaped != nil {
			report.Alerts = append(report.Alerts, Alert{
				Kind:       KindOrphanProcess,
				Profile:    name,
				Message:    fmt.Sprintf("headless process %d exited and its state file was reaped", reaped.PID),
				PID:        reaped.PID,
				ObservedAt: time.Now().UTC().Format(time.RFC3339),
			})
		}
	}
	sort.Slice(report.Alerts, func(i, j int) bool {
		a, b := report.Alerts[i], report.Alerts[j]
		if a.Profile != b.Profile {
			return a.Profile < b.Profile
		}
		if kindRank(a.Kind) != kindRank(b.Kind) {
			return kindRank(a.Kind) < kindRank(b.Kind)
		}
		return a.BucketID < b.BucketID
	})
	return report, nil
}

func profileNames(profileName string) ([]string, error) {
	if profileName == "" {
		return profile.ListProfiles()
	}
	if err := config.ValidateProfileName(profileName); err != nil {
		return nil, err
	}
	if !profile.ProfileExists(profileName) {
		return nil, fmt.Errorf("profile %q does not exist", profileName)
	}
	return []string{profileName}, nil
}

func quotaAlertsFor(profileName string, policy quota.ThrottlePolicy) ([]Alert, error) {
	series, err := quota.LoadSeries(profileName, quota.HistoryQuery{
		Source: quota.SourceQuota,
		Limit:  2,
	})
	if err != nil {
		return nil, err
	}
	samples := series.Samples
	if len(samples) == 0 {
		return nil, nil
	}
	latest := samples[len(samples)-1]
	var previous *quota.Sample
	if len(samples) >= 2 {
		previous = &samples[len(samples)-2]
	}
	prevByID := map[string]quota.BucketPoint{}
	if previous != nil {
		for _, bucket := range previous.Buckets {
			prevByID[bucket.BucketID] = bucket
		}
	}
	observed := latest.Timestamp.UTC().Format(time.RFC3339)
	depletionThreshold := policy.EffectiveDepletionThreshold()
	var alerts []Alert
	for _, bucket := range latest.Buckets {
		if bucket.RemainingFraction <= quota.MinRemainingFraction {
			remaining := bucket.RemainingFraction
			alerts = append(alerts, Alert{
				Kind:              KindQuotaCritical,
				Profile:           profileName,
				BucketID:          bucket.BucketID,
				Message:           fmt.Sprintf("%s remaining quota is %.0f%% (threshold %.0f%%)", bucket.BucketID, remaining*100, quota.MinRemainingFraction*100),
				RemainingFraction: &remaining,
				ObservedAt:        observed,
			})
		} else if bucket.RemainingFraction <= depletionThreshold {
			remaining := bucket.RemainingFraction
			alerts = append(alerts, Alert{
				Kind:              KindQuotaNearingDepletion,
				Profile:           profileName,
				BucketID:          bucket.BucketID,
				Message:           fmt.Sprintf("%s remaining quota is %.0f%% (nearing depletion threshold %.0f%%)", bucket.BucketID, remaining*100, depletionThreshold*100),
				RemainingFraction: &remaining,
				ObservedAt:        observed,
			})
		}
		prev, ok := prevByID[bucket.BucketID]
		if !ok {
			continue
		}
		if prev.RemainingFraction-bucket.RemainingFraction >= quota.MinRemainingFraction {
			remaining := bucket.RemainingFraction
			before := prev.RemainingFraction
			alerts = append(alerts, Alert{
				Kind:              KindQuotaDrop,
				Profile:           profileName,
				BucketID:          bucket.BucketID,
				Message:           fmt.Sprintf("%s remaining quota fell from %.0f%% to %.0f%%", bucket.BucketID, before*100, remaining*100),
				RemainingFraction: &remaining,
				PreviousFraction:  &before,
				ObservedAt:        observed,
			})
		}
	}
	return alerts, nil
}

func tokenAlertsFor(profileName string, policy quota.ThrottlePolicy, now time.Time) ([]Alert, error) {
	burn, err := quota.CalculateTokenBurnRate(profileName, time.Hour, now)
	if err != nil {
		return nil, err
	}
	if burn.TokensBurned == 0 {
		return nil, nil
	}
	observed := now.UTC().Format(time.RFC3339)
	var alerts []Alert
	if policy.MaxTokensPerHour > 0 && burn.TokensBurned >= policy.MaxTokensPerHour {
		alerts = append(alerts, Alert{
			Kind:          KindTokenBudgetExceeded,
			Profile:       profileName,
			Message:       fmt.Sprintf("hourly token budget exceeded: burned %d tokens in past hour (limit %d)", burn.TokensBurned, policy.MaxTokensPerHour),
			TokensBurned:  burn.TokensBurned,
			TokenBurnRate: burn.TokensPerHour,
			BudgetLimit:   policy.MaxTokensPerHour,
			ObservedAt:    observed,
		})
	}
	if policy.MaxTokensPerMinute > 0 && burn.TokensPerMinute >= float64(policy.MaxTokensPerMinute) {
		alerts = append(alerts, Alert{
			Kind:          KindTokenBurnRateHigh,
			Profile:       profileName,
			Message:       fmt.Sprintf("token burn rate high: %.1f tokens/min (limit %d)", burn.TokensPerMinute, policy.MaxTokensPerMinute),
			TokensBurned:  burn.TokensBurned,
			TokenBurnRate: burn.TokensPerMinute,
			BudgetLimit:   policy.MaxTokensPerMinute,
			ObservedAt:    observed,
		})
	}
	return alerts, nil
}

func kindRank(kind string) int {
	switch kind {
	case KindQuotaCritical:
		return 0
	case KindQuotaNearingDepletion:
		return 1
	case KindQuotaDrop:
		return 2
	case KindTokenBudgetExceeded:
		return 3
	case KindTokenBurnRateHigh:
		return 4
	case KindOrphanProcess:
		return 5
	default:
		return 6
	}
}
