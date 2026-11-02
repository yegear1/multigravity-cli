package quota

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSharedCooldownPool_Lifecycle(t *testing.T) {
	tempDir := t.TempDir()
	restore := SetTestCooldownPoolPath(filepath.Join(tempDir, "test-cooldown-pool.json"))
	defer restore()

	ResetSharedCooldowns()
	defer ResetSharedCooldowns()

	target := "profile:dev-test-1"
	onCd, _, _ := IsTargetOnCooldown(target)
	if onCd {
		t.Fatalf("expected target %q not to be on cooldown initially", target)
	}

	// Record cooldown for 10 minutes
	err := RecordCooldown(target, 10*time.Minute, "rate limited on model", "test")
	if err != nil {
		t.Fatalf("unexpected error recording cooldown: %v", err)
	}

	onCd, until, reason := IsTargetOnCooldown(target)
	if !onCd {
		t.Fatalf("expected target %q to be on active cooldown", target)
	}
	if reason != "rate limited on model" {
		t.Errorf("expected reason 'rate limited on model', got %q", reason)
	}
	if time.Until(until) <= 0 {
		t.Errorf("expected until to be in future, got %v", until)
	}

	// Clear cooldown
	err = RecordCooldown(target, 0, "", "test")
	if err != nil {
		t.Fatalf("unexpected error clearing cooldown: %v", err)
	}

	onCd, _, _ = IsTargetOnCooldown(target)
	if onCd {
		t.Fatalf("expected target %q to be cleared from cooldown", target)
	}
}

func TestSharedCooldownPool_ModelAndBucket(t *testing.T) {
	tempDir := t.TempDir()
	restore := SetTestCooldownPoolPath(filepath.Join(tempDir, "test-model-cooldown-pool.json"))
	defer restore()

	ResetSharedCooldowns()
	defer ResetSharedCooldowns()

	// 1. Exact model cooldown
	RecordModelCooldown("gemini-4-argon", 5*time.Minute, "quota exhausted for argon")
	onCd, _, reason := IsModelOnCooldown("gemini-4-argon")
	if !onCd {
		t.Fatalf("expected model gemini-4-argon to be on cooldown")
	}
	if reason != "quota exhausted for argon" {
		t.Errorf("expected reason 'quota exhausted for argon', got %q", reason)
	}

	// Base model match with suffix: gemini-4-argon-medium should match base gemini-4-argon
	onCdMedium, _, _ := IsModelOnCooldown("gemini-4-argon-medium")
	if !onCdMedium {
		t.Fatalf("expected model gemini-4-argon-medium to inherit cooldown from gemini-4-argon")
	}

	// Unrelated model should not be on cooldown
	onCdFlash, _, _ := IsModelOnCooldown("gemini-3.8-flash")
	if onCdFlash {
		t.Fatalf("gemini-3.8-flash should not be on cooldown")
	}

	// 2. Clear model cooldown
	RecordModelCooldown("gemini-4-argon", 0, "")
	onCd, _, _ = IsModelOnCooldown("gemini-4-argon")
	if onCd {
		t.Fatalf("expected gemini-4-argon cooldown to be cleared")
	}

	// 3. Bucket cooldown
	RecordBucketCooldown("argon", 5*time.Minute, "bucket argon throttled")
	onCdBucket, _, bReason := IsModelOnCooldown("gemini-4-argon-high")
	if !onCdBucket {
		t.Fatalf("expected gemini-4-argon-high to be on cooldown via argon bucket")
	}
	if bReason != "bucket argon throttled" {
		t.Errorf("expected 'bucket argon throttled', got %q", bReason)
	}

	// Clear bucket
	RecordBucketCooldown("argon", 0, "")

	// 4. Global cooldown
	RecordGlobalCooldown(5*time.Minute, "provider outage")
	onCdGlobal, _, gReason := IsModelOnCooldown("any-other-model")
	if !onCdGlobal {
		t.Fatalf("expected any model to be on cooldown when global cooldown active")
	}
	if gReason != "provider outage" {
		t.Errorf("expected 'provider outage', got %q", gReason)
	}
}

