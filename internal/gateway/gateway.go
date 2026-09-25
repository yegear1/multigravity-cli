package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// TokenResolverFunc resolves an access token dynamically from the request or profile context
type TokenResolverFunc func(r *http.Request, profileName string) (string, error)

// Gateway handles OpenAI-compatible endpoints with multi-account routing and auto-failover
type Gateway struct {
	client        *Client
	tokenResolver TokenResolverFunc
	router        *Router
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

func WithGatewayRouter(r *Router) GatewayOption {
	return func(g *Gateway) {
		if r != nil {
			g.router = r
		}
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
	if g.router == nil {
		g.router = NewRouter(WithRouterTokenResolver(g.tokenResolver))
	} else if g.router.tokenResolver == nil && g.tokenResolver != nil {
		g.router.tokenResolver = g.tokenResolver
	}
	return g
}

// Router returns the active multi-account router instance
func (g *Gateway) Router() *Router {
	return g.router
}

// SetRouter replaces or sets the router instance
func (g *Gateway) SetRouter(r *Router) {
	if r != nil {
		g.router = r
		if g.router.tokenResolver == nil && g.tokenResolver != nil {
			g.router.tokenResolver = g.tokenResolver
		}
	}
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

// HandleRouterStatus returns the pool telemetry and health of all registered profiles
func (g *Gateway) HandleRouterStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		g.writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "invalid_request_error", "method_not_allowed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(g.router.GetStatus())
}

// HandleRouterReset clears all rate-limit cooldowns across all profiles in the pool
func (g *Gateway) HandleRouterReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		g.writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "invalid_request_error", "method_not_allowed")
		return
	}
	g.router.ResetCooldowns()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"message": "all profile cooldowns have been reset",
		"pool":    g.router.GetStatus(),
	})
}

// HandleRouterStrategy updates the active distribution strategy
func (g *Gateway) HandleRouterStrategy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		g.writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "invalid_request_error", "method_not_allowed")
		return
	}
	var req SetStrategyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON payload: %v", err), "invalid_request_error", "invalid_json")
		return
	}
	strategy := ParseRoutingStrategy(req.Strategy)
	g.router.SetStrategy(strategy)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":   "ok",
		"strategy": string(strategy),
		"pool":     g.router.GetStatus(),
	})
}

