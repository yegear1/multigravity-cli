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
