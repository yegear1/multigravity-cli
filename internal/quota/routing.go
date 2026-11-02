package quota

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

// RouteWeight represents the task complexity level for quota routing
type RouteWeight string

const (
	WeightLight  RouteWeight = "light"
	WeightMedium RouteWeight = "medium"
	WeightHeavy  RouteWeight = "heavy"

	// CriticalQuotaThreshold defines the depleted quota floor fraction (1%).
	CriticalQuotaThreshold = 0.01

	// DefaultSafetyQuotaThreshold defines the safety quota floor fraction (1%) when minFraction <= 0.
	DefaultSafetyQuotaThreshold = 0.01
)

// SafetyThresholdForModel returns the safety quota floor fraction for a given model and weight.
// It returns 0.15 for TierReasoning, TierArchitecture, or WeightHeavy, and 0.01 (DefaultSafetyQuotaThreshold) otherwise.
func SafetyThresholdForModel(model string, weight RouteWeight) float64 {
	if NormalizeWeight(weight) == WeightHeavy {
		return 0.15
	}
	if strings.TrimSpace(model) == "" {
		return DefaultSafetyQuotaThreshold
	}
	switch TierForModel(model) {
	case TierReasoning, TierArchitecture:
		return 0.15
	default:
		return DefaultSafetyQuotaThreshold
	}
}

// NormalizeWeight normalizes and validates an empty or custom weight to RouteWeight.
func NormalizeWeight(w RouteWeight) RouteWeight {
	if w == "" {
		return WeightMedium
	}
	return w
}

// ParseRouteWeight normalizes and validates a weight parameter
func ParseRouteWeight(s string) (RouteWeight, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(WeightLight):
		return WeightLight, nil
	case string(WeightMedium), "":
		return WeightMedium, nil
	case string(WeightHeavy):
		return WeightHeavy, nil
	default:
		return "", fmt.Errorf("invalid weight '%s': must be light, medium, or heavy", s)
	}
}

// RouteCandidate represents an evaluated profile candidate for a workload
type RouteCandidate struct {
	Profile           string  `json:"profile"`
	Model             string  `json:"model"`
	Weight            string  `json:"weight"`
	RemainingFraction float64 `json:"remaining_fraction"`
	ResetTime         string  `json:"reset_time,omitempty"`
	WindowType        string  `json:"window_type,omitempty"`
	Score             float64 `json:"score"`
	Eligible          bool    `json:"eligible"`
	Reason            string  `json:"reason"`
}

// RoutePlan represents the full recommendation and candidate rankings
type RoutePlan struct {
	Recommended       *RouteCandidate  `json:"recommended"`
	Candidates        []RouteCandidate `json:"candidates"`
	Weight            string           `json:"weight"`
	RecommendedEffort string           `json:"recommended_effort,omitempty"`
	EvaluatedAt       string           `json:"evaluated_at"`
}

// RouteRecommendation is an alias for RoutePlan representing the quota route recommendation
type RouteRecommendation = RoutePlan

// RoutingFraction returns the tightest live quota among weekly and 5h buckets.
// Buckets that classify as neither window are ignored. ok is false when no
// scored bucket is present, so callers can keep the profile quota unknown.
func RoutingFraction(data *QuotaSummaryResponse, now time.Time) (fraction float64, reset time.Time, ok bool) {
	if data == nil {
		return 0, time.Time{}, false
	}

	var best *QuotaBucket
	for _, group := range data.Response.Groups {
		for i := range group.Buckets {
			bucket := group.Buckets[i]
			window := ClassifyWindow(bucket, now)
			if window != WindowWeekly && window != Window5h {
				continue
			}
			if best == nil || bucket.RemainingFraction < best.RemainingFraction {
				copied := bucket
				best = &copied
			}
		}
	}
	if best == nil {
		return 0, time.Time{}, false
	}

	reset, _ = time.Parse(time.RFC3339, best.ResetTime)
	if reset.IsZero() {
		reset, _ = time.Parse(time.RFC3339Nano, best.ResetTime)
	}
	return best.RemainingFraction, reset, true
}

// ModelForWeight returns the canonical model identifier recommended for the given weight
func ModelForWeight(w RouteWeight) string {
	switch w {
	case WeightLight:
		return "gemini-3.8-flash-low"
	case WeightHeavy:
		return "gemini-3.8-flash-high"
	default:
		return "gemini-3.8-flash-medium"
	}
}

// EffortForWeight returns the reasoning effort level recommended for the given weight
func EffortForWeight(w RouteWeight) string {
	switch w {
	case WeightLight:
		return "low"
	case WeightHeavy:
		return "high"
	default:
		return "medium"
	}
}

