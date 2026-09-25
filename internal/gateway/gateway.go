package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TokenResolverFunc resolves an access token dynamically from the request or profile context
type TokenResolverFunc func(r *http.Request, profileName string) (string, error)

// Gateway handles OpenAI-compatible endpoints
type Gateway struct {
	client        *Client
	tokenResolver TokenResolverFunc
}

// GatewayOption defines functional options for configuring the Gateway
type GatewayOption func(*Gateway)

func WithGatewayClient(c *Client) GatewayOption {
	return func(g *Gateway) {
		if c != nil {
			g.client = c
		}
	}
}

func WithTokenResolver(fn TokenResolverFunc) GatewayOption {
	return func(g *Gateway) {
		g.tokenResolver = fn
	}
}

// NewGateway creates a new OpenAI-compatible Gateway instance
func NewGateway(opts ...GatewayOption) *Gateway {
	g := &Gateway{
		client: NewClient(),
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

func (g *Gateway) writeError(w http.ResponseWriter, status int, msg, errType, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(OpenAIErrorResponse{
		Error: OpenAIErrorDetail{
			Message: msg,
			Type:    errType,
			Code:    code,
		},
	})
}

// HandleModels responds with the supported model list
func (g *Gateway) HandleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		g.writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "invalid_request_error", "method_not_allowed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ListSupportedModels())
}

// HandleChatCompletions handles both streaming and non-streaming OpenAI chat completions
func (g *Gateway) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		g.writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "invalid_request_error", "method_not_allowed")
		return
	}

	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to decode JSON body: %v", err), "invalid_request_error", "invalid_json")
		return
	}

	if req.Model == "" {
		g.writeError(w, http.StatusBadRequest, "The 'model' field is required", "invalid_request_error", "missing_model")
		return
	}

	if len(req.Messages) == 0 {
		g.writeError(w, http.StatusBadRequest, "The 'messages' field is required and cannot be empty", "invalid_request_error", "missing_messages")
		return
	}

	modelId := NormalizeModel(req.Model)
	contents, systemPrompt := CollapseOpenAIMessages(req.Messages)

	cloudReq := &CloudCodeRequest{
		Model: modelId,
		Request: CloudCodeRequestBody{
			Model:    modelId,
			Contents: contents,
		},
	}

	if systemPrompt != "" {
		cloudReq.Request.SystemInstruction = &CloudCodeSystemInstruction{
			Parts: []CloudCodePart{{Text: systemPrompt}},
		}
	}

	if req.MaxTokens > 0 || req.Temperature > 0 {
		cloudReq.Request.GenerationConfig = &CloudCodeGenerationConfig{
			MaxOutputTokens: req.MaxTokens,
			Temperature:     req.Temperature,
		}
	}

	// Resolve access token
	token := ""
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	}

	// Determine profile context (header, query, or body)
	profName := req.Profile
	if profName == "" {
		profName = r.Header.Get("X-Profile")
	}
	if profName == "" {
		profName = r.URL.Query().Get("profile")
	}

	if token == "" && g.tokenResolver != nil {
		resolved, err := g.tokenResolver(r, profName)
		if err == nil && resolved != "" {
			token = resolved
		}
	}

	completionID := "chatcmpl-" + generateUUID()
	created := time.Now().Unix()

	// ── Non-streaming mode ──────────────────────────────────────────────────────
	if !req.Stream {
		var contentBuilder strings.Builder
		err := g.client.StreamGenerateContent(r.Context(), cloudReq, token, func(delta, finishReason string) error {
			contentBuilder.WriteString(delta)
			return nil
		})
		if err != nil {
			g.writeError(w, http.StatusBadGateway, fmt.Sprintf("Upstream gateway error: %v", err), "api_error", "upstream_error")
			return
		}

		resp := ChatCompletionResponse{
			ID:      completionID,
			Object:  "chat.completion",
			Created: created,
			Model:   req.Model,
			Choices: []ChatCompletionChoice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: contentBuilder.String(),
					},
					FinishReason: "stop",
				},
			},
			Usage: UsageInfo{
				PromptTokens:     0,
				CompletionTokens: 0,
				TotalTokens:      0,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// ── Streaming mode (SSE) ────────────────────────────────────────────────────
	flusher, ok := w.(http.Flusher)
	if !ok {
		g.writeError(w, http.StatusInternalServerError, "Streaming is not supported by the response writer", "api_error", "streaming_unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	err := g.client.StreamGenerateContent(r.Context(), cloudReq, token, func(delta, finishReason string) error {
		if delta == "" {
			return nil
		}
		chunk := ChatCompletionChunk{
			ID:      completionID,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   req.Model,
			Choices: []ChatCompletionChunkChoice{
				{
					Index: 0,
					Delta: ChatMessageDelta{
						Content: delta,
					},
					FinishReason: nil,
				},
			},
		}
		data, _ := json.Marshal(chunk)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return nil
	})

	if err != nil {
		errResp := OpenAIErrorResponse{
			Error: OpenAIErrorDetail{
				Message: fmt.Sprintf("Stream interrupted: %v", err),
				Type:    "api_error",
				Code:    "upstream_stream_error",
			},
		}
		data, _ := json.Marshal(errResp)
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
		flusher.Flush()
		return
	}

	// Final stop chunk
	stop := "stop"
	finalChunk := ChatCompletionChunk{
		ID:      completionID,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   req.Model,
		Choices: []ChatCompletionChunkChoice{
			{
				Index:        0,
				Delta:        ChatMessageDelta{},
				FinishReason: &stop,
			},
		},
	}
	finalData, _ := json.Marshal(finalChunk)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", finalData)
	_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}