func TestSharedCooldownPool_PersistenceAndLocking(t *testing.T) {
	tempDir := t.TempDir()
	poolFile := filepath.Join(tempDir, "shared-pool.json")
	restore := SetTestCooldownPoolPath(poolFile)
	defer restore()

	ResetSharedCooldowns()
	defer ResetSharedCooldowns()

	// Concurrent writes
	var wg sync.WaitGroup
	workers := 10
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tgt := fmt.Sprintf("profile:worker-%d", idx)
			_ = RecordCooldown(tgt, 10*time.Minute, "concurrent test", "worker")
		}(i)
	}
	wg.Wait()

	// Verify all entries persisted and readable
	for i := 0; i < workers; i++ {
		tgt := fmt.Sprintf("profile:worker-%d", i)
		onCd, _, _ := IsTargetOnCooldown(tgt)
		if !onCd {
			t.Errorf("expected target %s to be on active cooldown", tgt)
		}
	}
}

func TestSharedCooldownPool_RoutingPreflightRejection(t *testing.T) {
	tempDir := t.TempDir()
	restore := SetTestCooldownPoolPath(filepath.Join(tempDir, "preflight-pool.json"))
	defer restore()

	ResetSharedCooldowns()
	defer ResetSharedCooldowns()

	model := "gemini-3.8-flash-medium"
	RecordModelCooldown(model, 10*time.Minute, "rate limit 429")
	defer RecordModelCooldown(model, 0, "")

	candidates := []RouteCandidate{
		{Profile: "p1", Eligible: true, RemainingFraction: 0.90, Score: 900},
		{Profile: "p2", Eligible: true, RemainingFraction: 0.85, Score: 850},
	}

	_, err := PreflightProfilesForModelFromCandidates(candidates, 1, model, 0.05)
	if err == nil {
		t.Fatal("expected error from PreflightProfilesForModelFromCandidates when model is on cooldown, got nil")
	}

	expectedSub := "is under active shared cooldown"
	if !stringsContains(err.Error(), expectedSub) {
		t.Fatalf("expected error containing %q, got: %v", expectedSub, err)
	}
}

func TestParseRetryAfterHeader(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"", 0},
		{"   ", 0},
		{"120", 120 * time.Second},
		{"30s", 30 * time.Second},
		{"5m", 5 * time.Minute},
		{"invalid", 0},
	}

	for _, tc := range tests {
		got := ParseRetryAfterHeader(tc.input)
		if got != tc.expected {
			t.Errorf("ParseRetryAfterHeader(%q) = %v, expected %v", tc.input, got, tc.expected)
		}
	}

	// Test HTTP-date format (RFC1123)
	future := time.Now().Add(60 * time.Second).UTC()
	dateStr := future.Format(http.TimeFormat)
	gotDate := ParseRetryAfterHeader(dateStr)
	if gotDate < 50*time.Second || gotDate > 70*time.Second {
		t.Errorf("ParseRetryAfterHeader(%q) = %v, expected ~60s", dateStr, gotDate)
	}
}

func TestParseQuotaResetDuration_RetryAfterSupport(t *testing.T) {
	// 1. Plain integer seconds
	d1 := ParseQuotaResetDuration("180", 5*time.Minute)
	if d1 != 180*time.Second {
		t.Errorf("expected 180s, got %v", d1)
	}

	// 2. Duration string
	d2 := ParseQuotaResetDuration("3m", 5*time.Minute)
	if d2 != 3*time.Minute {
		t.Errorf("expected 3m, got %v", d2)
	}

	// 3. Legacy "Resets in 45m10s."
	d3 := ParseQuotaResetDuration("Resource exhausted. Resets in 45m10s. Please retry later.", 5*time.Minute)
	expected := 45*time.Minute + 10*time.Second
	if d3 != expected {
		t.Errorf("expected %v, got %v", expected, d3)
	}
}

func stringsContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && containsSub(s, sub)))
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
