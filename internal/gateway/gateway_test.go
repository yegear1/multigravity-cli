package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "mg-gateway-home-")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("MULTIGRAVITY_HOME", dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestNormalizeModel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"gpt-4o", "gemini-2.5-pro"},
		{"gpt-4o-mini", "gemini-2.5-flash"},
		{"claude-3-5-sonnet", "claude-sonnet-4-6"},
		{"claude-3-7-sonnet", "gemini-3.6-flash-high"},
		{"gemini-2.5-pro", "gemini-2.5-pro"},
		{"gemini-2.5-flash", "gemini-2.5-flash"},
		{"unknown-pro-model", "gemini-2.5-pro"},
		{"unknown-flash-model", "gemini-2.5-flash"},
		{"", "gemini-2.5-pro"},
	}

	for _, tt := range tests {
		got := NormalizeModel(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeModel(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestListSupportedModels(t *testing.T) {
	resp := ListSupportedModels()
	if resp.Object != "list" {
		t.Fatalf("expected object 'list', got %q", resp.Object)
	}
	if len(resp.Data) == 0 {
		t.Fatalf("expected models in list, got 0")
	}
	foundGemini := false
	for _, m := range resp.Data {
		if m.ID == "gemini-2.5-pro" {
			foundGemini = true
			break
		}
	}
	if !foundGemini {
		t.Errorf("gemini-2.5-pro not found in models list")
	}
}

func TestCollapseOpenAIMessages(t *testing.T) {
	msgs := []ChatMessage{
		{
			Role:    "system",
			Content: "You are a helpful assistant.",
		},
		{
			Role:    "user",
			Content: "Hello world",
		},
		{
			Role:    "assistant",
			Content: "Hi there!",
		},
		{
			Role: "user",
			Content: []ChatMessagePart{
				{Type: "text", Text: "Look at this image:"},
				{
					Type: "image_url",
					ImageURL: &ImageURLParam{
						URL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
					},
				},
			},
		},
	}

	contents, sysPrompt := CollapseOpenAIMessages(msgs)
	if sysPrompt != "You are a helpful assistant." {
		t.Errorf("unexpected sysPrompt: %q", sysPrompt)
	}

	if len(contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(contents))
	}

	if contents[0].Role != "user" || len(contents[0].Parts) != 1 || contents[0].Parts[0].Text != "Hello world" {
		t.Errorf("unexpected first content: %+v", contents[0])
	}

	if contents[1].Role != "model" || len(contents[1].Parts) != 1 || contents[1].Parts[0].Text != "Hi there!" {
		t.Errorf("unexpected second content: %+v", contents[1])
	}

	// Multimodal turn
	last := contents[2]
	if last.Role != "user" {
		t.Errorf("expected role user for last content, got %q", last.Role)
	}
	if len(last.Parts) != 2 {
		t.Fatalf("expected 2 parts (image + text), got %d", len(last.Parts))
	}
	if last.Parts[0].InlineData == nil || last.Parts[0].InlineData.MimeType != "image/png" {
		t.Errorf("expected inline image part, got %+v", last.Parts[0])
	}
	if last.Parts[1].Text != "Look at this image:" {
		t.Errorf("expected text part, got %+v", last.Parts[1])
	}
}

func TestParseGoogleSSELine(t *testing.T) {
	// 1. Data chunk with content
	line1 := `data: {"response":{"candidates":[{"content":{"parts":[{"text":"Hello, "}]}}]}}`
	delta, finish, done := ParseGoogleSSELine(line1)
	if delta != "Hello, " || finish != "" || done {
		t.Errorf("line1 parsed incorrectly: delta=%q finish=%q done=%v", delta, finish, done)
	}

	// 2. Data chunk with finishReason
	line2 := `data: {"response":{"candidates":[{"content":{"parts":[{"text":"world!"}]},"finishReason":"STOP"}]}}`
	delta2, finish2, done2 := ParseGoogleSSELine(line2)
	if delta2 != "world!" || finish2 != "STOP" || done2 {
		t.Errorf("line2 parsed incorrectly: delta=%q finish=%q done=%v", delta2, finish2, done2)
	}

	// 3. DONE line
	line3 := `data: [DONE]`
	delta3, finish3, done3 := ParseGoogleSSELine(line3)
	if delta3 != "" || finish3 != "stop" || !done3 {
		t.Errorf("line3 parsed incorrectly: delta=%q finish=%q done=%v", delta3, finish3, done3)
	}

	// 4. Irrelevant or empty line
	delta4, finish4, done4 := ParseGoogleSSELine(": keep-alive")
	if delta4 != "" || finish4 != "" || done4 {
		t.Errorf("line4 parsed incorrectly: delta=%q finish=%q done=%v", delta4, finish4, done4)
	}
}

func TestCloudCodeClientHeaders(t *testing.T) {
	client := NewClient()
	headers := client.BuildRequestHeaders("test-token-123")

	if headers.Get("Authorization") != "Bearer test-token-123" {
		t.Errorf("expected Bearer test-token-123, got %q", headers.Get("Authorization"))
	}
	if headers.Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json, got %q", headers.Get("Content-Type"))
	}
	if headers.Get("X-Goog-Api-Client") == "" {
		t.Errorf("expected X-Goog-Api-Client header to be present")
	}

	// CRITICAL INVARIANT: NO x-goog-user-project
	if headers.Get("x-goog-user-project") != "" || headers.Get("X-Goog-User-Project") != "" {
		t.Fatalf("INVARIANT VIOLATION: x-goog-user-project must never be present!")
	}
}

func TestGatewayHandleModels(t *testing.T) {
	gw := NewGateway()

	req := httptest.NewRequest("GET", "/v1/models", nil)
	w := httptest.NewRecorder()

	gw.HandleModels(w, req)
	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var modelList ModelListResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelList); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if modelList.Object != "list" || len(modelList.Data) == 0 {
		t.Errorf("unexpected modelList: %+v", modelList)
	}
}