// fetchActiveServers discovers running language servers and populates their quota summaries.
func fetchActiveServers(targetProf string) ([]ActiveServer, error) {
	live, err := FindActiveServers(targetProf)
	if err != nil {
		return nil, fmt.Errorf("finding active servers: %w", err)
	}
	servers, err := BuildQuotaSummary(targetProf, live)
	if err != nil {
		servers = live
		for i := range servers {
			servers[i].QuotaKnown = true
		}
	}
	return servers, nil
}

// RecommendRoute evaluates active servers and generates a deterministic route recommendation
func RecommendRoute(servers []ActiveServer, weight RouteWeight, now time.Time) RoutePlan {
	normWeight := weight
	if normWeight == "" {
		normWeight = WeightMedium
	}

	plan := RoutePlan{
		Candidates:        []RouteCandidate{},
		Weight:            string(normWeight),
		RecommendedEffort: EffortForWeight(normWeight),
		EvaluatedAt:       now.UTC().Format(time.RFC3339),
	}

	bestByProfile := make(map[string]RouteCandidate)
	model := ModelForWeight(normWeight)

	for _, srv := range servers {
		prof := strings.TrimSpace(srv.Profile)
		if prof == "" || prof == "host" {
			continue
		}

		cand := evaluateServer(srv, normWeight, model, now)
		existing, found := bestByProfile[prof]
		if !found || compareCandidates(cand, existing) < 0 {
			bestByProfile[prof] = cand
		}
	}

	for _, cand := range bestByProfile {
		plan.Candidates = append(plan.Candidates, cand)
	}

	sort.SliceStable(plan.Candidates, func(i, j int) bool {
		return compareCandidates(plan.Candidates[i], plan.Candidates[j]) < 0
	})

	if len(plan.Candidates) > 0 && plan.Candidates[0].Eligible {
		top := plan.Candidates[0]
		plan.Recommended = &top
	}

	return plan
}

// RecommendRouteFromServers evaluates active servers and generates a deterministic route recommendation.
func RecommendRouteFromServers(servers []ActiveServer, weight RouteWeight, now time.Time) RoutePlan {
	return RecommendRoute(servers, weight, now)
}

