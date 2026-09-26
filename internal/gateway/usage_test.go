package gateway

import (
	"bytes"
	"encoding/json"
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
