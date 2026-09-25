package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractAnthropicSystem(t *testing.T) {
	// String format
	s1 := ExtractAnthropicSystem("You are Claude.")
	if s1 != "You are Claude." {
		t.Errorf("expected 'You are Claude.', got %q", s1)
	}

	// Content blocks format
	blocks := []AnthropicContentBlock{
		{Type: "text", Text: "Instruction 1"},
		{Type: "text", Text: "Instruction 2"},
	}
	s2 := ExtractAnthropicSystem(blocks)
	expected2 := "Instruction 1\n\nInstruction 2"
	if s2 != expected2 {
		t.Errorf("expected %q, got %q", expected2, s2)
	}

	// Nil format
	if s3 := ExtractAnthropicSystem(nil); s3 != "" {
		t.Errorf("expected empty string for nil, got %q", s3)
	}
}

func TestCollapseAnthropicMessages(t *testing.T) {
	msgs := []AnthropicMessage{
		{
			Role:    "user",
			Content: "Hello!",
		},
		{
			Role:    "assistant",
			Content: "Hi there!",
		},
		{
			Role: "user",
			Content: []AnthropicContentBlock{
				{
					Type: "text",
					Text: "Analyze this image:",
				},
				{
					Type: "image",
					Source: &AnthropicImageSource{
						Type:      "base64",
						MediaType: "image/png",
						Data:      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
					},
				},
			},
		},
	}

	contents, sysPrompt := CollapseAnthropicMessages(msgs, "System instructions here.")
	if sysPrompt != "System instructions here." {
		t.Errorf("unexpected sysPrompt: %q", sysPrompt)
	}

	if len(contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(contents))
	}

	if contents[0].Role != "user" || len(contents[0].Parts) != 1 || contents[0].Parts[0].Text != "Hello!" {
		t.Errorf("unexpected content 0: %+v", contents[0])
	}

	if contents[1].Role != "model" || len(contents[1].Parts) != 1 || contents[1].Parts[0].Text != "Hi there!" {
		t.Errorf("unexpected content 1 (should map assistant to model): %+v", contents[1])
	}

	if contents[2].Role != "user" || len(contents[2].Parts) != 2 {
		t.Fatalf("unexpected content 2: %+v", contents[2])
	}
	if contents[2].Parts[0].Text != "Analyze this image:" {
		t.Errorf("expected text part in content 2, got %+v", contents[2].Parts[0])
	}
	if contents[2].Parts[1].InlineData == nil || contents[2].Parts[1].InlineData.MimeType != "image/png" {
		t.Errorf("expected inline base64 image in content 2, got %+v", contents[2].Parts[1])
	}
}

func TestGatewayMessagesNonStreaming(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-api-key-123" {
			t.Errorf("expected Authorization: Bearer test-api-key-123, got %q", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		_, _ = fmt.Fprint(w, "data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Anthropic response here!\"}]}}]}}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	client := NewClient(
		WithEndpoints([]string{upstream.URL}),
		WithHTTPClient(upstream.Client()),
	)
	router := NewRouter()
	router.SyncProfiles([]string{"default"})

	gw := NewGateway(
		WithGatewayClient(client),
		WithGatewayRouter(router),
	)

	reqPayload := AnthropicMessageRequest{
		Model: "claude-3-7-sonnet-20250219",
		Messages: []AnthropicMessage{
			{Role: "user", Content: "Hello Claude!"},
		},
		MaxTokens: 1024,
	}
	body, _ := json.Marshal(reqPayload)

	httpReq := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	httpReq.Header.Set("x-api-key", "test-api-key-123")
	w := httptest.NewRecorder()

	gw.HandleMessages(w, httpReq)
	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(b))
	}

	if resp.Header.Get("X-Profile-Used") != "default" {
		t.Errorf("expected X-Profile-Used 'default', got %q", resp.Header.Get("X-Profile-Used"))
	}

	var msgResp AnthropicMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
		t.Fatalf("failed to decode Anthropic response: %v", err)
	}

	if msgResp.Type != "message" || msgResp.Role != "assistant" {
		t.Errorf("unexpected type or role: %+v", msgResp)
	}
	if !strings.HasPrefix(msgResp.ID, "msg_") {
		t.Errorf("expected ID with prefix 'msg_', got %q", msgResp.ID)
	}
	if len(msgResp.Content) != 1 || msgResp.Content[0].Text != "Anthropic response here!" {
		t.Errorf("unexpected content: %+v", msgResp.Content)
	}
	if msgResp.StopReason != "end_turn" {
		t.Errorf("expected stop_reason 'end_turn', got %q", msgResp.StopReason)
	}
}