func evaluateServer(srv ActiveServer, weight RouteWeight, model string, now time.Time) RouteCandidate {
	cand := RouteCandidate{
		Profile:  srv.Profile,
		Model:    model,
		Weight:   string(weight),
		Eligible: false,
	}

	if srv.Data == nil || !srv.QuotaKnown || len(srv.Data.Response.Groups) == 0 {
		cand.Score = -1000
		cand.Reason = "quota unknown / telemetry unavailable"
		return cand
	}

	var geminiWeekly, gemini5h *QuotaBucket
	var fallbackWeekly, fallback5h *QuotaBucket

	for _, group := range srv.Data.Response.Groups {
		isGeminiGroup := strings.Contains(strings.ToLower(group.DisplayName), "gemini")
		for i := range group.Buckets {
			b := group.Buckets[i]
			w := ClassifyWindow(b, now)
			bID := strings.ToLower(b.BucketID)
			isGemini := isGeminiGroup || strings.Contains(bID, "gemini")

			switch w {
			case Window5h:
				if isGemini {
					if gemini5h == nil || b.RemainingFraction < gemini5h.RemainingFraction {
						copied := b
						gemini5h = &copied
					}
				} else {
					if fallback5h == nil || b.RemainingFraction < fallback5h.RemainingFraction {
						copied := b
						fallback5h = &copied
					}
				}
			case WindowWeekly:
				if isGemini {
					if geminiWeekly == nil || b.RemainingFraction < geminiWeekly.RemainingFraction {
						copied := b
						geminiWeekly = &copied
					}
				} else {
					if fallbackWeekly == nil || b.RemainingFraction < fallbackWeekly.RemainingFraction {
						copied := b
						fallbackWeekly = &copied
					}
				}
			}
		}
	}

	effective5h := gemini5h
	if effective5h == nil {
		effective5h = fallback5h
	}
	effectiveWeekly := geminiWeekly
	if effectiveWeekly == nil {
		effectiveWeekly = fallbackWeekly
	}

	switch weight {
	case WeightLight:
		if effective5h != nil {
			cand.RemainingFraction = effective5h.RemainingFraction
			cand.ResetTime = effective5h.ResetTime
			cand.WindowType = Window5h
			if effective5h.RemainingFraction <= CriticalQuotaThreshold {
				if effectiveWeekly != nil && effectiveWeekly.RemainingFraction > CriticalQuotaThreshold {
					cand.RemainingFraction = effectiveWeekly.RemainingFraction
					cand.ResetTime = effectiveWeekly.ResetTime
					cand.WindowType = WindowWeekly
					cand.Eligible = true
					cand.Score = effectiveWeekly.RemainingFraction * 1000.0
					cand.Reason = fmt.Sprintf("5h window depleted (<=1%%); fallback to weekly quota (%.0f%% remaining)", cand.RemainingFraction*100)
				} else {
					cand.Score = -1000
					cand.Reason = fmt.Sprintf("critical quota (%.0f%% remaining, <=1%%)", effective5h.RemainingFraction*100)
				}
			} else if effective5h.RemainingFraction >= 0.50 {
				cand.Eligible = true
				cand.Score = effective5h.RemainingFraction*1000.0 + 100.0
				cand.Reason = fmt.Sprintf("healthy 5h quota window (%.0f%% remaining)", effective5h.RemainingFraction*100)
			} else {
				cand.Eligible = true
				cand.Score = effective5h.RemainingFraction * 1000.0
				cand.Reason = fmt.Sprintf("available 5h quota window (%.0f%% remaining)", effective5h.RemainingFraction*100)
			}
		} else if effectiveWeekly != nil {
			cand.RemainingFraction = effectiveWeekly.RemainingFraction
			cand.ResetTime = effectiveWeekly.ResetTime
			cand.WindowType = WindowWeekly
			if effectiveWeekly.RemainingFraction <= CriticalQuotaThreshold {
				cand.Score = -1000
				cand.Reason = fmt.Sprintf("critical weekly quota (%.0f%% remaining, <=1%%)", effectiveWeekly.RemainingFraction*100)
			} else {
				cand.Eligible = true
				cand.Score = effectiveWeekly.RemainingFraction * 1000.0
				cand.Reason = fmt.Sprintf("available weekly quota (%.0f%% remaining)", effectiveWeekly.RemainingFraction*100)
			}
		} else {
			cand.Score = -1000
			cand.Reason = "no recognized 5h or weekly quota window"
		}

	case WeightHeavy:
		if effectiveWeekly != nil {
			cand.RemainingFraction = effectiveWeekly.RemainingFraction
			cand.ResetTime = effectiveWeekly.ResetTime
			cand.WindowType = WindowWeekly
			if effectiveWeekly.RemainingFraction <= 0.15 {
				cand.Score = -1000
				cand.Reason = fmt.Sprintf("insufficient weekly quota for heavy workload (%.0f%% remaining, threshold >15%%)", effectiveWeekly.RemainingFraction*100)
			} else if effectiveWeekly.RemainingFraction >= 0.35 {
				cand.Eligible = true
				cand.Score = effectiveWeekly.RemainingFraction*1000.0 + 50.0
				cand.Reason = fmt.Sprintf("optimal weekly quota (%.0f%% remaining)", effectiveWeekly.RemainingFraction*100)
			} else {
				cand.Eligible = true
				cand.Score = effectiveWeekly.RemainingFraction * 1000.0
				cand.Reason = fmt.Sprintf("moderate weekly quota (%.0f%% remaining)", effectiveWeekly.RemainingFraction*100)
			}
		} else {
			cand.Score = -1000
			cand.Reason = "no recognized weekly quota window for heavy workload"
		}

	default: // WeightMedium
		tightestFraction, tightestReset, ok := RoutingFraction(srv.Data, now)
		if !ok {
			cand.Score = -1000
			cand.Reason = "no recognized 5h or weekly quota window"
			return cand
		}
		cand.RemainingFraction = tightestFraction
		if !tightestReset.IsZero() {
			cand.ResetTime = tightestReset.Format(time.RFC3339)
		}
		if effective5h != nil && effective5h.RemainingFraction == tightestFraction {
			cand.WindowType = Window5h
		} else {
			cand.WindowType = WindowWeekly
		}

		if tightestFraction <= CriticalQuotaThreshold {
			cand.Score = -1000
			cand.Reason = fmt.Sprintf("critical quota (%.0f%% remaining, <=1%%)", tightestFraction*100)
		} else if tightestFraction < 0.20 {
			cand.Eligible = true
			cand.Score = tightestFraction * 1000.0
			cand.Reason = fmt.Sprintf("low quota budget (%.0f%% remaining)", tightestFraction*100)
		} else {
			cand.Eligible = true
			cand.Score = tightestFraction * 1000.0
			cand.Reason = fmt.Sprintf("healthy quota budget (%.0f%% remaining)", tightestFraction*100)
		}
	}

	return cand
}

// compareCandidates returns < 0 if a should rank before b (better candidate)
func compareCandidates(a, b RouteCandidate) int {
	if a.Eligible != b.Eligible {
		if a.Eligible {
			return -1
		}
		return 1
	}

	if a.Score != b.Score {
		if a.Score > b.Score {
			return -1
		}
		return 1
	}

	if a.RemainingFraction != b.RemainingFraction {
		if a.RemainingFraction > b.RemainingFraction {
			return -1
		}
		return 1
	}

	// Tie-break: sooner reset time ranks higher
	ta, erra := time.Parse(time.RFC3339, a.ResetTime)
	tb, errb := time.Parse(time.RFC3339, b.ResetTime)
	if erra == nil && errb == nil && !ta.Equal(tb) {
		if ta.Before(tb) {
			return -1
		}
		return 1
	}

	// Final deterministic tie-breaker: profile name alphabetical ascending
	return strings.Compare(a.Profile, b.Profile)
}

