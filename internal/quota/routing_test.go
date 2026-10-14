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
	if planLight.Recommended.Model != "gemini-2.5-flash" {
		t.Fatalf("expected light model gemini-2.5-flash, got %s", planLight.Recommended.Model)
	}
	if planLight.Recommended.WindowType != Window5h {
		t.Fatalf("expected light window to be 5h, got %s", planLight.Recommended.WindowType)
	}

	// Medium workload
	planMedium := RecommendRoute([]ActiveServer{serverWork}, WeightMedium, now)
	if planMedium.Recommended == nil {
		t.Fatal("expected recommendation for medium, got nil")
	}
	if planMedium.Recommended.Model != "gemini-3.8-flash-tiered" {
		t.Fatalf("expected medium model gemini-3.8-flash-tiered, got %s", planMedium.Recommended.Model)
	}

	// Heavy workload
	planHeavy := RecommendRoute([]ActiveServer{serverWork}, WeightHeavy, now)
	if planHeavy.Recommended == nil {
		t.Fatal("expected recommendation for heavy, got nil")
	}
	if planHeavy.Recommended.Model != "gemini-2.5-pro" {
		t.Fatalf("expected heavy model gemini-2.5-pro, got %s", planHeavy.Recommended.Model)
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
}
