package alert

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/headless"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func TestEvaluateQuotaCriticalAndDrop(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	recordQuota(t, "work", "gemini-weekly", 0.40)
	recordQuota(t, "work", "gemini-weekly", 0.04)

	report, err := Evaluate("work")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Alerts) != 2 {
		t.Fatalf("alerts: %+v", report.Alerts)
	}
	if report.Alerts[0].Kind != KindQuotaCritical || report.Alerts[0].BucketID != "gemini-weekly" {
		t.Fatalf("critical: %+v", report.Alerts[0])
	}
	if report.Alerts[0].RemainingFraction == nil || *report.Alerts[0].RemainingFraction != 0.04 {
		t.Fatalf("remaining: %+v", report.Alerts[0].RemainingFraction)
	}
	if report.Alerts[1].Kind != KindQuotaDrop || report.Alerts[1].PreviousFraction == nil || *report.Alerts[1].PreviousFraction != 0.40 {
		t.Fatalf("drop: %+v", report.Alerts[1])
	}
}

func TestEvaluateIgnoresSmallMovement(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	recordQuota(t, "work", "gemini-weekly", 0.90)
	recordQuota(t, "work", "gemini-weekly", 0.80)
	recordQuota(t, "work", "gemini-weekly", 0.79)

	report, err := Evaluate("work")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Alerts) != 0 {
		t.Fatalf("expected no alerts, got %+v", report.Alerts)
	}
}

func TestEvaluateOrphanReapsDeadHeadless(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "work", ".multigravity"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeHeadlessState(t, home, "work", 888888)

	restore := headless.SetTestHooks(nil, nil, func(int) bool { return false }, nil, nil)
	defer restore()

	report, err := Evaluate("work")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Alerts) != 1 || report.Alerts[0].Kind != KindOrphanProcess || report.Alerts[0].PID != 888888 {
		t.Fatalf("alerts: %+v", report.Alerts)
	}
	statePath := filepath.Join(home, "work", ".multigravity", "headless.json")
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatal("expected stale headless state to be removed")
	}

	again, err := Evaluate("work")
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Alerts) != 0 {
		t.Fatalf("second pass should be quiet, got %+v", again.Alerts)
	}
}

func TestEvaluateLeavesLiveHeadless(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "work", ".multigravity"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeHeadlessState(t, home, "work", 4242)

	restore := headless.SetTestHooks(nil, nil, func(int) bool { return true }, nil, nil)
	defer restore()

	report, err := Evaluate("")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Alerts) != 0 {
		t.Fatalf("live process must not alert, got %+v", report.Alerts)
	}
	if _, err := os.Stat(filepath.Join(home, "work", ".multigravity", "headless.json")); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateMissingProfile(t *testing.T) {
	t.Setenv("MULTIGRAVITY_HOME", t.TempDir())
	if _, err := Evaluate("missing"); err == nil {
		t.Fatal("expected missing profile error")
	}
}

func TestEvaluateQuotaNearingDepletion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 0.08 is <= 0.10 (nearing depletion) but > 0.05 (critical)
	recordQuota(t, "work", "gemini-weekly", 0.08)

	report, err := Evaluate("work")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Alerts) != 1 {
		t.Fatalf("expected 1 alert, got %+v", report.Alerts)
	}
	if report.Alerts[0].Kind != KindQuotaNearingDepletion {
		t.Fatalf("expected KindQuotaNearingDepletion, got %s", report.Alerts[0].Kind)
	}
	if report.Alerts[0].RemainingFraction == nil || *report.Alerts[0].RemainingFraction != 0.08 {
		t.Fatalf("expected 0.08, got %+v", report.Alerts[0].RemainingFraction)
	}
}

func TestEvaluateTokenBudgetExceededAndBurnRate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "work"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(home, ".multigravity", "throttle-policy.json")
	restore := quota.SetTestThrottleConfigPath(cfgPath)
	defer restore()

	cfg := &quota.ThrottleConfig{
		DefaultPolicy: quota.ThrottlePolicy{
			MaxTokensPerHour:    1000,
			MaxTokensPerMinute:  20,
			DepletionThreshold:  0.10,
			AutoThrottleEnabled: true,
		},
		Profiles: make(map[string]quota.ThrottlePolicy),
	}
	if err := quota.SaveThrottleConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Record 1500 tokens in the past hour
	if err := quota.RecordTokens("work", quota.SourceGateway, "gemini-3.8-flash-tiered", 500, 1000, 1500, false); err != nil {
		t.Fatal(err)
	}

	report, err := Evaluate("work")
	if err != nil {
		t.Fatal(err)
	}
	// 1500 tokens in 1h window:
	// tokens_burned = 1500 >= 1000 -> KindTokenBudgetExceeded
	// tokens_per_min = 1500 / 60 = 25 >= 20 -> KindTokenBurnRateHigh
	if len(report.Alerts) != 2 {
		t.Fatalf("expected 2 alerts, got %+v", report.Alerts)
	}
	if report.Alerts[0].Kind != KindTokenBudgetExceeded || report.Alerts[0].TokensBurned != 1500 {
		t.Fatalf("unexpected budget alert: %+v", report.Alerts[0])
	}
	if report.Alerts[1].Kind != KindTokenBurnRateHigh || report.Alerts[1].BudgetLimit != 20 {
		t.Fatalf("unexpected burn rate alert: %+v", report.Alerts[1])
	}
}

func recordQuota(t *testing.T, profileName, bucketID string, fraction float64) {
	t.Helper()
	err := quota.RecordQuotaSnapshot(profileName, &quota.QuotaSummaryResponse{
		Response: quota.QuotaResponse{
			Groups: []quota.QuotaGroup{{
				Buckets: []quota.QuotaBucket{{
					BucketID:          bucketID,
					RemainingFraction: fraction,
					ResetTime:         time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
				}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func writeHeadlessState(t *testing.T, home, profileName string, pid int) {
	t.Helper()
	info := headless.InstanceInfo{
		Profile:   profileName,
		PID:       pid,
		Port:      1234,
		CSRFToken: "stale",
		Status:    headless.StateRunning,
		StartedAt: time.Now().UTC(),
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, profileName, ".multigravity", "headless.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