// SelectEligibleProfilesFromCandidates filters candidates where Eligible is true,
// partitions non-excluded profiles first (preserving candidate order) followed by
// excluded profiles as fallback when fewer than count non-excluded profiles are available,
// and returns up to count profile names (or all if count <= 0).
func SelectEligibleProfilesFromCandidates(candidates []RouteCandidate, count int, excludeProfiles ...string) []string {
	excludedSet := make(map[string]bool, len(excludeProfiles))
	for _, p := range excludeProfiles {
		p = strings.TrimSpace(p)
		if p != "" {
			excludedSet[p] = true
		}
	}

	var nonExcluded []string
	var excluded []string
	seen := make(map[string]bool)

	for _, c := range candidates {
		if !c.Eligible {
			continue
		}
		prof := strings.TrimSpace(c.Profile)
		if prof == "" || seen[prof] {
			continue
		}
		seen[prof] = true

		if excludedSet[prof] {
			excluded = append(excluded, prof)
		} else {
			nonExcluded = append(nonExcluded, prof)
		}
	}

	combined := append(nonExcluded, excluded...)
	if count > 0 && len(combined) > count {
		return combined[:count]
	}
	if combined == nil {
		return []string{}
	}
	return combined
}

// BucketGroupForModel maps model identifiers to their corresponding quota bucket family:
// - gemini-4-argon-* or containing "argon" -> "argon"
// - claude-*, gpt-*, o1-*, o3-*, 3p* -> "3p"
// - gemini-* or empty "" -> "gemini"
func BucketGroupForModel(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return "gemini"
	}
	if strings.Contains(m, "argon") {
		return "argon"
	}
	if strings.HasPrefix(m, "claude-") ||
		strings.HasPrefix(m, "gpt-") ||
		strings.HasPrefix(m, "o1-") ||
		strings.HasPrefix(m, "o3-") ||
		strings.HasPrefix(m, "3p") {
		return "3p"
	}
	return "gemini"
}