func TestGatewayMessagesStreaming(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		_, _ = fmt.Fprint(w, "data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello \"}]}}]}}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"from stream!\"}]}}]}}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	client := NewClient(
		WithEndpoints([]string{upstream.URL}),
		WithHTTPClient(upstream.Client()),
	)
	router := NewRouter()
	router.SyncProfiles([]string{"stream-prof"})

	gw := NewGateway(
		WithGatewayClient(client),
		WithGatewayRouter(router),
	)

	reqPayload := AnthropicMessageRequest{
		Model: "claude-3-5-sonnet",
		Messages: []AnthropicMessage{
			{Role: "user", Content: "Stream to me"},
		},
		Stream:    true,
		MaxTokens: 2048,
	}
	body, _ := json.Marshal(reqPayload)

	httpReq := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer test-stream-token")
	w := httptest.NewRecorder()

	gw.HandleMessages(w, httpReq)
	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %q", resp.Header.Get("Content-Type"))
	}

	rawBody, _ := io.ReadAll(resp.Body)
	bodyStr := string(rawBody)

	// Verify required Anthropic event sequence
	expectedEvents := []string{
		"event: message_start",
		"\"type\":\"message_start\"",
		"event: content_block_start",
		"\"type\":\"content_block_start\"",
		"event: content_block_delta",
		"\"text\":\"Hello \"",
		"\"text\":\"from stream!\"",
		"event: content_block_stop",
		"\"type\":\"content_block_stop\"",
		"event: message_delta",
		"\"type\":\"message_delta\"",
		"event: message_stop",
		"\"type\":\"message_stop\"",
	}

	for _, evt := range expectedEvents {
		if !strings.Contains(bodyStr, evt) {
			t.Errorf("missing expected event pattern in stream: %q\nFull output:\n%s", evt, bodyStr)
		}
	}
}

func TestGatewayMessagesAutoFailover(t *testing.T) {
	profileAttempts := make(map[string]int)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "Bearer token-p1" {
			profileAttempts["p1"]++
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":429,"message":"Quota exceeded on p1"}}`))
			return
		}

		profileAttempts["p2"]++
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Success after failover!\"}]}}]}}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	client := NewClient(
		WithEndpoints([]string{upstream.URL}),
		WithHTTPClient(upstream.Client()),
	)
	router := NewRouter(WithRouterStrategy(StrategyPriority))
	router.SyncProfiles([]string{"p1", "p2"})

	gw := NewGateway(
		WithGatewayClient(client),
		WithGatewayRouter(router),
		WithTokenResolver(func(r *http.Request, profileName string) (string, error) {
			return "token-" + profileName, nil
		}),
	)

	reqPayload := AnthropicMessageRequest{
		Model: "claude-3-7-sonnet",
		Messages: []AnthropicMessage{
			{Role: "user", Content: "Failover test"},
		},
		MaxTokens: 512,
	}
	body, _ := json.Marshal(reqPayload)

	httpReq := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	w := httptest.NewRecorder()

	gw.HandleMessages(w, httpReq)
	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(b))
	}

	if resp.Header.Get("X-Profile-Used") != "p2" {
		t.Errorf("expected X-Profile-Used 'p2', got %q", resp.Header.Get("X-Profile-Used"))
	}
	if resp.Header.Get("X-Failover-Count") != "1" {
		t.Errorf("expected X-Failover-Count '1', got %q", resp.Header.Get("X-Failover-Count"))
	}

	if profileAttempts["p1"] != 1 || profileAttempts["p2"] != 1 {
		t.Errorf("unexpected profile attempt counts: %+v", profileAttempts)
	}
}

func TestGatewayMessagesValidationErrors(t *testing.T) {
	gw := NewGateway()

	// 1. Invalid method
	req1 := httptest.NewRequest("GET", "/v1/messages", nil)
	w1 := httptest.NewRecorder()
	gw.HandleMessages(w1, req1)
	if w1.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", w1.Code)
	}

	// 2. Empty messages
	reqPayload := AnthropicMessageRequest{
		Model:    "claude-3-5-sonnet",
		Messages: []AnthropicMessage{},
	}
	b2, _ := json.Marshal(reqPayload)
	req2 := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(b2))
	w2 := httptest.NewRecorder()
	gw.HandleMessages(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on empty messages, got %d", w2.Code)
	}
	var errResp AnthropicErrorResponse
	if err := json.NewDecoder(w2.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode Anthropic error response: %v", err)
	}
	if errResp.Type != "error" || errResp.Error.Type != "invalid_request_error" {
		t.Errorf("unexpected error format: %+v", errResp)
	}
}
