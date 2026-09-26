package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// ExtractAnthropicSystem extracts the system prompt string from string, array of blocks, or raw JSON
func ExtractAnthropicSystem(system any) string {
	if system == nil {
		return ""
	}
	switch v := system.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		var sb strings.Builder
		for _, rawItem := range v {
			if mItem, ok := rawItem.(map[string]any); ok {
				if t, ok := mItem["text"].(string); ok && strings.TrimSpace(t) != "" {
					if sb.Len() > 0 {
						sb.WriteString("\n\n")
					}
					sb.WriteString(strings.TrimSpace(t))
				}
			}
		}
		return sb.String()
	case []AnthropicContentBlock:
		var sb strings.Builder
		for _, b := range v {
			if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n\n")
				}
				sb.WriteString(strings.TrimSpace(b.Text))
			}
		}
		return sb.String()
	default:
		// Attempt raw json unmarshal
		if raw, ok := v.(json.RawMessage); ok {
			var str string
			if err := json.Unmarshal(raw, &str); err == nil {
				return strings.TrimSpace(str)
			}
			var blocks []AnthropicContentBlock
			if err := json.Unmarshal(raw, &blocks); err == nil {
				var sb strings.Builder
				for _, b := range blocks {
					if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
						if sb.Len() > 0 {
							sb.WriteString("\n\n")
						}
						sb.WriteString(strings.TrimSpace(b.Text))
					}
				}
				return sb.String()
			}
		}
		return ""
	}
}

// CollapseAnthropicMessages converts Anthropic Messages and system prompt into CloudCode format
func CollapseAnthropicMessages(messages []AnthropicMessage, system any) ([]CloudCodeContent, string) {
	systemPrompt := ExtractAnthropicSystem(system)
	var contents []CloudCodeContent

	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		cloudRole := "user"
		if role == "assistant" || role == "model" {
			cloudRole = "model"
		}

		var parts []CloudCodePart

		switch v := msg.Content.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				parts = append(parts, CloudCodePart{Text: strings.TrimSpace(v)})
			}
		case []any:
			for _, item := range v {
				if mItem, ok := item.(map[string]any); ok {
					pType, _ := mItem["type"].(string)
					switch pType {
					case "text":
						if t, ok := mItem["text"].(string); ok && strings.TrimSpace(t) != "" {
							parts = append(parts, CloudCodePart{Text: strings.TrimSpace(t)})
						}
					case "image":
						if src, ok := mItem["source"].(map[string]any); ok {
							srcType, _ := src["type"].(string)
							mediaType, _ := src["media_type"].(string)
							data, _ := src["data"].(string)
							if srcType == "base64" && mediaType != "" && data != "" {
								parts = append(parts, CloudCodePart{
									InlineData: &CloudCodeInlineData{
										MimeType: mediaType,
										Data:     data,
									},
								})
							}
						}
					}
				}
			}
		case []AnthropicContentBlock:
			for _, b := range v {
				switch b.Type {
				case "text":
					if strings.TrimSpace(b.Text) != "" {
						parts = append(parts, CloudCodePart{Text: strings.TrimSpace(b.Text)})
					}
				case "image":
					if b.Source != nil && b.Source.Type == "base64" && b.Source.MediaType != "" && b.Source.Data != "" {
						parts = append(parts, CloudCodePart{
							InlineData: &CloudCodeInlineData{
								MimeType: b.Source.MediaType,
								Data:     b.Source.Data,
							},
						})
					}
				}
			}
		default:
			if raw, ok := v.(json.RawMessage); ok {
				var str string
				if err := json.Unmarshal(raw, &str); err == nil {
					if strings.TrimSpace(str) != "" {
						parts = append(parts, CloudCodePart{Text: strings.TrimSpace(str)})
					}
				} else {
					var blocks []AnthropicContentBlock
					if err := json.Unmarshal(raw, &blocks); err == nil {
						for _, b := range blocks {
							if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
								parts = append(parts, CloudCodePart{Text: strings.TrimSpace(b.Text)})
							} else if b.Type == "image" && b.Source != nil && b.Source.Type == "base64" {
								parts = append(parts, CloudCodePart{
									InlineData: &CloudCodeInlineData{
										MimeType: b.Source.MediaType,
										Data:     b.Source.Data,
									},
								})
							}
						}
					}
				}
			}
		}

		if len(parts) > 0 {
			contents = append(contents, CloudCodeContent{
				Role:  cloudRole,
				Parts: parts,
			})
		}
	}

	return contents, systemPrompt
}