// ScoreServerForBucket extracts weekly and 5h quota fractions for target bucketGroup and scores the candidate.
func ScoreServerForBucket(srv ActiveServer, bucketGroup string, weight RouteWeight, now time.Time) RouteCandidate {
	if bucketGroup == "" {
		bucketGroup = "gemini"
	}
	normWeight := NormalizeWeight(weight)
	model := ModelForWeight(normWeight)

	cand := RouteCandidate{
		Profile:  srv.Profile,
		Model:    model,
		Weight:   string(normWeight),
		Eligible: false,
	}

	if srv.Data == nil || !srv.QuotaKnown || len(srv.Data.Response.Groups) == 0 {
		cand.Score = -1000
		cand.Reason = "quota unknown / telemetry unavailable"
		return cand
	}

	// Check if any bucket on the server has RemainingFraction <= CriticalQuotaThreshold.
	for _, group := range srv.Data.Response.Groups {
		for _, b := range group.Buckets {
			if b.RemainingFraction <= CriticalQuotaThreshold {
				cand.Score = -1000
				cand.RemainingFraction = b.RemainingFraction
				cand.ResetTime = b.ResetTime
				cand.Reason = fmt.Sprintf("critical quota on bucket %s (%.0f%% remaining, <=1%%)", b.BucketID, b.RemainingFraction*100)
				return cand
			}
		}
	}

	var targetWeekly, target5h *QuotaBucket
	var hasTargetWeekly bool
	var hasAnyNamedBucket bool

	for _, group := range srv.Data.Response.Groups {
		for i := range group.Buckets {
			b := group.Buckets[i]
			bID := strings.TrimSpace(strings.ToLower(b.BucketID))
			if bID != "" {
				hasAnyNamedBucket = true
			}

			// Match logic:
			// Backward compatibility: when b.BucketID == "", treat unnamed buckets as matching the target bucket.
			matchesTargetWeekly := false
			matchesTarget5h := false

			if bID == "" {
				w := ClassifyWindow(b, now)
				if w == WindowWeekly {
					matchesTargetWeekly = true
				} else if w == Window5h {
					matchesTarget5h = true
				}
			} else {
				switch bucketGroup {
				case "argon":
					if bID == "gemini-argon-wtu-weekly" || strings.Contains(bID, "argon") {
						matchesTargetWeekly = true
					}
					// Argon typically shares 5h with gemini or has gemini-5h
					if bID == "gemini-5h" || (strings.Contains(bID, "5h") && strings.Contains(bID, "argon")) {
						matchesTarget5h = true
					}
				case "3p":
					if bID == "3p-weekly" || strings.HasPrefix(bID, "3p-") && strings.Contains(bID, "weekly") {
						matchesTargetWeekly = true
					}
					if bID == "3p-5h" || strings.HasPrefix(bID, "3p-") && strings.Contains(bID, "5h") {
						matchesTarget5h = true
					}
				default: // "gemini"
					if bID == "gemini-weekly" || (strings.Contains(bID, "gemini") && strings.Contains(bID, "weekly") && !strings.Contains(bID, "argon")) {
						matchesTargetWeekly = true
					}
					if bID == "gemini-5h" || (strings.Contains(bID, "gemini") && strings.Contains(bID, "5h") && !strings.Contains(bID, "argon")) {
						matchesTarget5h = true
					}
				}
			}

			if matchesTargetWeekly {
				hasTargetWeekly = true
				if targetWeekly == nil || b.RemainingFraction < targetWeekly.RemainingFraction {
					copied := b
					targetWeekly = &copied
				}
			}
			if matchesTarget5h {
				if target5h == nil || b.RemainingFraction < target5h.RemainingFraction {
					copied := b
					target5h = &copied
				}
			}
		}
	}

	// Specification rule:
	// When named BucketIDs are present in srv.Data.Response.Groups, do NOT assume default_untouched = 1.0
	// for a missing target weekly bucket when only unrelated bucket families exist
	// (e.g. only 3p-weekly is present in buckets when scoring gemini or argon, or only gemini-weekly is present when scoring 3p)
	if hasAnyNamedBucket && !hasTargetWeekly {
		cand.Score = -1000
		cand.Reason = fmt.Sprintf("no recognized weekly quota window for bucket group %s", bucketGroup)
		return cand
	}

	switch normWeight {
	case WeightLight:
		if target5h != nil {
			cand.RemainingFraction = target5h.RemainingFraction
			cand.ResetTime = target5h.ResetTime
			cand.WindowType = Window5h
			if target5h.RemainingFraction <= CriticalQuotaThreshold {
				if targetWeekly != nil && targetWeekly.RemainingFraction > CriticalQuotaThreshold {
					cand.RemainingFraction = targetWeekly.RemainingFraction
					cand.ResetTime = targetWeekly.ResetTime
					cand.WindowType = WindowWeekly
					cand.Eligible = true
					cand.Score = targetWeekly.RemainingFraction * 1000.0
					cand.Reason = fmt.Sprintf("5h window depleted (<=1%%); fallback to weekly quota (%.0f%% remaining)", cand.RemainingFraction*100)
				} else {
					cand.Score = -1000
					cand.Reason = fmt.Sprintf("critical quota (%.0f%% remaining, <=1%%)", target5h.RemainingFraction*100)
				}
			} else if target5h.RemainingFraction >= 0.50 {
				cand.Eligible = true
				cand.Score = target5h.RemainingFraction*1000.0 + 100.0
				cand.Reason = fmt.Sprintf("healthy 5h quota window (%.0f%% remaining)", target5h.RemainingFraction*100)
			} else {
				cand.Eligible = true
				cand.Score = target5h.RemainingFraction * 1000.0
				cand.Reason = fmt.Sprintf("available 5h quota window (%.0f%% remaining)", target5h.RemainingFraction*100)
			}
		} else if targetWeekly != nil {
			cand.RemainingFraction = targetWeekly.RemainingFraction
			cand.ResetTime = targetWeekly.ResetTime
			cand.WindowType = WindowWeekly
			if targetWeekly.RemainingFraction <= CriticalQuotaThreshold {
				cand.Score = -1000
				cand.Reason = fmt.Sprintf("critical weekly quota (%.0f%% remaining, <=1%%)", targetWeekly.RemainingFraction*100)
			} else {
				cand.Eligible = true
				cand.Score = targetWeekly.RemainingFraction * 1000.0
				cand.Reason = fmt.Sprintf("available weekly quota (%.0f%% remaining)", targetWeekly.RemainingFraction*100)
			}
		} else {
			cand.Score = -1000
			cand.Reason = "no recognized 5h or weekly quota window"
		}

	case WeightHeavy:
		if targetWeekly != nil {
			cand.RemainingFraction = targetWeekly.RemainingFraction
			cand.ResetTime = targetWeekly.ResetTime
			cand.WindowType = WindowWeekly
			if targetWeekly.RemainingFraction <= 0.15 {
				cand.Score = -1000
				cand.Reason = fmt.Sprintf("insufficient weekly quota for heavy workload (%.0f%% remaining, threshold >15%%)", targetWeekly.RemainingFraction*100)
			} else if targetWeekly.RemainingFraction >= 0.35 {
				cand.Eligible = true
				cand.Score = targetWeekly.RemainingFraction*1000.0 + 50.0
				cand.Reason = fmt.Sprintf("optimal weekly quota (%.0f%% remaining)", targetWeekly.RemainingFraction*100)
			} else {
				cand.Eligible = true
				cand.Score = targetWeekly.RemainingFraction * 1000.0
				cand.Reason = fmt.Sprintf("moderate weekly quota (%.0f%% remaining)", targetWeekly.RemainingFraction*100)
			}
		} else {
			cand.Score = -1000
			cand.Reason = "no recognized weekly quota window for heavy workload"
		}

	default: // WeightMedium
		var tightest *QuotaBucket
		if targetWeekly != nil && (tightest == nil || targetWeekly.RemainingFraction < tightest.RemainingFraction) {
			tightest = targetWeekly
		}
		if target5h != nil && (tightest == nil || target5h.RemainingFraction < tightest.RemainingFraction) {
			tightest = target5h
		}

		if tightest == nil {
			cand.Score = -1000
			cand.Reason = "no recognized 5h or weekly quota window"
			return cand
		}

		cand.RemainingFraction = tightest.RemainingFraction
		cand.ResetTime = tightest.ResetTime
		if target5h != nil && target5h.RemainingFraction == tightest.RemainingFraction {
			cand.WindowType = Window5h
		} else {
			cand.WindowType = WindowWeekly
		}

		if tightest.RemainingFraction <= CriticalQuotaThreshold {
			cand.Score = -1000
			cand.Reason = fmt.Sprintf("critical quota (%.0f%% remaining, <=1%%)", tightest.RemainingFraction*100)
		} else if tightest.RemainingFraction < 0.20 {
			cand.Eligible = true
			cand.Score = tightest.RemainingFraction * 1000.0
			cand.Reason = fmt.Sprintf("low quota budget (%.0f%% remaining)", tightest.RemainingFraction*100)
		} else {
			cand.Eligible = true
			cand.Score = tightest.RemainingFraction * 1000.0
			cand.Reason = fmt.Sprintf("healthy quota budget (%.0f%% remaining)", tightest.RemainingFraction*100)
		}
	}

	return cand
}

