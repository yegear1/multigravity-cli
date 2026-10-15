package quota

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCalculateTokenBurnRate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	profileName := "testprof"
	if err := os.MkdirAll(filepath.Join(home, profileName), 0o755); err != nil {
		t.Fatal(err)
	}

	baseTime := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	prevNow := historyNow
	historyNow = func() time.Time { return baseTime }
	defer func() { historyNow = prevNow }()

	// Record 3 token events at baseTime - 40m, baseTime - 20m, baseTime
	historyNow = func() time.Time { return baseTime.Add(-40 * time.Minute) }
	if err := RecordTokens(profileName, SourceGateway, "gemini-3.8-flash-tiered", 100, 200, 300, false); err != nil {
		t.Fatal(err)
	}

	historyNow = func() time.Time { return baseTime.Add(-20 * time.Minute) }
	if err := RecordTokens(profileName, SourceGateway, "gemini-3.8-flash-tiered", 200, 300, 500, false); err != nil {
		t.Fatal(err)
	}

	historyNow = func() time.Time { return baseTime }
	if err := RecordTokens(profileName, SourceHeadless, "gemini-3.8-flash-tiered", 100, 100, 200, true); err != nil {
		t.Fatal(err)
	}

	// Calculate 1h window: all 3 samples included (300 + 500 + 200 = 1000 tokens)
	burn1h, err := CalculateTokenBurnRate(profileName, 1*time.Hour, baseTime)
	if err != nil {
		t.Fatal(err)
	}
	if burn1h.TokensBurned != 1000 {
		t.Fatalf("expected 1000 tokens burned, got %d", burn1h.TokensBurned)
	}
	if burn1h.SampleCount != 3 {
		t.Fatalf("expected 3 samples, got %d", burn1h.SampleCount)
	}
	if burn1h.TokensPerHour != 1000.0 {
		t.Fatalf("expected 1000.0 tokens/hr, got %f", burn1h.TokensPerHour)
	}
	if burn1h.TokensPerMinute != 1000.0/60.0 {
		t.Fatalf("expected ~16.66 tokens/min, got %f", burn1h.TokensPerMinute)
	}

	// Calculate 30m window: only baseTime - 20m and baseTime included (500 + 200 = 700 tokens)
	burn30m, err := CalculateTokenBurnRate(profileName, 30*time.Minute, baseTime)
	if err != nil {
		t.Fatal(err)
	}
	if burn30m.TokensBurned != 700 {
		t.Fatalf("expected 700 tokens burned, got %d", burn30m.TokensBurned)
	}
	if burn30m.SampleCount != 2 {
		t.Fatalf("expected 2 samples, got %d", burn30m.SampleCount)
	}
}

func TestCalculateTokenBurnRateEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	profileName := "emptyprof"
	if err := os.MkdirAll(filepath.Join(home, profileName), 0o755); err != nil {
		t.Fatal(err)
	}

	burn, err := CalculateTokenBurnRate(profileName, time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if burn.TokensBurned != 0 || burn.SampleCount != 0 {
		t.Fatalf("expected 0 tokens burned and 0 samples, got %d and %d", burn.TokensBurned, burn.SampleCount)
	}
}

func TestCalculateTokenBurnRateMissingProfile(t *testing.T) {
	t.Setenv("MULTIGRAVITY_HOME", t.TempDir())
	if _, err := CalculateTokenBurnRate("nonexistent", time.Hour, time.Now()); err == nil {
		t.Fatal("expected error for nonexistent profile")
	}
}

func TestCalculateFleetBurnRates(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "p1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "p2"), 0o755); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	prevNow := historyNow
	historyNow = func() time.Time { return now }
	defer func() { historyNow = prevNow }()

	if err := RecordTokens("p1", SourceGateway, "gemini-2.5-flash", 50, 50, 100, false); err != nil {
		t.Fatal(err)
	}

	fleet, err := CalculateFleetBurnRates(time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(fleet) != 2 {
		t.Fatalf("expected 2 profiles in fleet, got %d", len(fleet))
	}
	if fleet["p1"].TokensBurned != 100 {
		t.Fatalf("expected 100 tokens burned for p1, got %d", fleet["p1"].TokensBurned)
	}
	if fleet["p2"].TokensBurned != 0 {
		t.Fatalf("expected 0 tokens burned for p2, got %d", fleet["p2"].TokensBurned)
	}
}

func TestThrottleConfigPersistenceAndOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "throttle.json")
	restore := SetTestThrottleConfigPath(cfgPath)
	defer restore()

	// Load when file does not exist -> defaults
	cfg, err := LoadThrottleConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultPolicy.MaxTokensPerHour != DefaultMaxTokensPerHour {
		t.Fatalf("expected default %d, got %d", DefaultMaxTokensPerHour, cfg.DefaultPolicy.MaxTokensPerHour)
	}
	if cfg.DefaultPolicy.DepletionThreshold != DefaultDepletionThreshold {
		t.Fatalf("expected default %f, got %f", DefaultDepletionThreshold, cfg.DefaultPolicy.DepletionThreshold)
	}

	// Policy for profile with no override
	pDefault := cfg.PolicyForProfile("anyprof")
	if pDefault.MaxTokensPerHour != DefaultMaxTokensPerHour {
		t.Fatalf("expected default policy for anyprof, got %d", pDefault.MaxTokensPerHour)
	}

	// Add profile override and save
	cfg.Profiles["heavyprof"] = ThrottlePolicy{
		Profile:             "heavyprof",
		MaxTokensPerHour:    100000,
		MaxTokensPerMinute:  5000,
		DepletionThreshold:  0.08,
		ThrottleDelayMs:     500,
		AutoThrottleEnabled: true,
	}
	if err := SaveThrottleConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Reload
	reloaded, err := LoadThrottleConfig()
	if err != nil {
		t.Fatal(err)
	}
	heavyPol := reloaded.PolicyForProfile("heavyprof")
	if heavyPol.MaxTokensPerHour != 100000 || heavyPol.DepletionThreshold != 0.08 {
		t.Fatalf("unexpected heavy policy: %+v", heavyPol)
	}
	if heavyPol.ThrottleDelay() != 500*time.Millisecond {
		t.Fatalf("expected 500ms delay, got %v", heavyPol.ThrottleDelay())
	}
}
