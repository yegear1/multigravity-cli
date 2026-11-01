package quota

import "testing"
import "time"

func TestRoutingFractionUsesTightestWindow(t *testing.T) {
	now := time.Date(2026, 9, 27, 3, 0, 0, 0, time.UTC)
	data := &QuotaSummaryResponse{
		Response: QuotaResponse{
			Groups: []QuotaGroup{{
				Buckets: []QuotaBucket{
					{BucketID: "gemini-weekly", RemainingFraction: 0.40, ResetTime: now.Add(48 * time.Hour).Format(time.RFC3339)},
					{BucketID: "gemini-5h", RemainingFraction: 0.15, ResetTime: now.Add(2 * time.Hour).Format(time.RFC3339)},
					{BucketID: "other", RemainingFraction: 0.01, ResetTime: ""},
				},
			}},
		},
	}

	fraction, reset, ok := RoutingFraction(data, now)
	if !ok || fraction != 0.15 || reset.IsZero() {
		t.Fatalf("fraction=%v reset=%v ok=%v", fraction, reset, ok)
	}

	if _, _, ok := RoutingFraction(&QuotaSummaryResponse{}, now); ok {
		t.Fatal("empty summary reported a quota")
	}
}

func TestParseRouteWeight(t *testing.T) {
	tests := []struct {
		input    string
		expected RouteWeight
		wantErr  bool
	}{
		{"light", WeightLight, false},
		{"LIGHT", WeightLight, false},
		{"medium", WeightMedium, false},
		{"", WeightMedium, false},
		{"heavy", WeightHeavy, false},
		{"invalid", "", true},
		{"extreme", "", true},
	}

	for _, tt := range tests {
		got, err := ParseRouteWeight(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseRouteWeight(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.expected {
			t.Errorf("ParseRouteWeight(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestRecommendRoute_WeightsAndModels(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	serverWork := ActiveServer{
		Profile:    "work",
		QuotaKnown: true,
		Data: &QuotaSummaryResponse{
			Response: QuotaResponse{
				Groups: []QuotaGroup{{
					DisplayName: "Gemini Models",
					Buckets: []QuotaBucket{
						{BucketID: "gemini-weekly", RemainingFraction: 0.80, ResetTime: now.Add(48 * time.Hour).Format(time.RFC3339)},
						{BucketID: "gemini-5h", RemainingFraction: 0.90, ResetTime: now.Add(3 * time.Hour).Format(time.RFC3339)},
					},
				}},
			},
		},
	}

	// Light workload
	planLight := RecommendRoute([]ActiveServer{serverWork}, WeightLight, now)
	if planLight.Recommended == nil {
		t.Fatal("expected recommendation for light, got nil")
	}
	if planLight.Recommended.Model != "gemini-3.8-flash-low" {
		t.Fatalf("expected light model gemini-3.8-flash-low, got %s", planLight.Recommended.Model)
	}
	if planLight.RecommendedEffort != "low" {
		t.Fatalf("expected light effort low, got %s", planLight.RecommendedEffort)
	}
	if planLight.Recommended.WindowType != Window5h {
		t.Fatalf("expected light window to be 5h, got %s", planLight.Recommended.WindowType)
	}

	// Medium workload
	planMedium := RecommendRoute([]ActiveServer{serverWork}, WeightMedium, now)
	if planMedium.Recommended == nil {
		t.Fatal("expected recommendation for medium, got nil")
	}
	if planMedium.Recommended.Model != "gemini-3.8-flash-medium" {
		t.Fatalf("expected medium model gemini-3.8-flash-medium, got %s", planMedium.Recommended.Model)
	}
	if planMedium.RecommendedEffort != "medium" {
		t.Fatalf("expected medium effort medium, got %s", planMedium.RecommendedEffort)
	}

	// Default workload (empty string) defaults to medium
	planDefault := RecommendRoute([]ActiveServer{serverWork}, "", now)
	if planDefault.Recommended == nil {
		t.Fatal("expected recommendation for default, got nil")
	}
	if planDefault.Recommended.Model != "gemini-3.8-flash-medium" {
		t.Fatalf("expected default model gemini-3.8-flash-medium, got %s", planDefault.Recommended.Model)
	}
	if planDefault.RecommendedEffort != "medium" {
		t.Fatalf("expected default effort medium, got %s", planDefault.RecommendedEffort)
	}

	// Heavy workload
	planHeavy := RecommendRoute([]ActiveServer{serverWork}, WeightHeavy, now)
	if planHeavy.Recommended == nil {
		t.Fatal("expected recommendation for heavy, got nil")
	}
	if planHeavy.Recommended.Model != "gemini-3.8-flash-high" {
		t.Fatalf("expected heavy model gemini-3.8-flash-high, got %s", planHeavy.Recommended.Model)
	}
	if planHeavy.RecommendedEffort != "high" {
		t.Fatalf("expected heavy effort high, got %s", planHeavy.RecommendedEffort)
	}
	if planHeavy.Recommended.WindowType != WindowWeekly {
		t.Fatalf("expected heavy window to be weekly, got %s", planHeavy.Recommended.WindowType)
	}
}

func TestRecommendRoute_DeterministicTieBreaking(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	// Two profiles with identical quota and reset times
	srvBeta := ActiveServer{
		Profile:    "beta-prof",
		QuotaKnown: true,
		Data: &QuotaSummaryResponse{
			Response: QuotaResponse{
				Groups: []QuotaGroup{{
					DisplayName: "Gemini",
					Buckets: []QuotaBucket{
						{BucketID: "gemini-weekly", RemainingFraction: 0.60, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
					},
				}},
			},
		},
	}
	srvAlpha := ActiveServer{
		Profile:    "alpha-prof",
		QuotaKnown: true,
		Data: &QuotaSummaryResponse{
			Response: QuotaResponse{
				Groups: []QuotaGroup{{
					DisplayName: "Gemini",
					Buckets: []QuotaBucket{
						{BucketID: "gemini-weekly", RemainingFraction: 0.60, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
					},
				}},
			},
		},
	}

	plan := RecommendRoute([]ActiveServer{srvBeta, srvAlpha}, WeightMedium, now)
	if plan.Recommended == nil {
		t.Fatal("expected recommendation, got nil")
	}
	// Alphabetical tie-breaker: alpha-prof comes before beta-prof
	if plan.Recommended.Profile != "alpha-prof" {
		t.Fatalf("expected alpha-prof to win alphabetical tie-breaker, got %s", plan.Recommended.Profile)
	}
	if len(plan.Candidates) != 2 || plan.Candidates[0].Profile != "alpha-prof" || plan.Candidates[1].Profile != "beta-prof" {
		t.Fatalf("candidates order unexpected: %+v", plan.Candidates)
	}
}

func TestRecommendRoute_CriticalQuotaAndEligibility(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	// Depleted profile
	srvDepleted := ActiveServer{
		Profile:    "depleted",
		QuotaKnown: true,
		Data: &QuotaSummaryResponse{
			Response: QuotaResponse{
				Groups: []QuotaGroup{{
					DisplayName: "Gemini",
					Buckets: []QuotaBucket{
						{BucketID: "gemini-weekly", RemainingFraction: 0.03, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
					},
				}},
			},
		},
	}

	plan := RecommendRoute([]ActiveServer{srvDepleted}, WeightMedium, now)
	if plan.Recommended != nil {
		t.Fatalf("expected nil recommendation for depleted profile, got %+v", plan.Recommended)
	}
	if len(plan.Candidates) != 1 || plan.Candidates[0].Eligible {
		t.Fatalf("expected 1 ineligible candidate, got %+v", plan.Candidates)
	}

	// Heavy workload requires >15% weekly quota
	srvLowWeekly := ActiveServer{
		Profile:    "low-weekly",
		QuotaKnown: true,
		Data: &QuotaSummaryResponse{
			Response: QuotaResponse{
				Groups: []QuotaGroup{{
					DisplayName: "Gemini",
					Buckets: []QuotaBucket{
						{BucketID: "gemini-weekly", RemainingFraction: 0.12, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
					},
				}},
			},
		},
	}
	planHeavy := RecommendRoute([]ActiveServer{srvLowWeekly}, WeightHeavy, now)
	if planHeavy.Recommended != nil {
		t.Fatalf("expected low-weekly to be ineligible for heavy workload, got %+v", planHeavy.Recommended)
	}

	// Empty and unknown servers
	planEmpty := RecommendRoute([]ActiveServer{}, WeightMedium, now)
	if planEmpty.Recommended != nil || len(planEmpty.Candidates) != 0 {
		t.Fatalf("expected empty plan for zero servers, got %+v", planEmpty)
	}
	if planEmpty.RecommendedEffort != "medium" {
		t.Fatalf("expected medium effort for empty servers, got %s", planEmpty.RecommendedEffort)
	}
}

func TestModelAndEffortForWeight(t *testing.T) {
	tests := []struct {
		weight       RouteWeight
		wantedModel  string
		wantedEffort string
	}{
		{WeightLight, "gemini-3.8-flash-low", "low"},
		{WeightMedium, "gemini-3.8-flash-medium", "medium"},
		{WeightHeavy, "gemini-3.8-flash-high", "high"},
		{"", "gemini-3.8-flash-medium", "medium"},
		{"unknown", "gemini-3.8-flash-medium", "medium"},
	}

	for _, tt := range tests {
		gotModel := ModelForWeight(tt.weight)
		if gotModel != tt.wantedModel {
			t.Errorf("ModelForWeight(%q) = %s, want %s", tt.weight, gotModel, tt.wantedModel)
		}
		gotEffort := EffortForWeight(tt.weight)
		if gotEffort != tt.wantedEffort {
			t.Errorf("EffortForWeight(%q) = %s, want %s", tt.weight, gotEffort, tt.wantedEffort)
		}
	}
}

func TestSelectEligibleProfilesFromCandidates(t *testing.T) {
	t.Run("secondary-account priority", func(t *testing.T) {
		candidates := []RouteCandidate{
			{Profile: "primary-account", Eligible: true, Score: 950},
			{Profile: "secondary-account", Eligible: true, Score: 850},
			{Profile: "tertiary-account", Eligible: true, Score: 750},
		}

		// When primary-account is excluded, secondary-account and tertiary-account must be prioritized first
		selected := SelectEligibleProfilesFromCandidates(candidates, 1, "primary-account")
		if len(selected) != 1 || selected[0] != "secondary-account" {
			t.Fatalf("expected [secondary-account], got %v", selected)
		}

		selectedTwo := SelectEligibleProfilesFromCandidates(candidates, 2, "primary-account")
		if len(selectedTwo) != 2 || selectedTwo[0] != "secondary-account" || selectedTwo[1] != "tertiary-account" {
			t.Fatalf("expected [secondary-account, tertiary-account], got %v", selectedTwo)
		}
	})

	t.Run("fallback to excluded profile", func(t *testing.T) {
		candidates := []RouteCandidate{
			{Profile: "primary-account", Eligible: true, Score: 950},
			{Profile: "secondary-account", Eligible: true, Score: 850},
		}

		// Requesting 2 profiles while primary-account is excluded:
		// Only 1 non-excluded profile (secondary-account) exists, so primary-account is used as fallback
		selected := SelectEligibleProfilesFromCandidates(candidates, 2, "primary-account")
		if len(selected) != 2 {
			t.Fatalf("expected 2 profiles, got %v", selected)
		}
		if selected[0] != "secondary-account" || selected[1] != "primary-account" {
			t.Fatalf("expected [secondary-account, primary-account], got %v", selected)
		}

		// Requesting 1 profile when all eligible candidates are excluded
		selectedFallback := SelectEligibleProfilesFromCandidates(candidates, 1, "primary-account", "secondary-account")
		if len(selectedFallback) != 1 || selectedFallback[0] != "primary-account" {
			t.Fatalf("expected [primary-account] as fallback, got %v", selectedFallback)
		}
	})

	t.Run("<=5% floor enforcement", func(t *testing.T) {
		candidates := []RouteCandidate{
			{Profile: "depleted-floor", Eligible: false, RemainingFraction: 0.04, Score: -1000},
			{Profile: "exact-floor", Eligible: false, RemainingFraction: 0.05, Score: -1000},
			{Profile: "healthy-account", Eligible: true, RemainingFraction: 0.50, Score: 600},
		}

		// Even when requesting count=5, ineligible profiles (<=5% floor) must never be returned
		selected := SelectEligibleProfilesFromCandidates(candidates, 5)
		if len(selected) != 1 || selected[0] != "healthy-account" {
			t.Fatalf("expected [healthy-account], got %v", selected)
		}

		// When all candidates are <= 5% (Eligible == false), result must be empty
		allDepleted := []RouteCandidate{
			{Profile: "depleted-1", Eligible: false, RemainingFraction: 0.02},
			{Profile: "depleted-2", Eligible: false, RemainingFraction: 0.05},
		}
		selectedEmpty := SelectEligibleProfilesFromCandidates(allDepleted, 2)
		if len(selectedEmpty) != 0 {
			t.Fatalf("expected empty slice, got %v", selectedEmpty)
		}

		// End-to-end evaluation check with RecommendRoute confirming <=5% floor marks Eligible=false
		now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
		depletedServer := ActiveServer{
			Profile:    "critical-prof",
			QuotaKnown: true,
			Data: &QuotaSummaryResponse{
				Response: QuotaResponse{
					Groups: []QuotaGroup{{
						DisplayName: "Gemini",
						Buckets: []QuotaBucket{
							{BucketID: "gemini-weekly", RemainingFraction: 0.05, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
						},
					}},
				},
			},
		}
		healthyServer := ActiveServer{
			Profile:    "healthy-prof",
			QuotaKnown: true,
			Data: &QuotaSummaryResponse{
				Response: QuotaResponse{
					Groups: []QuotaGroup{{
						DisplayName: "Gemini",
						Buckets: []QuotaBucket{
							{BucketID: "gemini-weekly", RemainingFraction: 0.80, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
						},
					}},
				},
			},
		}
		plan := RecommendRoute([]ActiveServer{depletedServer, healthyServer}, WeightMedium, now)
		routeSelected := SelectEligibleProfilesFromCandidates(plan.Candidates, 5)
		if len(routeSelected) != 1 || routeSelected[0] != "healthy-prof" {
			t.Fatalf("expected only healthy-prof after RecommendRoute floor enforcement, got %v", routeSelected)
		}
	})

	t.Run("count <= 0 returns all partitioned", func(t *testing.T) {
		candidates := []RouteCandidate{
			{Profile: "p1", Eligible: true},
			{Profile: "p2", Eligible: true},
			{Profile: "p3", Eligible: true},
		}
		selected := SelectEligibleProfilesFromCandidates(candidates, 0, "p1")
		if len(selected) != 3 || selected[0] != "p2" || selected[1] != "p3" || selected[2] != "p1" {
			t.Fatalf("expected [p2, p3, p1], got %v", selected)
		}
	})
}

func TestSelectEligibleProfiles(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	cleanup := SetTestHooks(func(targetProf string) ([]ActiveServer, error) {
		return []ActiveServer{
			{
				Profile:    "active-prof-1",
				QuotaKnown: true,
				Data: &QuotaSummaryResponse{
					Response: QuotaResponse{
						Groups: []QuotaGroup{{
							DisplayName: "Gemini",
							Buckets: []QuotaBucket{
								{BucketID: "gemini-weekly", RemainingFraction: 0.70, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
							},
						}},
					},
				},
			},
			{
				Profile:    "active-prof-2",
				QuotaKnown: true,
				Data: &QuotaSummaryResponse{
					Response: QuotaResponse{
						Groups: []QuotaGroup{{
							DisplayName: "Gemini",
							Buckets: []QuotaBucket{
								{BucketID: "gemini-weekly", RemainingFraction: 0.03, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
							},
						}},
					},
				},
			},
		}, nil
	})
	defer cleanup()

	selected, err := SelectEligibleProfiles(2, WeightMedium)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selected) != 1 || selected[0] != "active-prof-1" {
		t.Fatalf("expected [active-prof-1], got %v", selected)
	}

	selectedFallback, err := SelectEligibleProfiles(1, WeightMedium, "active-prof-1")
	if err != nil {
		t.Fatalf("unexpected error with fallback: %v", err)
	}
	if len(selectedFallback) != 1 || selectedFallback[0] != "active-prof-1" {
		t.Fatalf("expected fallback to [active-prof-1], got %v", selectedFallback)
	}
}

func TestSelectEligibleProfiles_NoEligibleReturnsError(t *testing.T) {
	cleanup := SetTestHooks(func(targetProf string) ([]ActiveServer, error) {
		return []ActiveServer{}, nil
	})
	defer cleanup()

	_, err := SelectEligibleProfiles(1, WeightMedium)
	if err == nil {
		t.Fatal("expected error when no eligible profiles available, got nil")
	}
}

func TestBucketGroupForModel(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"gemini-4-argon-low", "argon"},
		{"gemini-argon-pro", "argon"},
		{"custom-argon-model", "argon"},
		{"claude-3-7-sonnet", "3p"},
		{"gpt-4o", "3p"},
		{"o1-preview", "3p"},
		{"o3-mini", "3p"},
		{"3p-model-x", "3p"},
		{"3p", "3p"},
		{"gemini-2.5-pro", "gemini"},
		{"gemini-3.8-flash", "gemini"},
		{"", "gemini"},
		{"other-unknown", "gemini"},
	}

	for _, tt := range tests {
		got := BucketGroupForModel(tt.model)
		if got != tt.expected {
			t.Errorf("BucketGroupForModel(%q) = %q, want %q", tt.model, got, tt.expected)
		}
	}
}

func TestScoreServerForBucket(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	t.Run("backward compatibility with unnamed BucketID", func(t *testing.T) {
		srv := ActiveServer{
			Profile:    "unnamed-prof",
			QuotaKnown: true,
			Data: &QuotaSummaryResponse{
				Response: QuotaResponse{
					Groups: []QuotaGroup{{
						DisplayName: "General",
						Buckets: []QuotaBucket{
							{BucketID: "", RemainingFraction: 0.80, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
						},
					}},
				},
			},
		}

		// When BucketID == "", it should match the target bucket regardless of group
		candGemini := ScoreServerForBucket(srv, "gemini", WeightMedium, now)
		if !candGemini.Eligible || candGemini.RemainingFraction != 0.80 {
			t.Fatalf("expected eligible candidate with 0.80 remaining, got %+v", candGemini)
		}

		candArgon := ScoreServerForBucket(srv, "argon", WeightMedium, now)
		if !candArgon.Eligible || candArgon.RemainingFraction != 0.80 {
			t.Fatalf("expected eligible candidate for argon with 0.80 remaining, got %+v", candArgon)
		}
	})

	t.Run("missing target weekly bucket when only unrelated named buckets exist", func(t *testing.T) {
		srv3POnly := ActiveServer{
			Profile:    "3p-only-prof",
			QuotaKnown: true,
			Data: &QuotaSummaryResponse{
				Response: QuotaResponse{
					Groups: []QuotaGroup{{
						DisplayName: "3P Models",
						Buckets: []QuotaBucket{
							{BucketID: "3p-weekly", RemainingFraction: 0.90, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
						},
					}},
				},
			},
		}

		// Scoring gemini or argon when only 3p-weekly exists should NOT assume default 1.0 or be eligible
		candGemini := ScoreServerForBucket(srv3POnly, "gemini", WeightMedium, now)
		if candGemini.Eligible || candGemini.Score != -1000 {
			t.Fatalf("expected ineligible candidate for gemini when only 3p-weekly exists, got %+v", candGemini)
		}

		candArgon := ScoreServerForBucket(srv3POnly, "argon", WeightMedium, now)
		if candArgon.Eligible || candArgon.Score != -1000 {
			t.Fatalf("expected ineligible candidate for argon when only 3p-weekly exists, got %+v", candArgon)
		}

		// But scoring 3p on this server should be eligible
		cand3P := ScoreServerForBucket(srv3POnly, "3p", WeightMedium, now)
		if !cand3P.Eligible || cand3P.RemainingFraction != 0.90 {
			t.Fatalf("expected eligible candidate for 3p, got %+v", cand3P)
		}

		// Conversely, server with only gemini-weekly scored for 3p
		srvGeminiOnly := ActiveServer{
			Profile:    "gemini-only-prof",
			QuotaKnown: true,
			Data: &QuotaSummaryResponse{
				Response: QuotaResponse{
					Groups: []QuotaGroup{{
						DisplayName: "Gemini Models",
						Buckets: []QuotaBucket{
							{BucketID: "gemini-weekly", RemainingFraction: 0.90, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
						},
					}},
				},
			},
		}
		cand3PFromGemini := ScoreServerForBucket(srvGeminiOnly, "3p", WeightMedium, now)
		if cand3PFromGemini.Eligible || cand3PFromGemini.Score != -1000 {
			t.Fatalf("expected ineligible candidate for 3p when only gemini-weekly exists, got %+v", cand3PFromGemini)
		}
	})

	t.Run("any bucket on server has critical quota", func(t *testing.T) {
		srvWithCritical := ActiveServer{
			Profile:    "partially-depleted",
			QuotaKnown: true,
			Data: &QuotaSummaryResponse{
				Response: QuotaResponse{
					Groups: []QuotaGroup{{
						DisplayName: "Models",
						Buckets: []QuotaBucket{
							{BucketID: "gemini-weekly", RemainingFraction: 0.90, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
							{BucketID: "3p-weekly", RemainingFraction: 0.03, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
						},
					}},
				},
			},
		}

		cand := ScoreServerForBucket(srvWithCritical, "gemini", WeightMedium, now)
		if cand.Eligible || cand.Score != -1000 {
			t.Fatalf("expected ineligible candidate because server has bucket with <= 5%% quota, got %+v", cand)
		}
	})
}

func TestSelectEligibleProfilesForModel(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	cleanup := SetTestHooks(func(targetProf string) ([]ActiveServer, error) {
		return []ActiveServer{
			{
				Profile:    "gemini-rich",
				QuotaKnown: true,
				Data: &QuotaSummaryResponse{
					Response: QuotaResponse{
						Groups: []QuotaGroup{{
							DisplayName: "Gemini",
							Buckets: []QuotaBucket{
								{BucketID: "gemini-weekly", RemainingFraction: 0.85, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
								{BucketID: "gemini-5h", RemainingFraction: 0.90, ResetTime: now.Add(3 * time.Hour).Format(time.RFC3339)},
							},
						}},
					},
				},
			},
			{
				Profile:    "3p-rich",
				QuotaKnown: true,
				Data: &QuotaSummaryResponse{
					Response: QuotaResponse{
						Groups: []QuotaGroup{{
							DisplayName: "3P",
							Buckets: []QuotaBucket{
								{BucketID: "3p-weekly", RemainingFraction: 0.80, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
								{BucketID: "3p-5h", RemainingFraction: 0.85, ResetTime: now.Add(3 * time.Hour).Format(time.RFC3339)},
							},
						}},
					},
				},
			},
		}, nil
	})
	defer cleanup()

	// Select for Claude model should select 3p-rich
	selected3P, err := SelectEligibleProfilesForModel(1, "claude-3-7-sonnet", WeightMedium)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selected3P) != 1 || selected3P[0] != "3p-rich" {
		t.Fatalf("expected [3p-rich], got %v", selected3P)
	}

	// Select for Gemini model should select gemini-rich
	selectedGemini, err := SelectEligibleProfilesForModel(1, "gemini-2.5-pro", WeightMedium)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selectedGemini) != 1 || selectedGemini[0] != "gemini-rich" {
		t.Fatalf("expected [gemini-rich], got %v", selectedGemini)
	}

	// Requesting unavailable bucket family returns error
	_, err = SelectEligibleProfilesForModel(1, "gemini-4-argon-flash", WeightMedium)
	if err == nil {
		t.Fatal("expected error when no server has argon bucket, got nil")
	}
}

func TestProfileCooldown(t *testing.T) {
	prof := "test-cooldown-profile"
	t.Cleanup(func() {
		RecordProfileCooldown(prof, 0)
	})

	if IsProfileOnCooldown(prof) {
		t.Fatalf("expected profile %q not to be on cooldown initially", prof)
	}

	RecordProfileCooldown(prof, 10*time.Minute)
	if !IsProfileOnCooldown(prof) {
		t.Fatalf("expected profile %q to be on active cooldown", prof)
	}
	if !IsProfileOnCooldown("  " + prof + "  ") {
		t.Fatalf("expected trimmed profile %q to be on active cooldown", prof)
	}

	// Clearing cooldown with 0 duration
	RecordProfileCooldown(prof, 0)
	if IsProfileOnCooldown(prof) {
		t.Fatalf("expected cooldown for %q to be cleared", prof)
	}

	// Negative duration should not be on cooldown
	RecordProfileCooldown(prof, -1*time.Minute)
	if IsProfileOnCooldown(prof) {
		t.Fatalf("expected negative cooldown for %q to return false", prof)
	}
}

func TestPreflightProfilesForModelFromCandidates(t *testing.T) {
	p1 := "preflight-p1"
	p2 := "preflight-p2"
	p3 := "preflight-p3"
	t.Cleanup(func() {
		RecordProfileCooldown(p1, 0)
		RecordProfileCooldown(p2, 0)
		RecordProfileCooldown(p3, 0)
	})

	t.Run("default safety threshold (0.20) rejects low quota", func(t *testing.T) {
		candidates := []RouteCandidate{
			{Profile: p1, RemainingFraction: 0.15, Eligible: true, Score: 150},
			{Profile: p2, RemainingFraction: 0.40, Eligible: true, Score: 400},
		}

		selected, err := PreflightProfilesForModelFromCandidates(candidates, 2, "gemini-2.5-pro", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(selected) != 1 || selected[0].Profile != p2 {
			t.Fatalf("expected only %s above default 0.20 threshold, got %+v", p2, selected)
		}
	})

	t.Run("rejects active 429 cooldown", func(t *testing.T) {
		candidates := []RouteCandidate{
			{Profile: p1, RemainingFraction: 0.50, Eligible: true, Score: 500},
			{Profile: p2, RemainingFraction: 0.40, Eligible: true, Score: 400},
		}

		RecordProfileCooldown(p1, 5*time.Minute)
		defer RecordProfileCooldown(p1, 0)

		selected, err := PreflightProfilesForModelFromCandidates(candidates, 2, "gemini-2.5-pro", 0.20)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(selected) != 1 || selected[0].Profile != p2 {
			t.Fatalf("expected %s on cooldown to be skipped, got %+v", p1, selected)
		}
	})

	t.Run("custom minFraction threshold", func(t *testing.T) {
		candidates := []RouteCandidate{
			{Profile: p1, RemainingFraction: 0.25, Eligible: true, Score: 250},
			{Profile: p2, RemainingFraction: 0.45, Eligible: true, Score: 450},
		}

		// minFraction 0.30 should reject p1
		selected, err := PreflightProfilesForModelFromCandidates(candidates, 2, "gemini-2.5-pro", 0.30)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(selected) != 1 || selected[0].Profile != p2 {
			t.Fatalf("expected only %s above 0.30, got %+v", p2, selected)
		}

		// minFraction 0.50 should reject both and return error
		_, err = PreflightProfilesForModelFromCandidates(candidates, 2, "gemini-2.5-pro", 0.50)
		if err == nil {
			t.Fatal("expected error when no candidates meet minFraction 0.50, got nil")
		}
	})

	t.Run("fallback to excluded profiles", func(t *testing.T) {
		candidates := []RouteCandidate{
			{Profile: p1, RemainingFraction: 0.10, Eligible: true, Score: 100}, // below threshold
			{Profile: p2, RemainingFraction: 0.60, Eligible: true, Score: 600}, // excluded but healthy
		}

		// p1 is rejected due to threshold; p2 is excluded but serves as fallback
		selected, err := PreflightProfilesForModelFromCandidates(candidates, 1, "gemini-2.5-pro", 0.20, p2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(selected) != 1 || selected[0].Profile != p2 {
			t.Fatalf("expected fallback to %s, got %+v", p2, selected)
		}
	})

	t.Run("ineligible candidates are rejected", func(t *testing.T) {
		candidates := []RouteCandidate{
			{Profile: p1, RemainingFraction: 0.80, Eligible: false, Score: -1000},
		}

		_, err := PreflightProfilesForModelFromCandidates(candidates, 1, "gemini-2.5-pro", 0.20)
		if err == nil {
			t.Fatal("expected error when candidate is ineligible, got nil")
		}
	})
}

func TestPreflightProfilesForModel(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	profHealthy := "preflight-srv-healthy"
	profLow := "preflight-srv-low"
	profCooldown := "preflight-srv-cooldown"

	t.Cleanup(func() {
		RecordProfileCooldown(profCooldown, 0)
	})

	cleanup := SetTestHooks(func(targetProf string) ([]ActiveServer, error) {
		return []ActiveServer{
			{
				Profile:    profHealthy,
				QuotaKnown: true,
				Data: &QuotaSummaryResponse{
					Response: QuotaResponse{
						Groups: []QuotaGroup{{
							DisplayName: "Gemini",
							Buckets: []QuotaBucket{
								{BucketID: "gemini-weekly", RemainingFraction: 0.75, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
								{BucketID: "gemini-5h", RemainingFraction: 0.80, ResetTime: now.Add(3 * time.Hour).Format(time.RFC3339)},
							},
						}},
					},
				},
			},
			{
				Profile:    profLow,
				QuotaKnown: true,
				Data: &QuotaSummaryResponse{
					Response: QuotaResponse{
						Groups: []QuotaGroup{{
							DisplayName: "Gemini",
							Buckets: []QuotaBucket{
								{BucketID: "gemini-weekly", RemainingFraction: 0.12, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
								{BucketID: "gemini-5h", RemainingFraction: 0.15, ResetTime: now.Add(3 * time.Hour).Format(time.RFC3339)},
							},
						}},
					},
				},
			},
			{
				Profile:    profCooldown,
				QuotaKnown: true,
				Data: &QuotaSummaryResponse{
					Response: QuotaResponse{
						Groups: []QuotaGroup{{
							DisplayName: "Gemini",
							Buckets: []QuotaBucket{
								{BucketID: "gemini-weekly", RemainingFraction: 0.70, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
								{BucketID: "gemini-5h", RemainingFraction: 0.70, ResetTime: now.Add(3 * time.Hour).Format(time.RFC3339)},
							},
						}},
					},
				},
			},
		}, nil
	})
	defer cleanup()

	RecordProfileCooldown(profCooldown, 5*time.Minute)

	selected, err := PreflightProfilesForModel(3, "gemini-2.5-pro", WeightMedium, 0.20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selected) != 1 || selected[0].Profile != profHealthy {
		t.Fatalf("expected only [%s] to pass preflight (profLow below 0.20, profCooldown on cd), got %+v", profHealthy, selected)
	}

	// Putting profHealthy on cooldown leaves 0 profiles passing preflight -> should error
	RecordProfileCooldown(profHealthy, 5*time.Minute)
	defer RecordProfileCooldown(profHealthy, 0)

	_, err = PreflightProfilesForModel(1, "gemini-2.5-pro", WeightMedium, 0.20)
	if err == nil {
		t.Fatal("expected error when all eligible servers are on cooldown or below threshold, got nil")
	}
}
