package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func TestRouterScoreDepletionPenalty(t *testing.T) {
	router := NewRouter(WithRouterStrategy(StrategySmart))

	mockNow := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	router.SetNowFunc(func() time.Time { return mockNow })

	// pHealthy has 50% remaining quota
	pHealthy := router.RegisterProfile("healthy")
	pHealthy.QuotaKnown = true
	pHealthy.RemainingFraction = 0.50

	// pDepleted has 8% remaining quota (<= 0.10 depletion threshold)
	pDepleted := router.RegisterProfile("depleted")
	pDepleted.QuotaKnown = true
	pDepleted.RemainingFraction = 0.08

	scoreHealthy := pHealthy.Score(mockNow)
	scoreDepleted := pDepleted.Score(mockNow)

	if scoreHealthy <= scoreDepleted {
		t.Fatalf("expected healthy score (%f) to be significantly higher than depleted score (%f)", scoreHealthy, scoreDepleted)
	}

	// Smart strategy should select healthy node over depleted node
	chosen, err := router.SelectProfile(context.Background(), "gemini-3.8-flash-tiered", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if chosen.Name != "healthy" {
		t.Fatalf("expected healthy profile, got %s", chosen.Name)
	}
}

func TestRouterAutoThrottleEngaged(t *testing.T) {
	cfg := &quota.ThrottleConfig{
		DefaultPolicy: quota.ThrottlePolicy{
			DepletionThreshold:  0.15,
			ThrottleDelayMs:     50,
			AutoThrottleEnabled: true,
		},
		Profiles: make(map[string]quota.ThrottlePolicy),
	}
	router := NewRouter(WithRouterThrottleConfig(cfg))

	p := router.RegisterProfile("nearing-depletion")
	p.QuotaKnown = true
	p.RemainingFraction = 0.12 // <= 0.15

	chosen, err := router.SelectProfile(context.Background(), "gemini-3.8-flash-tiered", "nearing-depletion", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !chosen.IsCurrentlyThrottled() {
		t.Fatal("expected profile to be marked as throttled")
	}
	if chosen.GetThrottleDelay() != 50*time.Millisecond {
		t.Fatalf("expected 50ms throttle delay, got %v", chosen.GetThrottleDelay())
	}

	status := router.GetStatus()
	if len(status.Profiles) != 1 || !status.Profiles[0].IsThrottled {
		t.Fatalf("expected router status to report is_throttled=true: %+v", status)
	}
}

func TestGatewayThrottleHeadersOnChatCompletions(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]string{{"text": "throttled response"}},
					},
					"finishReason": "STOP",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	cfg := &quota.ThrottleConfig{
		DefaultPolicy: quota.ThrottlePolicy{
			DepletionThreshold:  0.20,
			ThrottleDelayMs:     10,
			AutoThrottleEnabled: true,
		},
		Profiles: make(map[string]quota.ThrottlePolicy),
	}
	router := NewRouter(WithRouterThrottleConfig(cfg))
	node := router.RegisterProfile("depleted-prof")
	node.QuotaKnown = true
	node.RemainingFraction = 0.10

	client := NewClient(WithEndpoints([]string{upstream.URL}))
	gw := NewGateway(
		WithGatewayClient(client),
		WithGatewayRouter(router),
		WithTokenResolver(func(r *http.Request, profileName string) (string, error) {
			return "test-token", nil
		}),
	)

	reqBody := `{"model": "gemini-3.8-flash-tiered", "messages": [{"role": "user", "content": "hello"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(reqBody))
	req.Header.Set("X-Profile", "depleted-prof")
	w := httptest.NewRecorder()

	gw.HandleChatCompletions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Throttled") != "true" {
		t.Errorf("expected X-Throttled header to be 'true', got %q", w.Header().Get("X-Throttled"))
	}
	if w.Header().Get("X-Throttle-Delay") != "10ms" {
		t.Errorf("expected X-Throttle-Delay header to be '10ms', got %q", w.Header().Get("X-Throttle-Delay"))
	}
}

func TestGatewayThrottleHeadersOnAnthropicMessages(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]string{{"text": "anthropic response"}},
					},
					"finishReason": "STOP",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	cfg := &quota.ThrottleConfig{
		DefaultPolicy: quota.ThrottlePolicy{
			DepletionThreshold:  0.20,
			ThrottleDelayMs:     10,
			AutoThrottleEnabled: true,
		},
		Profiles: make(map[string]quota.ThrottlePolicy),
	}
	router := NewRouter(WithRouterThrottleConfig(cfg))
	node := router.RegisterProfile("depleted-anthropic")
	node.QuotaKnown = true
	node.RemainingFraction = 0.10

	client := NewClient(WithEndpoints([]string{upstream.URL}))
	gw := NewGateway(
		WithGatewayClient(client),
		WithGatewayRouter(router),
		WithTokenResolver(func(r *http.Request, profileName string) (string, error) {
			return "test-token", nil
		}),
	)

	reqBody := `{"model": "claude-sonnet-4-6", "max_tokens": 100, "messages": [{"role": "user", "content": "hi"}]}`
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewBufferString(reqBody))
	req.Header.Set("X-Profile", "depleted-anthropic")
	w := httptest.NewRecorder()

	gw.HandleMessages(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Throttled") != "true" {
		t.Errorf("expected X-Throttled header to be 'true', got %q", w.Header().Get("X-Throttled"))
	}
	if w.Header().Get("X-Throttle-Delay") != "10ms" {
		t.Errorf("expected X-Throttle-Delay header to be '10ms', got %q", w.Header().Get("X-Throttle-Delay"))
	}
}
