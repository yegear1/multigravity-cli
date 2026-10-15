package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func TestGatewayRecordsTokenSeries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	if err := os.MkdirAll(config.GetProfileDir("primary"), 0o755); err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello\"}]}}],\"usageMetadata\":{\"promptTokenCount\":5,\"candidatesTokenCount\":7,\"totalTokenCount\":12}}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	client := NewClient(WithEndpoints([]string{upstream.URL}), WithHTTPClient(upstream.Client()))
	router := NewRouter()
	router.SyncProfiles([]string{"primary"})
	gw := NewGateway(WithGatewayClient(client), WithGatewayRouter(router))

	body := `{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	gw.HandleChatCompletions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	var resp ChatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Usage.TotalTokens != 12 || resp.Usage.PromptTokens != 5 || resp.Usage.CompletionTokens != 7 {
		t.Fatalf("usage: %+v", resp.Usage)
	}

	series, err := quota.LoadSeries("primary", quota.HistoryQuery{Source: quota.SourceGateway, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Samples) != 1 || series.Samples[0].Tokens.Estimated || series.Summary.TotalTokens != 12 {
		t.Fatalf("series: %+v", series)
	}
	if _, err := os.Stat(filepath.Join(config.GetProfileDir("primary"), ".multigravity", "quota-history.jsonl")); err != nil {
		t.Fatal(err)
	}
}

func TestGatewayEstimatesTokensWithoutUsageMetadata(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	if err := os.MkdirAll(config.GetProfileDir("primary"), 0o755); err != nil {
		t.Fatal(err)
	}

	const prompt = "Hi"
	const completion = "Hello"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":%q}]}}]}}\n\n", completion)
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	client := NewClient(WithEndpoints([]string{upstream.URL}), WithHTTPClient(upstream.Client()))
	router := NewRouter()
	router.SyncProfiles([]string{"primary"})
	gw := NewGateway(WithGatewayClient(client), WithGatewayRouter(router))

	body := fmt.Sprintf(`{"model":"gemini-2.5-pro","messages":[{"role":"user","content":%q}]}`, prompt)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	gw.HandleChatCompletions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	wantPrompt := quota.EstimateTokens(prompt)
	wantCompletion := quota.EstimateTokens(completion)
	var resp ChatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Usage.PromptTokens != wantPrompt || resp.Usage.CompletionTokens != wantCompletion || resp.Usage.TotalTokens != wantPrompt+wantCompletion {
		t.Fatalf("usage: %+v", resp.Usage)
	}

	series, err := quota.LoadSeries("primary", quota.HistoryQuery{Source: quota.SourceGateway, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Samples) != 1 || series.Samples[0].Tokens == nil || !series.Samples[0].Tokens.Estimated {
		t.Fatalf("series: %+v", series)
	}
	if series.Samples[0].Tokens.TotalTokens != wantPrompt+wantCompletion {
		t.Fatalf("sample tokens: %+v", series.Samples[0].Tokens)
	}
}

func TestGatewayCalibratesTokenEstimator(t *testing.T) {
	quota.ResetCalibration()
	defer quota.ResetCalibration()

	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	if err := os.MkdirAll(config.GetProfileDir("primary"), 0o755); err != nil {
		t.Fatal(err)
	}

	// 1. First upstream response contains live usageMetadata
	upstream1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"01234567890123456789\"}]}}],\"usageMetadata\":{\"promptTokenCount\":25,\"candidatesTokenCount\":25,\"totalTokenCount\":50}}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream1.Close()

	client1 := NewClient(WithEndpoints([]string{upstream1.URL}), WithHTTPClient(upstream1.Client()))
	router1 := NewRouter()
	router1.SyncProfiles([]string{"primary"})
	gw1 := NewGateway(WithGatewayClient(client1), WithGatewayRouter(router1))

	body1 := `{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"01234567890123456789"}]}`
	req1 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body1))
	req1.Header.Set("Authorization", "Bearer secret")
	rec1 := httptest.NewRecorder()
	gw1.HandleChatCompletions(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec1.Code, rec1.Body.String())
	}

	calStats := quota.GetCalibrationStats()
	if calStats.Samples != 1 {
		t.Fatalf("expected 1 calibration sample, got %d", calStats.Samples)
	}
	if calStats.RunesPerToken >= quota.DefaultRunesPerToken {
		t.Fatalf("expected runes per token < 4.0, got %f", calStats.RunesPerToken)
	}

	// 2. Second upstream response omits usageMetadata, relying on calibrated estimation
	upstream2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"01234567890123456789\"}]}}]}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream2.Close()

	client2 := NewClient(WithEndpoints([]string{upstream2.URL}), WithHTTPClient(upstream2.Client()))
	router2 := NewRouter()
	router2.SyncProfiles([]string{"primary"})
	gw2 := NewGateway(WithGatewayClient(client2), WithGatewayRouter(router2))

	req2 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body1))
	req2.Header.Set("Authorization", "Bearer secret")
	rec2 := httptest.NewRecorder()
	gw2.HandleChatCompletions(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec2.Code, rec2.Body.String())
	}

	var resp2 ChatCompletionResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatal(err)
	}

	// Uncalibrated for 20 runes prompt + 20 runes completion would be 5 + 5 = 10 tokens.
	// Calibrated ratio is denser (< 4.0), so estimated tokens should be > 10.
	if resp2.Usage.TotalTokens <= 10 {
		t.Fatalf("expected calibrated total tokens > 10, got %d", resp2.Usage.TotalTokens)
	}
}