// SelectEligibleProfiles discovers running language servers, computes route recommendations,
// delegates to SelectEligibleProfilesFromCandidates, and returns up to count eligible profile names.
// It returns an error if the resulting slice is empty or if discovery fails.
func SelectEligibleProfiles(count int, weight RouteWeight, excludeProfiles ...string) ([]string, error) {
	servers, err := fetchActiveServers("")
	if err != nil {
		return nil, fmt.Errorf("finding active servers: %w", err)
	}
	rec := RecommendRoute(servers, weight, time.Now())
	selected := SelectEligibleProfilesFromCandidates(rec.Candidates, count, excludeProfiles...)
	if len(selected) == 0 {
		return nil, fmt.Errorf("no eligible profiles found for workload weight %s", weight)
	}
	return selected, nil
}

// SelectEligibleProfilesForModel discovers running servers, scores each for the requested model's bucket group,
// sorts candidates deterministically, and delegates to SelectEligibleProfilesFromCandidates.
func SelectEligibleProfilesForModel(count int, model string, weight RouteWeight, excludeProfiles ...string) ([]string, error) {
	if onCd, until, reason := IsModelOnCooldown(model); onCd {
		return nil, fmt.Errorf("model %s is under active shared cooldown until %s (reason: %s)", model, until.Format(time.RFC3339), reason)
	}

	servers, err := fetchActiveServers("")
	if err != nil {
		return nil, fmt.Errorf("fetching active servers: %w", err)
	}

	bucketGroup := BucketGroupForModel(model)
	normWeight := NormalizeWeight(weight)
	now := time.Now()

	bestByProfile := make(map[string]RouteCandidate)
	for _, srv := range servers {
		prof := strings.TrimSpace(srv.Profile)
		if prof == "" || prof == "host" {
			continue
		}
		cand := ScoreServerForBucket(srv, bucketGroup, normWeight, now)
		existing, found := bestByProfile[prof]
		if !found || compareCandidates(cand, existing) < 0 {
			bestByProfile[prof] = cand
		}
	}

	candidates := make([]RouteCandidate, 0, len(bestByProfile))
	for _, cand := range bestByProfile {
		if IsProfileOnCooldown(cand.Profile) {
			continue
		}
		candidates = append(candidates, cand)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return compareCandidates(candidates[i], candidates[j]) < 0
	})

	selected := SelectEligibleProfilesFromCandidates(candidates, count, excludeProfiles...)
	if len(selected) == 0 {
		return nil, fmt.Errorf("no eligible profiles found for model %s (bucket %s, weight %s)", model, bucketGroup, normWeight)
	}
	return selected, nil
}

var (
	profileCooldownMu sync.RWMutex
	profileCooldowns  = make(map[string]time.Time)
)

