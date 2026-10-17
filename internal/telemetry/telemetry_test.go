package telemetry

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewEventDefaults(t *testing.T) {
	ev := NewEvent("info", "test message", "tr-12345")
	if ev.Level != "info" {
		t.Fatalf("expected level info, got %q", ev.Level)
	}
	if ev.Service != "multigravity" {
		t.Fatalf("expected service multigravity, got %q", ev.Service)
	}
	if ev.App != "multigravity" {
		t.Fatalf("expected app multigravity, got %q", ev.App)
	}
	if ev.Message != "test message" {
		t.Fatalf("expected message 'test message', got %q", ev.Message)
	}
	if ev.TraceID != "tr-12345" {
		t.Fatalf("expected trace_id 'tr-12345', got %q", ev.TraceID)
	}
	if ev.Timestamp == "" {
		t.Fatalf("expected timestamp to be populated")
	}
}

func TestEmitToCustomWriter(t *testing.T) {
	defer ResetSink()

	var buf bytes.Buffer
	SetWriter(&buf)

	ev := NewEvent("warn", "resource pressure", "trace-abc")
	ev.Profile = "test-prof"
	ev.TaskID = "task-xyz"
	Emit(ev)

	output := buf.String()
	if !strings.HasSuffix(output, "\n") {
		t.Fatalf("expected NDJSON with newline, got %q", output)
	}

	var parsed Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &parsed); err != nil {
		t.Fatalf("failed to parse emitted JSON: %v", err)
	}
	if parsed.Level != "warn" || parsed.TraceID != "trace-abc" || parsed.Profile != "test-prof" || parsed.TaskID != "task-xyz" {
		t.Fatalf("parsed event mismatch: %+v", parsed)
	}
}

func TestEmitTokenDelta(t *testing.T) {
	defer ResetSink()

	var buf bytes.Buffer
	SetWriter(&buf)

	EmitTokenDelta("prof-alpha", "gemini-3.8-flash-tiered", "gateway", "tr-delta-1", "task-100", 150, 350, 500, 42.5)

	var parsed Event
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &parsed); err != nil {
		t.Fatalf("failed to parse token delta event: %v", err)
	}

	if parsed.Profile != "prof-alpha" || parsed.Model != "gemini-3.8-flash-tiered" || parsed.Source != "gateway" {
		t.Fatalf("unexpected metadata: %+v", parsed)
	}
	if parsed.PromptTokens != 150 || parsed.CompletionTokens != 350 || parsed.TotalTokens != 500 || parsed.TokenDelta != 500 {
		t.Fatalf("unexpected token metrics: %+v", parsed)
	}
	if parsed.DurationMS != 42.5 {
		t.Fatalf("expected duration_ms 42.5, got %v", parsed.DurationMS)
	}
	if parsed.TraceID != "tr-delta-1" || parsed.TaskID != "task-100" {
		t.Fatalf("unexpected trace/task IDs: %+v", parsed)
	}
}

func TestEmitToLocalFileAndVector(t *testing.T) {
	defer ResetSink()

	tmpDir := t.TempDir()
	SetFileDir(tmpDir)

	var receivedData []byte
	var receivedMu sync.Mutex
	done := make(chan struct{}, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedMu.Lock()
		receivedData = body
		receivedMu.Unlock()
		select {
		case done <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	SetVectorURL(server.URL)

	ev := NewEvent("info", "vector test event", "tr-vector-99")
	ev.Profile = "vector-prof"
	Emit(ev)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for Vector HTTP POST")
	}

	receivedMu.Lock()
	defer receivedMu.Unlock()
	if !strings.Contains(string(receivedData), "tr-vector-99") {
		t.Fatalf("vector payload did not contain trace_id: %s", string(receivedData))
	}

	// Verify local file persistence
	time.Sleep(50 * time.Millisecond)
	localPath := filepath.Join(tmpDir, "telemetry.jsonl")
	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("failed to read local telemetry file: %v", err)
	}
	if !strings.Contains(string(data), "tr-vector-99") {
		t.Fatalf("local telemetry file did not contain trace_id: %s", string(data))
	}
}