func (g *Gateway) writeAnthropicError(w http.ResponseWriter, status int, msg, errType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(AnthropicErrorResponse{
		Type: "error",
		Error: AnthropicErrorDetail{
			Type:    errType,
			Message: msg,
		},
	})
}

// HandleMessages processes Anthropic-compatible /v1/messages requests
func (g *Gateway) HandleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		g.writeAnthropicError(w, http.StatusMethodNotAllowed, "Method not allowed", "invalid_request_error")
		return
	}

	var req AnthropicMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeAnthropicError(w, http.StatusBadRequest, fmt.Sprintf("Failed to decode JSON body: %v", err), "invalid_request_error")
		return
	}

	if req.Model == "" {
		req.Model = "claude-sonnet-4-6"
	}

	if len(req.Messages) == 0 {
		g.writeAnthropicError(w, http.StatusBadRequest, "The 'messages' field is required and cannot be empty", "invalid_request_error")
		return
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	modelId := NormalizeModel(req.Model)
	contents, systemPrompt := CollapseAnthropicMessages(req.Messages, req.System)

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

	var temp float64
	if req.Temperature != nil {
		temp = *req.Temperature
	}
	cloudReq.Request.GenerationConfig = &CloudCodeGenerationConfig{
		MaxOutputTokens: maxTokens,
		Temperature:     temp,
	}

	// 1. Resolve token from x-api-key or Authorization header
	headerToken := ""
	if apiKey := r.Header.Get("x-api-key"); apiKey != "" {
		headerToken = apiKey
	} else if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		headerToken = strings.TrimPrefix(authHeader, "Bearer ")
	}

	// 2. Determine profile context
	profName := req.Profile
	if profName == "" {
		profName = r.Header.Get("X-Profile")
	}
	if profName == "" {
		profName = r.URL.Query().Get("profile")
	}

	// 3. Determine routing strategy override
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

	// 4. Auto-failover toggle
	failoverEnabled := g.router.IsFailoverEnabled()
	if req.Failover != nil {
		failoverEnabled = *req.Failover
	} else if foHdr := r.Header.Get("X-Failover"); foHdr != "" {
		failoverEnabled = (foHdr != "false" && foHdr != "0" && foHdr != "no")
	} else if foQry := r.URL.Query().Get("failover"); foQry != "" {
		failoverEnabled = (foQry != "false" && foQry != "0" && foQry != "no")
	}

	msgID := "msg_" + generateUUID()

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
			g.writeAnthropicError(w, http.StatusInternalServerError, "Streaming is not supported by the response writer", "api_error")
			return
		}
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		node, err := g.router.SelectProfile(r.Context(), modelId, profName, excluded, strategyOverride)
		if err != nil {
			if errors.Is(err, ErrAllProfilesCoolingDown) {
				g.writeAnthropicError(w, http.StatusTooManyRequests, "All profiles in pool are currently rate-limited or cooling down", "rate_limit_error")
				return
			}
			if failoverCount > 0 {
				g.writeAnthropicError(w, http.StatusTooManyRequests, fmt.Sprintf("All eligible profiles exhausted after %d failovers: %v", failoverCount, err), "rate_limit_error")
				return
			}
			g.writeAnthropicError(w, http.StatusServiceUnavailable, fmt.Sprintf("Profile selection failed: %v", err), "api_error")
			return
		}

		currentProfile := node.Name

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
			upstreamUsage, callErr := g.client.StreamGenerateContent(r.Context(), cloudReq, activeToken, func(delta, finishReason string) error {
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
					continue
				}

				g.writeAnthropicError(w, http.StatusBadGateway, fmt.Sprintf("Upstream gateway error on profile %q: %v", currentProfile, callErr), "api_error")
				return
			}

			g.router.MarkSuccess(currentProfile)
			usage := recordGatewayUsage(currentProfile, req.Model, upstreamUsage, cloudRequestText(cloudReq), contentBuilder.String())

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Profile-Used", currentProfile)
			w.Header().Set("X-Failover-Count", strconv.Itoa(failoverCount))
			w.Header().Set("X-Remaining-Profiles", strconv.Itoa(g.router.AvailableCount()))
			w.Header().Set("X-Routing-Strategy", string(activeStrategy))

			resp := AnthropicMessageResponse{
				ID:   msgID,
				Type: "message",
				Role: "assistant",
				Content: []AnthropicContentBlock{
					{
						Type: "text",
						Text: contentBuilder.String(),
					},
				},
				Model:        req.Model,
				StopReason:   "end_turn",
				StopSequence: nil,
				Usage: AnthropicUsage{
					InputTokens:  usage.PromptTokens,
					OutputTokens: usage.CompletionTokens,
				},
			}

			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// ── Streaming mode (SSE) with safe connection probe ─────────────────────
		headersWritten := false
		var contentBuilder strings.Builder

		upstreamUsage, callErr := g.client.StreamGenerateContentWithConnect(
			r.Context(),
			cloudReq,
			activeToken,
			func() error {
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

				// 1. Emit message_start
				startEvt := AnthropicMessageStartEvent{
					Type: "message_start",
					Message: AnthropicMessageResponse{
						ID:           msgID,
						Type:         "message",
						Role:         "assistant",
						Content:      []AnthropicContentBlock{},
						Model:        req.Model,
						StopReason:   "",
						StopSequence: nil,
						Usage: AnthropicUsage{
							InputTokens:  0,
							OutputTokens: 0,
						},
					},
				}
				sData, _ := json.Marshal(startEvt)
				_, _ = fmt.Fprintf(w, "event: message_start\ndata: %s\n\n", sData)

				// 2. Emit content_block_start
				cbStartEvt := AnthropicContentBlockStartEvent{
					Type:  "content_block_start",
					Index: 0,
					ContentBlock: AnthropicContentBlock{
						Type: "text",
						Text: "",
					},
				}
				cbData, _ := json.Marshal(cbStartEvt)
				_, _ = fmt.Fprintf(w, "event: content_block_start\ndata: %s\n\n", cbData)

				flusher.Flush()
				return nil
			},
			func(delta, finishReason string) error {
				if delta == "" {
					return nil
				}
				contentBuilder.WriteString(delta)
				deltaEvt := AnthropicContentBlockDeltaEvent{
					Type:  "content_block_delta",
					Index: 0,
					Delta: AnthropicTextDelta{
						Type: "text_delta",
						Text: delta,
					},
				}
				dData, _ := json.Marshal(deltaEvt)
				_, _ = fmt.Fprintf(w, "event: content_block_delta\ndata: %s\n\n", dData)
				flusher.Flush()
				return nil
			},
		)

		if callErr != nil {
			if !headersWritten {
				isRateLimit, statusCode := IsRateLimitOrQuotaExhausted(callErr)
				if isRateLimit && failoverEnabled {
					g.router.MarkRateLimited(currentProfile, statusCode, callErr.Error(), 0)
					g.router.MarkFailover(currentProfile)
					excluded[currentProfile] = true
					failoverCount++
					continue
				}

				g.writeAnthropicError(w, http.StatusBadGateway, fmt.Sprintf("Upstream gateway error on profile %q: %v", currentProfile, callErr), "api_error")
				return
			}

			// Stream was interrupted mid-flight after headers were committed
			errEvt := AnthropicErrorResponse{
				Type: "error",
				Error: AnthropicErrorDetail{
					Type:    "api_error",
					Message: fmt.Sprintf("Stream interrupted: %v", callErr),
				},
			}
			eData, _ := json.Marshal(errEvt)
			_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", eData)
			flusher.Flush()
			return
		}

		// Stream completed successfully
		g.router.MarkSuccess(currentProfile)
		usage := recordGatewayUsage(currentProfile, req.Model, upstreamUsage, cloudRequestText(cloudReq), contentBuilder.String())

		// 3. Emit content_block_stop
		cbStop := AnthropicContentBlockStopEvent{
			Type:  "content_block_stop",
			Index: 0,
		}
		cbStopData, _ := json.Marshal(cbStop)
		_, _ = fmt.Fprintf(w, "event: content_block_stop\ndata: %s\n\n", cbStopData)

		// 4. Emit message_delta
		msgDelta := AnthropicMessageDeltaEvent{
			Type: "message_delta",
			Delta: AnthropicMessageDelta{
				StopReason:   "end_turn",
				StopSequence: nil,
			},
			Usage: AnthropicUsage{
				OutputTokens: usage.CompletionTokens,
			},
		}
		mdData, _ := json.Marshal(msgDelta)
		_, _ = fmt.Fprintf(w, "event: message_delta\ndata: %s\n\n", mdData)

		// 5. Emit message_stop
		msgStop := AnthropicMessageStopEvent{
			Type: "message_stop",
		}
		msData, _ := json.Marshal(msgStop)
		_, _ = fmt.Fprintf(w, "event: message_stop\ndata: %s\n\n", msData)

		flusher.Flush()
		return
	}

	g.writeAnthropicError(w, http.StatusTooManyRequests, fmt.Sprintf("All profiles exhausted after %d attempts", failoverCount), "rate_limit_error")
}
