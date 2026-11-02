package gateway

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func TestClient_RetryAfterHeaderAndModelError(t *testing.T) {
	tempDir := t.TempDir()
	restore := quota.SetTestCooldownPoolPath(filepath.Join(tempDir, "client-cooldown-pool.json"))
	defer restore()

	quota.ResetSharedCooldowns()
	defer quota.ResetSharedCooldowns()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":429,"message":"Resource has been exhausted (e.g. check quota)."}}`))
	}))
	defer srv.Close()

	client := NewClient(WithEndpoints([]string{srv.URL}), WithHTTPClient(http.DefaultClient))
	req := &CloudCodeRequest{Model: "gemini-4-argon"}

	_, err := client.StreamGenerateContent(t.Context(), req, "test-token", nil)
	if err == nil {
		t.Fatal("expected 429 upstream error, got nil")
	}

	var upstreamErr *UpstreamHTTPError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("expected *UpstreamHTTPError, got %T: %v", err, err)
	}

	if upstreamErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", upstreamErr.StatusCode)
	}
	if upstreamErr.RetryAfter != 120*time.Second {
		t.Errorf("expected RetryAfter 120s, got %v", upstreamErr.RetryAfter)
	}
	if upstreamErr.Model != "gemini-4-argon" {
		t.Errorf("expected Model 'gemini-4-argon', got %q", upstreamErr.Model)
	}

	// Test RecordUpstreamRateLimit integrates with shared cooldown pool
	RecordUpstreamRateLimit("", "profile-a", err)

	onCd, remaining, reason := CheckModelCooldown("gemini-4-argon")
	if !onCd {
		t.Fatalf("expected gemini-4-argon to be cooling down after RecordUpstreamRateLimit")
	}
	if remaining <= 0 {
		t.Errorf("expected remaining cooldown duration > 0, got %v", remaining)
	}
	if reason == "" {
		t.Errorf("expected reason to be recorded")
	}

	if !quota.IsProfileOnCooldown("profile-a") {
		t.Errorf("expected profile-a to be on cooldown")
	}
}