func TestGatewayChatCompletionsNonStreaming(t *testing.T) {
	// Mock Google upstream
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Invariant check: no x-goog-user-project
		if r.Header.Get("x-goog-user-project") != "" {
			t.Errorf("upstream received forbidden x-goog-user-project header")
		}

		if r.Header.Get("Authorization") != "Bearer mock-secret" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"response":{"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]}}`)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"response":{"candidates":[{"content":{"parts":[{"text":" from Antigravity!"}]}}]}}`)
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	client := NewClient(
		WithEndpoints([]string{upstream.URL}),
		WithHTTPClient(upstream.Client()),
	)

	gw := NewGateway(
		WithGatewayClient(client),
		WithTokenResolver(func(r *http.Request, p string) (string, error) {
			return "mock-secret", nil
		}),
	)

	body := `{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": "Say hello"}
		],
		"stream": false
	}`

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	gw.HandleChatCompletions(w, req)
	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var compResp ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&compResp); err != nil {
		t.Fatalf("failed to decode completion response: %v", err)
	}

	if compResp.Object != "chat.completion" {
		t.Errorf("expected object 'chat.completion', got %q", compResp.Object)
	}

	if len(compResp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(compResp.Choices))
	}

	if compResp.Choices[0].Message.Content != "Hello from Antigravity!" {
		t.Errorf("unexpected content: %q", compResp.Choices[0].Message.Content)
	}
}