// HandleChatCompletions handles both streaming and non-streaming OpenAI chat completions with multi-account routing & auto-failover
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

	// 1. Resolve initial base token from Authorization header if present
	headerToken := ""
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		headerToken = strings.TrimPrefix(authHeader, "Bearer ")
	}

	// 2. Determine profile context (header, query, or body)
	profName := req.Profile
	if profName == "" {
		profName = r.Header.Get("X-Profile")
	}
	if profName == "" {
		profName = r.URL.Query().Get("profile")
	}

	// 3. Determine routing strategy override if requested
	stratStr := req.Strategy
	if stratStr == "" {
		stratStr = r.Header.Get("X-Routing-Strategy")
	}
	if stratStr == "" {
		stratStr = r.URL.Query().Get("strategy")
	}
	var strategyOverride RoutingStrategy
	if stratStr != "" {
		strategyOverride = ParseRoutingStrategy(stratStr)
	}

	// 4. Determine auto-failover toggle
	failoverEnabled := g.router.IsFailoverEnabled()
	if req.Failover != nil {
		failoverEnabled = *req.Failover
	} else if foHdr := r.Header.Get("X-Failover"); foHdr != "" {
		failoverEnabled = (foHdr != "false" && foHdr != "0" && foHdr != "no")
	} else if foQry := r.URL.Query().Get("failover"); foQry != "" {
		failoverEnabled = (foQry != "false" && foQry != "0" && foQry != "no")
	}

	completionID := "chatcmpl-" + generateUUID()
	created := time.Now().Unix()

	// 5. Failover loop across candidate profiles
	excluded := make(map[string]bool)
	failoverCount := 0
	maxAttempts := g.router.AvailableCount() + 1
	if maxAttempts < 3 {
		maxAttempts = 3
	}

	// Pre-check flusher if streaming mode requested
	var flusher http.Flusher
	if req.Stream {
		var ok bool
		flusher, ok = w.(http.Flusher)
		if !ok {
			g.writeError(w, http.StatusInternalServerError, "Streaming is not supported by the response writer", "api_error", "streaming_unsupported")
			return
		}
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		node, err := g.router.SelectProfile(r.Context(), modelId, profName, excluded, strategyOverride)
		if err != nil {
			if errors.Is(err, ErrAllProfilesCoolingDown) {
				g.writeError(w, http.StatusTooManyRequests, "All profiles in pool are currently rate-limited or cooling down", "rate_limit_exceeded", "rate_limit_exceeded")
				return
			}
			if failoverCount > 0 {
				g.writeError(w, http.StatusTooManyRequests, fmt.Sprintf("All eligible profiles exhausted after %d failovers: %v", failoverCount, err), "rate_limit_exceeded", "rate_limit_exceeded")
				return
			}
			g.writeError(w, http.StatusServiceUnavailable, fmt.Sprintf("Profile selection failed: %v", err), "api_error", "profile_selection_failed")
			return
		}

		currentProfile := node.Name

		// Resolve token for this selected profile
		activeToken := headerToken
		if g.tokenResolver != nil {
			resolved, rErr := g.tokenResolver(r, currentProfile)
			if rErr == nil && resolved != "" {
				activeToken = resolved
			}
		}

		activeStrategy := strategyOverride
		if activeStrategy == "" {
			activeStrategy = g.router.GetStrategy()
		}

		// ── Non-streaming mode ──────────────────────────────────────────────────
		if !req.Stream {
			var contentBuilder strings.Builder
			callErr := g.client.StreamGenerateContent(r.Context(), cloudReq, activeToken, func(delta, finishReason string) error {
				contentBuilder.WriteString(delta)
				return nil
			})

			if callErr != nil {
				isRateLimit, statusCode := IsRateLimitOrQuotaExhausted(callErr)
				if isRateLimit && failoverEnabled {
					g.router.MarkRateLimited(currentProfile, statusCode, callErr.Error(), 0)
					g.router.MarkFailover(currentProfile)
					excluded[currentProfile] = true
					failoverCount++
					continue // failover to next eligible profile
				}

				g.writeError(w, http.StatusBadGateway, fmt.Sprintf("Upstream gateway error on profile %q: %v", currentProfile, callErr), "api_error", "upstream_error")
				return
			}

			// Success! Record metrics and write response
			g.router.MarkSuccess(currentProfile)

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Profile-Used", currentProfile)
			w.Header().Set("X-Failover-Count", strconv.Itoa(failoverCount))
			w.Header().Set("X-Remaining-Profiles", strconv.Itoa(g.router.AvailableCount()))
			w.Header().Set("X-Routing-Strategy", string(activeStrategy))

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

			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// ── Streaming mode (SSE) with safe connection probe ─────────────────────
		headersWritten := false

		callErr := g.client.StreamGenerateContentWithConnect(
			r.Context(),
			cloudReq,
			activeToken,
			func() error {
				// Invoked only upon successful HTTP 200 connection from upstream
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")
				w.Header().Set("X-Profile-Used", currentProfile)
				w.Header().Set("X-Failover-Count", strconv.Itoa(failoverCount))
				w.Header().Set("X-Remaining-Profiles", strconv.Itoa(g.router.AvailableCount()))
				w.Header().Set("X-Routing-Strategy", string(activeStrategy))
				w.WriteHeader(http.StatusOK)
				flusher.Flush()
				headersWritten = true
				return nil
			},
			func(delta, finishReason string) error {
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
			},
		)

		if callErr != nil {
			if !headersWritten {
				// Connection failed before sending headers to client: auto-failover is safe!
				isRateLimit, statusCode := IsRateLimitOrQuotaExhausted(callErr)
				if isRateLimit && failoverEnabled {
					g.router.MarkRateLimited(currentProfile, statusCode, callErr.Error(), 0)
					g.router.MarkFailover(currentProfile)
					excluded[currentProfile] = true
					failoverCount++
					continue // failover to next profile seamlessly!
				}

				g.writeError(w, http.StatusBadGateway, fmt.Sprintf("Upstream gateway error on profile %q: %v", currentProfile, callErr), "api_error", "upstream_error")
				return
			}

			// Stream was interrupted mid-flight after headers were committed
			errResp := OpenAIErrorResponse{
				Error: OpenAIErrorDetail{
					Message: fmt.Sprintf("Stream interrupted: %v", callErr),
					Type:    "api_error",
					Code:    "upstream_stream_error",
				},
			}
			data, _ := json.Marshal(errResp)
			_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
			flusher.Flush()
			return
		}

		// Stream completed successfully
		g.router.MarkSuccess(currentProfile)

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
		return
	}

	// If all attempts exhausted without successful response
	g.writeError(w, http.StatusTooManyRequests, fmt.Sprintf("All profiles exhausted after %d attempts", failoverCount), "rate_limit_exceeded", "rate_limit_exceeded")
}
