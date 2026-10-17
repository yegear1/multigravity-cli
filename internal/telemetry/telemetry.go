package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

// Event models a canonical VictoriaLogs / Vector event.
type Event struct {
	Timestamp        string  `json:"timestamp"`
	Level            string  `json:"level"`
	Service          string  `json:"service"`
	App              string  `json:"app"`
	Env              string  `json:"env"`
	Message          string  `json:"message"`
	TraceID          string  `json:"trace_id,omitempty"`
	TaskID           string  `json:"task_id,omitempty"`
	Profile          string  `json:"profile,omitempty"`
	Model            string  `json:"model,omitempty"`
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	TotalTokens      int     `json:"total_tokens,omitempty"`
	TokenDelta       int     `json:"token_delta,omitempty"`
	Source           string  `json:"source,omitempty"`
	DurationMS       float64 `json:"duration_ms,omitempty"`
	HTTPStatus       int     `json:"http_status,omitempty"`
	Status           string  `json:"status,omitempty"`
}

var (
	sinkMu          sync.RWMutex
	customVectorURL string
	customWriter    io.Writer
	customFileDir   string
	httpClient      = &http.Client{Timeout: 500 * time.Millisecond}
)

// SetVectorURL overrides the Vector HTTP endpoint for testing or explicit configuration.
func SetVectorURL(u string) {
	sinkMu.Lock()
	defer sinkMu.Unlock()
	customVectorURL = u
}

// SetWriter redirects telemetry output to a custom writer (useful for tests).
func SetWriter(w io.Writer) {
	sinkMu.Lock()
	defer sinkMu.Unlock()
	customWriter = w
}

// SetFileDir overrides the directory where telemetry.jsonl is written.
func SetFileDir(d string) {
	sinkMu.Lock()
	defer sinkMu.Unlock()
	customFileDir = d
}

// ResetSink restores default telemetry destination configuration.
func ResetSink() {
	sinkMu.Lock()
	defer sinkMu.Unlock()
	customVectorURL = ""
	customWriter = nil
	customFileDir = ""
}

func getVectorEndpoint() string {
	sinkMu.RLock()
	defer sinkMu.RUnlock()
	if customVectorURL != "" {
		return customVectorURL
	}
	if u := os.Getenv("VECTOR_URL"); u != "" {
		return u
	}
	if host := os.Getenv("VECTOR_HOST"); host != "" {
		port := os.Getenv("VECTOR_HTTP_PORT")
		if port == "" {
			port = "8686"
		}
		return fmt.Sprintf("http://%s:%s/logs", host, port)
	}
	return ""
}

// NewEvent initializes an Event with canonical defaults (service, app, env, timestamp, level).
func NewEvent(level, message, traceID string) Event {
	lvl := strings.ToLower(strings.TrimSpace(level))
	if lvl == "" {
		lvl = "info"
	}
	srv := os.Getenv("SERVICE_NAME")
	if srv == "" {
		srv = "multigravity"
	}
	env := os.Getenv("ENV")
	if env == "" {
		env = os.Getenv("ENVIRONMENT")
	}
	if env == "" {
		env = "production"
	}

	return Event{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     lvl,
		Service:   srv,
		App:       "multigravity",
		Env:       env,
		Message:   message,
		TraceID:   strings.TrimSpace(traceID),
	}
}

// Emit writes the event to configured sinks (local telemetry log, custom writer, and Vector HTTP).
func Emit(ev Event) {
	if ev.Timestamp == "" {
		ev.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if ev.Level == "" {
		ev.Level = "info"
	}
	if ev.Service == "" {
		ev.Service = "multigravity"
	}
	if ev.App == "" {
		ev.App = "multigravity"
	}
	if ev.Env == "" {
		ev.Env = "production"
	}

	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	dataWithNL := append(data, '\n')

	sinkMu.RLock()
	writer := customWriter
	fileDir := customFileDir
	sinkMu.RUnlock()

	// 1. Custom writer (test harness or CLI stderr)
	if writer != nil {
		_, _ = writer.Write(dataWithNL)
	} else if os.Getenv("MULTIGRAVITY_LOG_TELEMETRY") == "1" || os.Getenv("LOG_FORMAT") == "json" {
		_, _ = os.Stderr.Write(dataWithNL)
	}

	// 2. Local telemetry file persistence
	go appendLocalTelemetry(fileDir, dataWithNL)

	// 3. Asynchronous Vector HTTP shipment if configured
	endpoint := getVectorEndpoint()
	if endpoint != "" {
		go sendToVector(endpoint, dataWithNL)
	}
}

// EmitTokenDelta is a convenience helper for logging token consumption and burn deltas.
func EmitTokenDelta(profile, model, source, traceID, taskID string, prompt, completion, total int, durationMS float64) {
	ev := NewEvent("info", "token delta recorded", traceID)
	ev.Profile = profile
	ev.Model = model
	ev.Source = source
	ev.TaskID = taskID
	ev.PromptTokens = prompt
	ev.CompletionTokens = completion
	ev.TotalTokens = total
	ev.TokenDelta = total
	ev.DurationMS = durationMS

	Emit(ev)
}

func appendLocalTelemetry(customDir string, data []byte) {
	dir := customDir
	if dir == "" {
		dir = filepath.Join(config.GetMultigravityHome(), ".multigravity")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	path := filepath.Join(dir, "telemetry.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
}

func sendToVector(endpoint string, data []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}