// ParseQuotaResetDuration extracts a reset duration from provider rate-limit error messages
// (e.g., "Resets in 151h56m34s" or "Resets in 45m10s."), defaulting to fallback when absent or invalid.
func ParseQuotaResetDuration(logText string, fallback time.Duration) time.Duration {
	if dur := ParseRetryAfterHeader(logText); dur > 0 {
		return dur
	}
	idx := strings.Index(strings.ToLower(logText), "resets in ")
	if idx != -1 {
		rest := strings.TrimSpace(logText[idx+len("resets in "):])
		var token strings.Builder
		for i := 0; i < len(rest); i++ {
			ch := rest[i]
			if (ch >= '0' && ch <= '9') || ch == 'h' || ch == 'm' || ch == 's' || ch == '.' {
				token.WriteByte(ch)
			} else {
				break
			}
		}
		durStr := strings.TrimRight(token.String(), ".")
		if durStr != "" {
			if d, err := time.ParseDuration(durStr); err == nil && d > 0 {
				return d
			}
		}
	}
	return fallback
}

// RecordProfileCooldown records a cooldown duration for a profile in memory and persists it
// to the profile's quota-history.jsonl so separate CLI invocations respect active 429 cooldowns.
func RecordProfileCooldown(profile string, duration time.Duration) {
	prof := strings.TrimSpace(profile)
	if prof == "" {
		return
	}
	profileCooldownMu.Lock()
	if duration <= 0 {
		delete(profileCooldowns, prof)
		profileCooldownMu.Unlock()
		_ = RecordCooldown("profile:"+prof, 0, "", SourceCooldown)
		_ = appendSample(prof, Sample{
			Timestamp:     historyNow().UTC(),
			Profile:       prof,
			Source:        SourceCooldown,
			CooldownUntil: time.Time{}.UTC().Format(time.RFC3339),
		})
		return
	}
	until := time.Now().UTC().Add(duration)
	profileCooldowns[prof] = until
	profileCooldownMu.Unlock()

	_ = RecordCooldown("profile:"+prof, duration, "profile cooldown", SourceCooldown)
	_ = appendSample(prof, Sample{
		Timestamp:     historyNow().UTC(),
		Profile:       prof,
		Source:        SourceCooldown,
		CooldownUntil: until.Format(time.RFC3339),
	})
}

// IsProfileOnCooldown reports whether the profile is currently under an active cooldown,
// checking in-memory state first and falling back to the latest cooldown sample in quota-history.jsonl.
func IsProfileOnCooldown(profile string) bool {
	prof := strings.TrimSpace(profile)
	if prof == "" {
		return false
	}
	now := time.Now()
	profileCooldownMu.RLock()
	until, ok := profileCooldowns[prof]
	profileCooldownMu.RUnlock()
	if ok {
		return now.Before(until)
	}

	if onCd, _, _ := IsTargetOnCooldown("profile:" + prof); onCd {
		return true
	}

	samples, err := readSamples(prof, HistoryQuery{Source: SourceCooldown, Limit: 5})
	if err == nil && len(samples) > 0 {
		last := samples[len(samples)-1]
		if last.CooldownUntil != "" {
			if parsed, pErr := time.Parse(time.RFC3339, last.CooldownUntil); pErr == nil {
				profileCooldownMu.Lock()
				profileCooldowns[prof] = parsed
				profileCooldownMu.Unlock()
				return now.Before(parsed)
			}
		}
	}
	return false
}

// PreflightProfilesForModel discovers running servers, scores each for the requested model's bucket group,
// sorts candidates deterministically, and delegates to PreflightProfilesForModelFromCandidates.
func PreflightProfilesForModel(count int, model string, weight RouteWeight, minFraction float64, excludeProfiles ...string) ([]RouteCandidate, error) {
	if onCd, until, reason := IsModelOnCooldown(model); onCd {
		return nil, fmt.Errorf("model %s is under active shared cooldown until %s (reason: %s)", model, until.Format(time.RFC3339), reason)
	}

	servers, err := fetchActiveServers("")
	if err != nil {
		return nil, fmt.Errorf("fetching active servers: %w", err)
	}

	bucketGroup := BucketGroupForModel(model)
	normWeight := NormalizeWeight(weight)
	now := time.Now()

	bestByProfile := make(map[string]RouteCandidate)
	for _, srv := range servers {
		prof := strings.TrimSpace(srv.Profile)
		if prof == "" || prof == "host" {
			continue
		}
		cand := ScoreServerForBucket(srv, bucketGroup, normWeight, now)
		existing, found := bestByProfile[prof]
		if !found || compareCandidates(cand, existing) < 0 {
			bestByProfile[prof] = cand
		}
	}

	candidates := make([]RouteCandidate, 0, len(bestByProfile))
	for _, cand := range bestByProfile {
		candidates = append(candidates, cand)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return compareCandidates(candidates[i], candidates[j]) < 0
	})

	return PreflightProfilesForModelFromCandidates(candidates, count, model, minFraction, excludeProfiles...)
}