func TestGatewayChatCompletionsStreaming(t *testing.T) {
	// Mock Google upstream
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"response":{"candidates":[{"content":{"parts":[{"text":"Streamed "}]}}]}}`)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"response":{"candidates":[{"content":{"parts":[{"text":"chunk."}]}}]}}`)
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	client := NewClient(
		WithEndpoints([]string{upstream.URL}),
		WithHTTPClient(upstream.Client()),
	)

	gw := NewGateway(
		WithGatewayClient(client),
		WithTokenResolver(func(r *http.Request, p string) (string, error) {
			return "mock-secret", nil
		}),
	)

	body := `{
		"model": "gemini-2.5-pro",
		"messages": [
			{"role": "user", "content": "Stream to me"}
		],
		"stream": true
	}`

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	gw.HandleChatCompletions(w, req)
	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Errorf("expected text/event-stream, got %q", contentType)
	}

	rawBody, _ := io.ReadAll(resp.Body)
	bodyStr := string(rawBody)

	if !strings.Contains(bodyStr, `"chat.completion.chunk"`) {
		t.Errorf("expected body to contain chat.completion.chunk: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "Streamed ") || !strings.Contains(bodyStr, "chunk.") {
		t.Errorf("expected chunks in stream: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "data: [DONE]") {
		t.Errorf("expected data: [DONE] at end of stream: %s", bodyStr)
	}
}

func TestGatewayValidationErrors(t *testing.T) {
	gw := NewGateway()

	// 1. Missing model
	req1 := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(`{"messages": [{"role": "user", "content": "hi"}]}`))
	w1 := httptest.NewRecorder()
	gw.HandleChatCompletions(w1, req1)
	if w1.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing model, got %d", w1.Code)
	}

	// 2. Empty messages
	req2 := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(`{"model": "gemini-2.5-pro", "messages": []}`))
	w2 := httptest.NewRecorder()
	gw.HandleChatCompletions(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty messages, got %d", w2.Code)
	}

	// 3. Invalid JSON
	req3 := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(`{invalid json`))
	w3 := httptest.NewRecorder()
	gw.HandleChatCompletions(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w3.Code)
	}

	// 4. Method not allowed
	req4 := httptest.NewRequest("GET", "/v1/chat/completions", nil)
	w4 := httptest.NewRecorder()
	gw.HandleChatCompletions(w4, req4)
	if w4.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET on completions, got %d", w4.Code)
	}
}

func TestGatewayAutoFailoverNonStreaming(t *testing.T) {
	// Mock upstream: if token is "token-primary", return 429 Too Many Requests
	// if token is "token-secondary", return 200 OK
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "Bearer token-primary" {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error": "Resource has been exhausted"}`))
			return
		}
		if auth == "Bearer token-secondary" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"response":{"candidates":[{"content":{"parts":[{"text":"Success from secondary!"}]}}]}}`)
			_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	client := NewClient(
		WithEndpoints([]string{upstream.URL}),
		WithHTTPClient(upstream.Client()),
	)

	router := NewRouter(WithRouterStrategy(StrategyPriority))
	router.SyncProfiles([]string{"primary", "secondary"})

	gw := NewGateway(
		WithGatewayClient(client),
		WithGatewayRouter(router),
		WithTokenResolver(func(r *http.Request, profileName string) (string, error) {
			if profileName == "primary" {
				return "token-primary", nil
			}
			return "token-secondary", nil
		}),
	)

	body := `{"model": "gemini-2.5-pro", "messages": [{"role": "user", "content": "hi"}], "stream": false}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	gw.HandleChatCompletions(w, req)
	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 OK after failover, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Verify transparency headers
	if resp.Header.Get("X-Profile-Used") != "secondary" {
		t.Errorf("expected X-Profile-Used to be 'secondary', got %q", resp.Header.Get("X-Profile-Used"))
	}
	if resp.Header.Get("X-Failover-Count") != "1" {
		t.Errorf("expected X-Failover-Count to be '1', got %q", resp.Header.Get("X-Failover-Count"))
	}

	var compResp ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&compResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if compResp.Choices[0].Message.Content != "Success from secondary!" {
		t.Errorf("unexpected content: %s", compResp.Choices[0].Message.Content)
	}
}

