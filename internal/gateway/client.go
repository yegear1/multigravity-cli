package gateway

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

var DefaultCloudCodeEndpoints = []string{
	"https://cloudcode-pa.googleapis.com",
	"https://daily-cloudcode-pa.googleapis.com",
}

// Client interacts with the Google CloudCode PA streamGenerateContent API
type Client struct {
	httpClient *http.Client
	endpoints  []string
	userAgent  string
	platform   string
}

// ClientOption defines functional options for configuring Client
type ClientOption func(*Client)

func WithHTTPClient(c *http.Client) ClientOption {
	return func(cl *Client) {
		if c != nil {
			cl.httpClient = c
		}
	}
}

func WithEndpoints(endpoints []string) ClientOption {
	return func(cl *Client) {
		if len(endpoints) > 0 {
			cl.endpoints = endpoints
		}
	}
}

func WithUserAgent(ua string) ClientOption {
	return func(cl *Client) {
		cl.userAgent = ua
	}
}

func generateUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// NewClient initializes a new CloudCode PA client
func NewClient(opts ...ClientOption) *Client {
	plat := "LINUX"
	switch runtime.GOOS {
	case "windows":
		plat = "WINDOWS"
	case "darwin":
		plat = "MACOS"
	}

	c := &Client{
		httpClient: &http.Client{Timeout: 120 * time.Second},
		endpoints:  DefaultCloudCodeEndpoints,
		userAgent:  fmt.Sprintf("antigravity/1.11.3 %s/%s", runtime.GOOS, runtime.GOARCH),
		platform:   plat,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// BuildRequestHeaders constructs standard headers required by CloudCode upstream.
// CRITICAL INVARIANT: NEVER set the x-goog-user-project header.
func (c *Client) BuildRequestHeaders(token string) http.Header {
	h := make(http.Header)
	if token != "" {
		if !strings.HasPrefix(token, "Bearer ") {
			h.Set("Authorization", "Bearer "+token)
		} else {
			h.Set("Authorization", token)
		}
	}
	h.Set("Content-Type", "application/json")
	h.Set("User-Agent", c.userAgent)
	h.Set("X-Goog-Api-Client", "google-cloud-sdk vscode_cloudshelleditor/0.1")
	h.Set("requestId", generateUUID())
	h.Set("requestType", "agent")

	clientMeta := map[string]string{
		"ideType":    "ANTIGRAVITY",
		"platform":   c.platform,
		"pluginType": "GEMINI",
	}
	if metaJSON, err := json.Marshal(clientMeta); err == nil {
		h.Set("Client-Metadata", string(metaJSON))
	}

	return h
}

// GoogleSSEChunk maps the JSON structure returned inside Google CloudCode SSE lines
type GoogleSSEChunk struct {
	Response *struct {
		Candidates []struct {
			Content *struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	} `json:"response"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// ParseGoogleSSELine extracts text deltas from a raw SSE line received from Google.
// Returns (delta, finishReason, isDone).
func ParseGoogleSSELine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "data:") {
		return "", "", false
	}

	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if payload == "" {
		return "", "", false
	}
	if payload == "[DONE]" {
		return "", "stop", true
	}

	var chunk GoogleSSEChunk
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return "", "", false
	}

	if chunk.Response != nil && len(chunk.Response.Candidates) > 0 {
		cand := chunk.Response.Candidates[0]
		var delta string
		if cand.Content != nil && len(cand.Content.Parts) > 0 {
			var sb strings.Builder
			for _, part := range cand.Content.Parts {
				sb.WriteString(part.Text)
			}
			delta = sb.String()
		}
		return delta, cand.FinishReason, false
	}

	return "", "", false
}

// StreamGenerateContent sends the request to CloudCode PA upstream and streams deltas to onChunk.
func (c *Client) StreamGenerateContent(
	ctx context.Context,
	reqPayload *CloudCodeRequest,
	token string,
	onChunk func(delta string, finishReason string) error,
) error {
	return c.StreamGenerateContentWithConnect(ctx, reqPayload, token, nil, onChunk)
}

// StreamGenerateContentWithConnect sends the request to CloudCode PA upstream, triggers onConnect on HTTP 200, and streams deltas to onChunk.
func (c *Client) StreamGenerateContentWithConnect(
	ctx context.Context,
	reqPayload *CloudCodeRequest,
	token string,
	onConnect func() error,
	onChunk func(delta string, finishReason string) error,
) error {
	reqBody, err := json.Marshal(reqPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal cloudcode request: %w", err)
	}

	var lastErr error
	for _, endpoint := range c.endpoints {
		url := fmt.Sprintf("%s/v1internal:streamGenerateContent?alt=sse", strings.TrimRight(endpoint, "/"))
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
		if err != nil {
			lastErr = err
			continue
		}

		httpReq.Header = c.BuildRequestHeaders(token)

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			upstreamErr := &UpstreamHTTPError{
				StatusCode: resp.StatusCode,
				Message:    string(bodyBytes),
				Endpoint:   endpoint,
			}
			lastErr = upstreamErr
			// If 429 or 403, we can try the next endpoint if configured
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden {
				continue
			}
			return upstreamErr
		}

		if onConnect != nil {
			if err := onConnect(); err != nil {
				_ = resp.Body.Close()
				return err
			}
		}

		// Read SSE stream
		reader := bufio.NewReader(resp.Body)
		for {
			line, rErr := reader.ReadString('\n')
			if len(line) > 0 {
				delta, finishReason, isDone := ParseGoogleSSELine(line)
				if isDone {
					_ = resp.Body.Close()
					return nil
				}
				if delta != "" || finishReason != "" {
					if onChunk != nil {
						if err := onChunk(delta, finishReason); err != nil {
							_ = resp.Body.Close()
							return err
						}
					}
				}
			}
			if rErr != nil {
				_ = resp.Body.Close()
				if rErr == io.EOF {
					return nil
				}
				return rErr
			}
		}
	}

	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no upstream endpoints available")
}

// UpstreamHTTPError captures HTTP error responses from Google CloudCode upstream
type UpstreamHTTPError struct {
	StatusCode int
	Message    string
	Endpoint   string
}

func (e *UpstreamHTTPError) Error() string {
	return fmt.Sprintf("upstream %s returned HTTP %d: %s", e.Endpoint, e.StatusCode, e.Message)
}

// IsRateLimitOrQuotaExhausted tests if an error represents HTTP 429, 403 or quota limits
func IsRateLimitOrQuotaExhausted(err error) (bool, int) {
	if err == nil {
		return false, 0
	}
	var upstreamErr *UpstreamHTTPError
	if errors.As(err, &upstreamErr) {
		if upstreamErr.StatusCode == http.StatusTooManyRequests || upstreamErr.StatusCode == http.StatusForbidden {
			return true, upstreamErr.StatusCode
		}
	}
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "too many requests") || strings.Contains(errStr, "rate limit") {
		return true, http.StatusTooManyRequests
	}
	if strings.Contains(errStr, "403") || strings.Contains(errStr, "resource exhausted") || strings.Contains(errStr, "quota") {
		return true, http.StatusForbidden
	}
	return false, 0
}

