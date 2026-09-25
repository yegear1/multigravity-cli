package gateway

import (
	"context"
	"testing"
	"time"
)

func TestParseRoutingStrategy(t *testing.T) {
	tests := []struct {
		input    string
		expected RoutingStrategy
	}{
		{"smart", StrategySmart},
		{"smart-priority", StrategySmart},
		{"round-robin", StrategyRoundRobin},
		{"roundrobin", StrategyRoundRobin},
		{"rr", StrategyRoundRobin},
		{"priority", StrategyPriority},
		{"fallback", StrategyPriority},
		{"ordered", StrategyPriority},
		{"sticky", StrategySticky},
		{"affinity", StrategySticky},
		{"unknown", StrategySmart},
		{"", StrategySmart},
	}

	for _, tt := range tests {
		got := ParseRoutingStrategy(tt.input)
		if got != tt.expected {
			t.Errorf("ParseRoutingStrategy(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestRouterSmartStrategy(t *testing.T) {
	router := NewRouter(WithRouterStrategy(StrategySmart))

	mockNow := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	router.SetNowFunc(func() time.Time { return mockNow })

	// Register 3 profiles with different remaining quota fractions
	p1 := router.RegisterProfile("p1-low")
	p1.RemainingFraction = 0.20

	p2 := router.RegisterProfile("p2-high")
	p2.RemainingFraction = 0.95

	p3 := router.RegisterProfile("p3-med")
	p3.RemainingFraction = 0.50

	// 1. P2 should be chosen first because it has the highest quota fraction
	chosen, err := router.SelectProfile(context.Background(), "gemini-2.5-pro", "", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chosen.Name != "p2-high" {
		t.Errorf("expected p2-high, got %s", chosen.Name)
	}

	// 2. Mark P2 as rate-limited with 60s cooldown
	router.MarkRateLimited("p2-high", 429, "Too Many Requests", 60*time.Second)

	// Next selection should pick P3 (highest remaining among non-cooling down nodes)
	chosen2, err := router.SelectProfile(context.Background(), "gemini-2.5-pro", "", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chosen2.Name != "p3-med" {
		t.Errorf("expected p3-med, got %s", chosen2.Name)
	}

	// 3. Fast-forward time past P2's cooldown
	mockNow = mockNow.Add(70 * time.Second)

	// Now P2 should be eligible again
	chosen3, err := router.SelectProfile(context.Background(), "gemini-2.5-pro", "", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chosen3.Name != "p2-high" {
		t.Errorf("expected p2-high after cooldown expiry, got %s", chosen3.Name)
	}
}

func TestRouterRoundRobinStrategy(t *testing.T) {
	router := NewRouter(WithRouterStrategy(StrategyRoundRobin))
	router.SyncProfiles([]string{"prof-a", "prof-b", "prof-c"})

	selections := make([]string, 6)
	for i := 0; i < 6; i++ {
		node, err := router.SelectProfile(context.Background(), "gemini-2.5-pro", "", nil, "")
		if err != nil {
			t.Fatalf("attempt %d failed: %v", i, err)
		}
		selections[i] = node.Name
	}

	expected := []string{"prof-a", "prof-b", "prof-c", "prof-a", "prof-b", "prof-c"}
	for i, exp := range expected {
		if selections[i] != exp {
			t.Errorf("selection[%d] = %q; want %q", i, selections[i], exp)
		}
	}
}

func TestRouterPriorityStrategy(t *testing.T) {
	router := NewRouter(WithRouterStrategy(StrategyPriority))
	router.SyncProfiles([]string{"alpha", "beta", "gamma"})

	// Always selects alpha first as long as it is healthy
	for i := 0; i < 3; i++ {
		node, err := router.SelectProfile(context.Background(), "gemini-2.5-pro", "", nil, "")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if node.Name != "alpha" {
			t.Errorf("expected alpha, got %s", node.Name)
		}
	}

	// Rate limit alpha
	router.MarkRateLimited("alpha", 429, "Rate limited", 10*time.Minute)

	// Next selection should be beta
	node, err := router.SelectProfile(context.Background(), "gemini-2.5-pro", "", nil, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if node.Name != "beta" {
		t.Errorf("expected beta, got %s", node.Name)
	}
}

func TestRouterStickyStrategy(t *testing.T) {
	router := NewRouter(WithRouterStrategy(StrategySticky))
	router.SyncProfiles([]string{"p1", "p2"})

	// First pick selects one (via smart score)
	node1, err := router.SelectProfile(context.Background(), "gemini-2.5-pro", "", nil, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Subsequent picks stick to the same profile
	for i := 0; i < 3; i++ {
		node, err := router.SelectProfile(context.Background(), "gemini-2.5-pro", "", nil, "")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if node.Name != node1.Name {
			t.Errorf("expected sticky %s, got %s", node1.Name, node.Name)
		}
	}

	// When rate limited, it must unstick and switch to the other
	router.MarkRateLimited(node1.Name, 429, "Rate limit", 5*time.Minute)

	node2, err := router.SelectProfile(context.Background(), "gemini-2.5-pro", "", nil, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if node2.Name == node1.Name {
		t.Errorf("expected switch away from cooling down node, got %s", node2.Name)
	}
}

func TestRouterCooldownAndReset(t *testing.T) {
	router := NewRouter()
	router.SyncProfiles([]string{"p1", "p2"})

	mockNow := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	router.SetNowFunc(func() time.Time { return mockNow })

	router.MarkRateLimited("p1", 429, "Too Many Requests", 60*time.Second)
	router.MarkRateLimited("p2", 403, "Quota Exceeded", 60*time.Second)

	// Both are cooling down
	_, err := router.SelectProfile(context.Background(), "gemini-2.5-pro", "", nil, "")
	if err == nil {
		t.Fatalf("expected ErrAllProfilesCoolingDown, got nil")
	}

	// Status check
	status := router.GetStatus()
	if status.CooldownProfiles != 2 || status.HealthyProfiles != 0 {
		t.Errorf("expected 2 cooldown profiles, got %+v", status)
	}

	// Reset cooldowns manually
	router.ResetCooldowns()
	statusAfter := router.GetStatus()
	if statusAfter.CooldownProfiles != 0 || statusAfter.HealthyProfiles != 2 {
		t.Errorf("expected 0 cooldown profiles after reset, got %+v", statusAfter)
	}

	// Selection works again
	node, err := router.SelectProfile(context.Background(), "gemini-2.5-pro", "", nil, "")
	if err != nil {
		t.Fatalf("unexpected error after reset: %v", err)
	}
	if node == nil {
		t.Fatalf("expected node, got nil")
	}
}

func TestRouterExplicitProfileWithFailover(t *testing.T) {
	router := NewRouter(WithFailoverEnabled(true))
	router.SyncProfiles([]string{"target", "backup"})

	mockNow := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	router.SetNowFunc(func() time.Time { return mockNow })

	// When target is healthy, returns target
	node, err := router.SelectProfile(context.Background(), "gemini-2.5-pro", "target", nil, "")
	if err != nil || node.Name != "target" {
		t.Fatalf("expected target, got %v (%v)", node, err)
	}

	// Rate limit target
	router.MarkRateLimited("target", 429, "rate limited", 10*time.Minute)

	// With failover enabled, requesting target falls over to backup
	node2, err := router.SelectProfile(context.Background(), "gemini-2.5-pro", "target", nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node2.Name != "backup" {
		t.Errorf("expected failover to backup, got %s", node2.Name)
	}

	// With failover disabled, requesting target returns error
	router.SetFailover(false)
	_, err = router.SelectProfile(context.Background(), "gemini-2.5-pro", "target", nil, "")
	if err == nil {
		t.Errorf("expected error when failover disabled and profile is cooling down")
	}
}
