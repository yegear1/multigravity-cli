package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/profile"
)

func setupTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", filepath.Join(tempHome, "shortcuts"))

	srv := NewServer(Config{
		Host:      "127.0.0.1",
		Port:      8989,
		Version:   "2.0.0-test",
		StartTime: time.Now().Add(-10 * time.Second),
	})

	return srv, tempHome
}

func TestHealthEndpoints(t *testing.T) {
	srv, _ := setupTestServer(t)

	for _, path := range []string{"/health", "/api/v1/health"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("[%s] expected status 200, got %d", path, rec.Code)
		}

		var resp APIResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("[%s] failed to unmarshal response: %v", path, err)
		}

		if !resp.Success {
			t.Fatalf("[%s] expected success: true, got false", path)
		}

		dataMap, ok := resp.Data.(map[string]any)
		if !ok {
			t.Fatalf("[%s] expected Data to be map, got %T", path, resp.Data)
		}

		if dataMap["status"] != "ok" {
			t.Errorf("[%s] expected status ok, got %v", path, dataMap["status"])
		}
		if dataMap["version"] != "2.0.0-test" {
			t.Errorf("[%s] expected version 2.0.0-test, got %v", path, dataMap["version"])
		}
	}
}

func TestDoctorEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/doctor", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success: true")
	}
}

func TestProfilesAndStatsEndpoints(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create test profile
	err := profile.CreateProfile(profile.CreateOptions{
		Name:  "srv-test-prof",
		Color: "blue",
	})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	// 1. List profiles
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var listResp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to parse list resp: %v", err)
	}
	if !listResp.Success {
		t.Fatalf("expected listResp success true")
	}

	// 2. Get single profile
	req = httptest.NewRequest(http.MethodGet, "/api/v1/profiles/srv-test-prof", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for single profile, got %d", rec.Code)
	}

	// 3. Get nonexistent profile
	req = httptest.NewRequest(http.MethodGet, "/api/v1/profiles/nonexistent-xyz", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent profile, got %d", rec.Code)
	}

	// 4. Get profile stats
	req = httptest.NewRequest(http.MethodGet, "/api/v1/profiles/srv-test-prof/stats", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for profile stats, got %d", rec.Code)
	}

	// 5. Get aggregate stats
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for stats report, got %d", rec.Code)
	}

	// 6. Sharing status
	req = httptest.NewRequest(http.MethodGet, "/api/v1/profiles/srv-test-prof/sharing", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sharing status, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/profiles/srv-test-prof/sharing/mcp", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for mcp sharing, got %d", rec.Code)
	}

	// 7. Conversations
	req = httptest.NewRequest(http.MethodGet, "/api/v1/profiles/srv-test-prof/conversations", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for conversations, got %d", rec.Code)
	}

	// 8. Clean profile
	req = httptest.NewRequest(http.MethodPost, "/api/v1/profiles/srv-test-prof/clean", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for clean profile, got %d: %s", rec.Code, rec.Body.String())
	}

	// 9. Stop profile (not running)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/profiles/srv-test-prof/stop", strings.NewReader(`{"force": false}`))
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for stop profile, got %d", rec.Code)
	}
}

func TestCORSHeaders(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/profiles", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS preflight, got %d", rec.Code)
	}

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected CORS allow origin *, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestServerStartShutdown(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)

	srv := NewServer(Config{
		Host:      "127.0.0.1",
		Port:      19989, // Pick high random port
		Version:   "2.0.0-test",
		StartTime: time.Now(),
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	// Wait briefly for server to bind
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("failed to shutdown server: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			t.Fatalf("unexpected server start error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("server shutdown timed out")
	}
}