// PreflightProfilesForModelFromCandidates filters candidates rejecting profiles with RemainingFraction below minFraction
// (defaulting to SafetyThresholdForModel(model, WeightMedium) or DefaultSafetyQuotaThreshold when minFraction <= 0) or currently under active cooldown.
// It partitions non-excluded profiles first, followed by excluded profiles as fallback, returning up to count candidates.
func PreflightProfilesForModelFromCandidates(candidates []RouteCandidate, count int, model string, minFraction float64, excludeProfiles ...string) ([]RouteCandidate, error) {
	if onCd, until, reason := IsModelOnCooldown(model); onCd {
		return nil, fmt.Errorf("model %s is under active shared cooldown until %s (reason: %s)", model, until.Format(time.RFC3339), reason)
	}

	if minFraction <= 0 {
		if strings.TrimSpace(model) == "" {
			minFraction = DefaultSafetyQuotaThreshold
		} else {
			minFraction = SafetyThresholdForModel(model, WeightMedium)
		}
	}

	excludedSet := make(map[string]bool, len(excludeProfiles))
	for _, p := range excludeProfiles {
		p = strings.TrimSpace(p)
		if p != "" {
			excludedSet[p] = true
		}
	}

	var nonExcluded []RouteCandidate
	var excluded []RouteCandidate
	seen := make(map[string]bool)

	for _, c := range candidates {
		if !c.Eligible {
			continue
		}
		prof := strings.TrimSpace(c.Profile)
		if prof == "" || seen[prof] {
			continue
		}
		if c.RemainingFraction < minFraction {
			continue
		}
		if IsProfileOnCooldown(prof) {
			continue
		}
		seen[prof] = true

		if excludedSet[prof] {
			excluded = append(excluded, c)
		} else {
			nonExcluded = append(nonExcluded, c)
		}
	}

	combined := append(nonExcluded, excluded...)
	if count > 0 && len(combined) > count {
		combined = combined[:count]
	}
	if len(combined) == 0 {
		return nil, fmt.Errorf("no eligible profiles meet preflight threshold %.2f for model %s", minFraction, model)
	}
	return combined, nil
}

var (
	probeServerFn func(profileName string) (*ActiveServer, error)
	probeMu       sync.Mutex
)

// SetProbeTestHook overrides the server probing function for hermetic unit testing.
func SetProbeTestHook(fn func(profileName string) (*ActiveServer, error)) func() {
	probeMu.Lock()
	defer probeMu.Unlock()
	old := probeServerFn
	probeServerFn = fn
	return func() {
		probeMu.Lock()
		defer probeMu.Unlock()
		probeServerFn = old
	}
}

// ProbeInactiveProfiles discovers which target profiles (or all existing profiles if profiles is empty)
// lack an active language server, attempts to start/probe their language server to refresh quota metrics,
// and returns the names of successfully awakened profiles.
func ProbeInactiveProfiles(profiles []string) ([]string, error) {
	probeMu.Lock()
	pFn := probeServerFn
	probeMu.Unlock()

	var targets []string
	if len(profiles) > 0 {
		targets = profiles
	} else {
		profs, err := profile.ListProfiles()
		if err != nil {
			return nil, fmt.Errorf("listing profiles for probing: %w", err)
		}
		targets = profs
	}

	live, err := FindActiveServers("")
	activeSet := make(map[string]bool)
	if err == nil {
		for _, s := range live {
			if s.QuotaKnown && s.Profile != "" {
				activeSet[s.Profile] = true
			}
		}
	}

	var probed []string
	for _, p := range targets {
		if activeSet[p] {
			continue
		}

		if pFn != nil {
			srv, pErr := pFn(p)
			if pErr == nil && srv != nil {
				probed = append(probed, p)
			}
			continue
		}

		// Default on-demand probing: start headless server and retrieve quota
		inst, startErr := StartHeadlessServer(p)
		if startErr != nil {
			continue
		}

		statePath := filepath.Join(config.GetProfileDir(p), ".multigravity", "headless.json")
		if err := os.MkdirAll(filepath.Dir(statePath), 0755); err == nil {
			data, _ := json.Marshal(map[string]interface{}{
				"pid":        inst.PID,
				"port":       inst.Port,
				"csrf_token": inst.CSRF,
				"profile":    p,
				"started_at": time.Now().UTC().Format(time.RFC3339),
				"status":     "running",
			})
			_ = os.WriteFile(statePath, data, 0644)
		}

		client := NewClient(2 * time.Second)
		if res, qErr := client.RetrieveUserQuotaSummary(inst.Port, inst.CSRF); qErr == nil && res != nil {
			probed = append(probed, p)
		}
	}

	return probed, nil
}
