package quota

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// RouteWeight represents the task complexity level for quota routing
type RouteWeight string

const (
	WeightLight  RouteWeight = "light"
	WeightMedium RouteWeight = "medium"
	WeightHeavy  RouteWeight = "heavy"
)

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
	Recommended *RouteCandidate  `json:"recommended"`
	Candidates  []RouteCandidate `json:"candidates"`
	Weight      string           `json:"weight"`
	EvaluatedAt string           `json:"evaluated_at"`
}

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
		return "gemini-2.5-flash"
	case WeightHeavy:
		return "gemini-2.5-pro"
	default:
		return "gemini-3.8-flash-tiered"
	}
}

// RecommendRoute evaluates active servers and generates a deterministic route recommendation
func RecommendRoute(servers []ActiveServer, weight RouteWeight, now time.Time) RoutePlan {
	if weight == "" {
		weight = WeightMedium
	}

	plan := RoutePlan{
		Candidates:  []RouteCandidate{},
		Weight:      string(weight),
		EvaluatedAt: now.UTC().Format(time.RFC3339),
	}

	bestByProfile := make(map[string]RouteCandidate)
	model := ModelForWeight(weight)

	for _, srv := range servers {
		prof := strings.TrimSpace(srv.Profile)
		if prof == "" || prof == "host" {
			continue
		}

		cand := evaluateServer(srv, weight, model, now)
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
			if effective5h.RemainingFraction <= MinRemainingFraction {
				if effectiveWeekly != nil && effectiveWeekly.RemainingFraction > MinRemainingFraction {
					cand.RemainingFraction = effectiveWeekly.RemainingFraction
					cand.ResetTime = effectiveWeekly.ResetTime
					cand.WindowType = WindowWeekly
					cand.Eligible = true
					cand.Score = effectiveWeekly.RemainingFraction * 1000.0
					cand.Reason = fmt.Sprintf("5h window depleted (<=5%%); fallback to weekly quota (%.0f%% remaining)", cand.RemainingFraction*100)
				} else {
					cand.Score = -1000
					cand.Reason = fmt.Sprintf("critical quota (%.0f%% remaining, <=5%%)", effective5h.RemainingFraction*100)
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
			if effectiveWeekly.RemainingFraction <= MinRemainingFraction {
				cand.Score = -1000
				cand.Reason = fmt.Sprintf("critical weekly quota (%.0f%% remaining, <=5%%)", effectiveWeekly.RemainingFraction*100)
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

		if tightestFraction <= MinRemainingFraction {
			cand.Score = -1000
			cand.Reason = fmt.Sprintf("critical quota (%.0f%% remaining, <=5%%)", tightestFraction*100)
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
