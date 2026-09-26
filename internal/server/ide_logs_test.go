package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/idelog"
)

func TestIDELogsEndpoint(t *testing.T) {
	srv, home := setupTestServer(t)
	dir := filepath.Join(config.GetUserDataDir(filepath.Join(home, "work")), "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.log"), []byte("window ready --csrf_token secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "language_server.log"), []byte("do-not-stream\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/work/ide/logs?tail=10", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var wrapped struct {
		Success bool            `json:"success"`
		Data    idelog.Snapshot `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &wrapped); err != nil {
		t.Fatal(err)
	}
	if !wrapped.Success || wrapped.Data.Source != idelog.SourceIDE {
		t.Fatalf("payload: %+v", wrapped)
	}
	if strings.Contains(wrapped.Data.Logs, "secret") || strings.Contains(wrapped.Data.Logs, "do-not-stream") {
		t.Fatalf("logs: %q", wrapped.Data.Logs)
	}

	missing := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/absent/ide/logs", nil)
	missRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(missRec, missing)
	if missRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", missRec.Code)
	}
}

func TestIDELogsFollowSSE(t *testing.T) {
	srv, home := setupTestServer(t)
	dir := filepath.Join(config.GetUserDataDir(filepath.Join(home, "work")), "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.log"), []byte("ide up\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/work/ide/logs?follow=true&tail=5", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	cw := &cancelOnWrite{ResponseWriter: rec, cancel: cancel}
	srv.Handler().ServeHTTP(cw, req)

	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content-type %q", rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: log") || !strings.Contains(body, "ide up") {
		t.Fatalf("sse body: %q", body)
	}
}

type cancelOnWrite struct {
	http.ResponseWriter
	cancel func()
	once   sync.Once
}

func (c *cancelOnWrite) Write(p []byte) (int, error) {
	n, err := c.ResponseWriter.Write(p)
	c.once.Do(c.cancel)
	return n, err
}

func (c *cancelOnWrite) Flush() {
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