func TestGatewayAutoFailoverStreaming(t *testing.T) {
	// Mock upstream: if token is "token-primary", return 429
	// if token is "token-secondary", stream SSE
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "Bearer token-primary" {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error": "Quota exceeded for model"}`))
			return
		}
		if auth == "Bearer token-secondary" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"response":{"candidates":[{"content":{"parts":[{"text":"Streamed from secondary"}]}}]}}`)
			_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	client := NewClient(
		WithEndpoints([]string{upstream.URL}),
		WithHTTPClient(upstream.Client()),
	)

	router := NewRouter(WithRouterStrategy(StrategyPriority))
	router.SyncProfiles([]string{"primary", "secondary"})

	gw := NewGateway(
		WithGatewayClient(client),
		WithGatewayRouter(router),
		WithTokenResolver(func(r *http.Request, profileName string) (string, error) {
			if profileName == "primary" {
				return "token-primary", nil
			}
			return "token-secondary", nil
		}),
	)

	body := `{"model": "gemini-2.5-pro", "messages": [{"role": "user", "content": "stream"}], "stream": true}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	gw.HandleChatCompletions(w, req)
	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK after failover, got %d", resp.StatusCode)
	}

	if resp.Header.Get("X-Profile-Used") != "secondary" {
		t.Errorf("expected X-Profile-Used 'secondary', got %q", resp.Header.Get("X-Profile-Used"))
	}
	if resp.Header.Get("X-Failover-Count") != "1" {
		t.Errorf("expected X-Failover-Count '1', got %q", resp.Header.Get("X-Failover-Count"))
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	if !strings.Contains(bodyStr, "Streamed from secondary") {
		t.Errorf("expected stream chunk from secondary, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "data: [DONE]") {
		t.Errorf("expected [DONE] in stream, got: %s", bodyStr)
	}
}

func TestGatewayAllProfilesExhausted(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": "Rate limit exceeded"}`))
	}))
	defer upstream.Close()

	client := NewClient(
		WithEndpoints([]string{upstream.URL}),
		WithHTTPClient(upstream.Client()),
	)

	router := NewRouter(WithRouterStrategy(StrategyPriority))
	router.SyncProfiles([]string{"prof1", "prof2"})

	gw := NewGateway(
		WithGatewayClient(client),
		WithGatewayRouter(router),
	)

	body := `{"model": "gemini-2.5-pro", "messages": [{"role": "user", "content": "hi"}], "stream": false}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()

	gw.HandleChatCompletions(w, req)
	resp := w.Result()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 Too Many Requests when all exhausted, got %d", resp.StatusCode)
	}
}

func TestGatewayRouterManagementEndpoints(t *testing.T) {
	router := NewRouter(WithRouterStrategy(StrategySmart))
	router.SyncProfiles([]string{"dev-1", "dev-2"})

	gw := NewGateway(WithGatewayRouter(router))

	// 1. GET /v1/router/status
	reqStatus := httptest.NewRequest("GET", "/v1/router/status", nil)
	wStatus := httptest.NewRecorder()
	gw.HandleRouterStatus(wStatus, reqStatus)

	if wStatus.Code != http.StatusOK {
		t.Fatalf("expected 200 for status, got %d", wStatus.Code)
	}

	var status RouterStatus
	if err := json.NewDecoder(wStatus.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode router status: %v", err)
	}
	if status.TotalProfiles != 2 || status.HealthyProfiles != 2 {
		t.Errorf("unexpected status: %+v", status)
	}

	// 2. POST /v1/router/strategy
	stratPayload := `{"strategy": "round-robin"}`
	reqStrat := httptest.NewRequest("POST", "/v1/router/strategy", strings.NewReader(stratPayload))
	wStrat := httptest.NewRecorder()
	gw.HandleRouterStrategy(wStrat, reqStrat)

	if wStrat.Code != http.StatusOK {
		t.Fatalf("expected 200 for strategy update, got %d", wStrat.Code)
	}
	if router.GetStrategy() != StrategyRoundRobin {
		t.Errorf("expected strategy 'round-robin', got %q", router.GetStrategy())
	}

	// 3. POST /v1/router/reset
	router.MarkRateLimited("dev-1", 429, "rate limited", 5*time.Minute)
	if router.GetStatus().CooldownProfiles != 1 {
		t.Fatalf("expected 1 cooldown profile")
	}

	reqReset := httptest.NewRequest("POST", "/v1/router/reset", nil)
	wReset := httptest.NewRecorder()
	gw.HandleRouterReset(wReset, reqReset)

	if wReset.Code != http.StatusOK {
		t.Fatalf("expected 200 for reset, got %d", wReset.Code)
	}
	if router.GetStatus().CooldownProfiles != 0 {
		t.Errorf("expected 0 cooldown profiles after reset, got %d", router.GetStatus().CooldownProfiles)
	}
}
