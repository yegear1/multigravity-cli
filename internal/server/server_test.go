package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/gateway"
	"github.com/ye-dev/multigravity-cli/internal/prime"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
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

	req = httptest.NewRequest(http.MethodGet, "/api/v1/profiles/srv-test-prof/conversations?filter=active", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for active conversations, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/profiles/srv-test-prof/conversations?filter=in_use", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for in_use conversations, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/profiles/srv-test-prof/conversations?filter=open", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for open conversations, got %d", rec.Code)
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
	allowMethods := rec.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(allowMethods, "DELETE") {
		t.Errorf("expected CORS allow methods to include DELETE, got %q", allowMethods)
	}
	if !strings.Contains(allowMethods, "PUT") {
		t.Errorf("expected CORS allow methods to include PUT, got %q", allowMethods)
	}
	allowHeaders := rec.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(allowHeaders, "X-Routing-Strategy") || !strings.Contains(allowHeaders, "X-Failover") {
		t.Errorf("expected CORS allow headers to include X-Routing-Strategy and X-Failover, got %q", allowHeaders)
	}
	if !strings.Contains(allowHeaders, "x-api-key") || !strings.Contains(allowHeaders, "anthropic-version") {
		t.Errorf("expected CORS allow headers to include x-api-key and anthropic-version, got %q", allowHeaders)
	}
	exposeHeaders := rec.Header().Get("Access-Control-Expose-Headers")
	if !strings.Contains(exposeHeaders, "X-Profile-Used") || !strings.Contains(exposeHeaders, "X-Failover-Count") {
		t.Errorf("expected CORS expose headers to include X-Profile-Used and X-Failover-Count, got %q", exposeHeaders)
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

func TestSSEEvents(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Broker().Stop()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	for _, path := range []string{"/api/v1/events", "/events"} {
		reqCtx, reqCancel := context.WithCancel(context.Background())

		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, ts.URL+path, nil)
		if err != nil {
			reqCancel()
			t.Fatalf("[%s] failed to create request: %v", path, err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			reqCancel()
			t.Fatalf("[%s] failed to connect to SSE endpoint: %v", path, err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			reqCancel()
			t.Fatalf("[%s] expected 200, got %d", path, resp.StatusCode)
		}
		if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
			resp.Body.Close()
			reqCancel()
			t.Fatalf("[%s] expected text/event-stream, got %q", path, resp.Header.Get("Content-Type"))
		}

		reader := bufio.NewReader(resp.Body)

		// Read first event (should be "init")
		line1, err := reader.ReadString('\n')
		if err != nil {
			resp.Body.Close()
			reqCancel()
			t.Fatalf("[%s] failed to read line1: %v", path, err)
		}
		if strings.TrimSpace(line1) != "event: init" {
			resp.Body.Close()
			reqCancel()
			t.Fatalf("[%s] expected 'event: init', got %q", path, line1)
		}

		line2, err := reader.ReadString('\n')
		if err != nil {
			resp.Body.Close()
			reqCancel()
			t.Fatalf("[%s] failed to read line2: %v", path, err)
		}
		if !strings.HasPrefix(line2, "data: ") {
			resp.Body.Close()
			reqCancel()
			t.Fatalf("[%s] expected 'data: ...', got %q", path, line2)
		}

		// Consume empty line separating events
		_, _ = reader.ReadString('\n')

		// Broadcast a test event
		srv.Broker().Broadcast(SSEEvent{
			Event: "test_notice",
			Data: map[string]string{
				"msg": "sse stream active",
			},
		})

		line3, err := reader.ReadString('\n')
		if err != nil {
			resp.Body.Close()
			reqCancel()
			t.Fatalf("[%s] failed to read line3: %v", path, err)
		}
		if strings.TrimSpace(line3) != "event: test_notice" {
			resp.Body.Close()
			reqCancel()
			t.Fatalf("[%s] expected 'event: test_notice', got %q", path, line3)
		}

		line4, err := reader.ReadString('\n')
		if err != nil {
			resp.Body.Close()
			reqCancel()
			t.Fatalf("[%s] failed to read line4: %v", path, err)
		}
		if !strings.Contains(line4, "sse stream active") {
			resp.Body.Close()
			reqCancel()
			t.Fatalf("[%s] expected line4 to contain message, got %q", path, line4)
		}

		resp.Body.Close()
		reqCancel()
	}

	// Wait with timeout for server to unregister all clients
	deadline := time.Now().Add(2 * time.Second)
	for srv.Broker().ClientCount() > 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	if count := srv.Broker().ClientCount(); count != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", count)
	}
}

func TestBrokerUnit(t *testing.T) {
	b := NewBroker()
	defer b.Stop()

	ch := make(chan SSEEvent, 5)
	b.Register(ch)

	if b.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", b.ClientCount())
	}

	b.Broadcast(SSEEvent{
		Event: "custom_type",
		Data:  "custom_val",
	})

	select {
	case ev := <-ch:
		if ev.Event != "custom_type" || ev.Data != "custom_val" {
			t.Fatalf("unexpected event: %+v", ev)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for event")
	}

	b.Unregister(ch)
	if b.ClientCount() != 0 {
		t.Fatalf("expected 0 clients, got %d", b.ClientCount())
	}
}

func TestSSEActionBroadcast(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Broker().Stop()

	// Create test profile
	err := profile.CreateProfile(profile.CreateOptions{
		Name: "sse-action-prof",
	})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	reqCtx, reqCancel := context.WithCancel(context.Background())
	defer reqCancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, ts.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatalf("failed to create req: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect SSE: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Consume "init" event
	_, _ = reader.ReadString('\n') // event: init
	_, _ = reader.ReadString('\n') // data: ...
	_, _ = reader.ReadString('\n') // empty line

	// Call clean via HTTP POST
	cleanReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/profiles/sse-action-prof/clean", nil)
	cleanResp, err := http.DefaultClient.Do(cleanReq)
	if err != nil {
		t.Fatalf("failed to clean: %v", err)
	}
	cleanResp.Body.Close()
	if cleanResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 clean, got %d", cleanResp.StatusCode)
	}

	// Now reader should receive "event: action"
	line1, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read action event line: %v", err)
	}
	if strings.TrimSpace(line1) != "event: action" {
		t.Fatalf("expected 'event: action', got %q", line1)
	}

	line2, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read action data line: %v", err)
	}
	if !strings.Contains(line2, "clean") || !strings.Contains(line2, "sse-action-prof") {
		t.Fatalf("expected data to contain clean and profile name, got %q", line2)
	}
}

func TestCreateProfileEndpoints(t *testing.T) {
	srv, _ := setupTestServer(t)

	// 1. Create auth-only profile via POST /api/profiles
	body := `{"name": "test-new-auth", "auth_only": true, "color": "blue"}`
	req := httptest.NewRequest(http.MethodPost, "/api/profiles", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	if !profile.ProfileExists("test-new-auth") {
		t.Fatalf("expected profile test-new-auth to exist on disk")
	}

	info, err := profile.GetProfile("test-new-auth")
	if err != nil {
		t.Fatalf("failed to get created profile: %v", err)
	}
	if info.Type != "auth-only" {
		t.Errorf("expected profile type auth-only, got %q", info.Type)
	}
	if info.Color != "#1e40af" {
		t.Errorf("expected resolved color hex #1e40af, got %q", info.Color)
	}

	// 2. Create profile via POST /api/v1/profiles with kebab-case auth-only
	bodyV1 := `{"name": "test-v1-kebab", "auth-only": true}`
	reqV1 := httptest.NewRequest(http.MethodPost, "/api/v1/profiles", strings.NewReader(bodyV1))
	recV1 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recV1, reqV1)

	if recV1.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for v1, got %d: %s", recV1.Code, recV1.Body.String())
	}
	if !profile.ProfileExists("test-v1-kebab") {
		t.Fatalf("expected profile test-v1-kebab to exist")
	}

	// 3. Conflict on duplicate profile
	reqDup := httptest.NewRequest(http.MethodPost, "/api/profiles", strings.NewReader(`{"name": "test-new-auth"}`))
	recDup := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recDup, reqDup)

	if recDup.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate profile, got %d", recDup.Code)
	}

	// 4. Bad request on invalid name
	reqInv := httptest.NewRequest(http.MethodPost, "/api/profiles", strings.NewReader(`{"name": "invalid name!"}`))
	recInv := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recInv, reqInv)

	if recInv.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid name, got %d", recInv.Code)
	}

	// 5. Bad request on empty name
	reqEmpty := httptest.NewRequest(http.MethodPost, "/api/profiles", strings.NewReader(`{"name": ""}`))
	recEmpty := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recEmpty, reqEmpty)

	if recEmpty.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for empty name, got %d", recEmpty.Code)
	}

	// 6. Bad request on invalid JSON or nil body
	reqNil := httptest.NewRequest(http.MethodPost, "/api/profiles", nil)
	recNil := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recNil, reqNil)

	if recNil.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for nil body, got %d", recNil.Code)
	}
}

func TestDeleteProfileEndpoints(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create profiles for deletion tests
	if err := profile.CreateProfile(profile.CreateOptions{Name: "del-prof-simple"}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}
	if err := profile.CreateProfile(profile.CreateOptions{Name: "del-prof-running"}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	// 1. DELETE /api/profiles/del-prof-simple (idle)
	req := httptest.NewRequest(http.MethodDelete, "/api/profiles/del-prof-simple", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for delete, got %d: %s", rec.Code, rec.Body.String())
	}
	if profile.ProfileExists("del-prof-simple") {
		t.Errorf("expected del-prof-simple to be deleted")
	}

	// 2. DELETE nonexistent profile -> 404
	reqNon := httptest.NewRequest(http.MethodDelete, "/api/profiles/nonexistent-del", nil)
	recNon := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recNon, reqNon)

	if recNon.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent delete, got %d", recNon.Code)
	}

	// 3. DELETE running profile without force -> 409 Conflict
	restorePIDs := profile.SetGetProfilePIDsFn(func(name string) ([]int, error) {
		if name == "del-prof-running" {
			return []int{99991}, nil
		}
		return nil, nil
	})

	reqRun := httptest.NewRequest(http.MethodDelete, "/api/v1/profiles/del-prof-running", nil)
	recRun := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recRun, reqRun)

	if recRun.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for running profile delete, got %d: %s", recRun.Code, recRun.Body.String())
	}
	if !profile.ProfileExists("del-prof-running") {
		t.Errorf("profile should NOT be deleted when running without force")
	}

	// 4. DELETE running profile with force query param -> 200 OK
	restorePIDs() // Restore to idle before force delete executes StopProfile
	reqForce := httptest.NewRequest(http.MethodDelete, "/api/v1/profiles/del-prof-running?force=true", nil)
	recForce := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recForce, reqForce)

	if recForce.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for force delete, got %d: %s", recForce.Code, recForce.Body.String())
	}
	if profile.ProfileExists("del-prof-running") {
		t.Errorf("expected del-prof-running to be deleted with force")
	}
}

func TestLaunchAndRestartProfileEndpoints(t *testing.T) {
	srv, _ := setupTestServer(t)

	if err := profile.CreateProfile(profile.CreateOptions{Name: "prof-runner"}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	var lastLaunched string
	var lastArgs []string
	restoreLaunch := profile.SetLaunchProfileFn(func(name string, forwardArgs []string) error {
		lastLaunched = name
		lastArgs = forwardArgs
		return nil
	})
	defer restoreLaunch()

	// 1. POST /api/profiles/prof-runner/launch with args
	body := `{"args": ["--reuse-window", "/workspace/multigravity"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/profiles/prof-runner/launch", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for launch, got %d: %s", rec.Code, rec.Body.String())
	}
	if lastLaunched != "prof-runner" {
		t.Errorf("expected launched profile prof-runner, got %q", lastLaunched)
	}
	if len(lastArgs) != 2 || lastArgs[0] != "--reuse-window" {
		t.Errorf("expected forwarded args to match, got %v", lastArgs)
	}

	// 2. POST /api/v1/profiles/prof-runner/launch without body
	lastLaunched = ""
	lastArgs = nil
	reqV1 := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/prof-runner/launch", nil)
	recV1 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recV1, reqV1)

	if recV1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for v1 launch, got %d", recV1.Code)
	}
	if lastLaunched != "prof-runner" {
		t.Errorf("expected launched profile prof-runner, got %q", lastLaunched)
	}

	// 3. POST launch nonexistent profile -> 404
	reqNon := httptest.NewRequest(http.MethodPost, "/api/profiles/nonexistent-run/launch", nil)
	recNon := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recNon, reqNon)

	if recNon.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for launch nonexistent, got %d", recNon.Code)
	}

	// 4. POST /api/profiles/prof-runner/restart
	restorePIDs := profile.SetGetProfilePIDsFn(func(name string) ([]int, error) {
		return nil, nil // idle
	})
	defer restorePIDs()

	lastLaunched = ""
	lastArgs = nil
	reqRestart := httptest.NewRequest(http.MethodPost, "/api/profiles/prof-runner/restart", strings.NewReader(`{"args": ["--wait"]}`))
	recRestart := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recRestart, reqRestart)

	if recRestart.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for restart, got %d: %s", recRestart.Code, recRestart.Body.String())
	}
	if lastLaunched != "prof-runner" {
		t.Errorf("expected restarted profile prof-runner, got %q", lastLaunched)
	}
	if len(lastArgs) != 1 || lastArgs[0] != "--wait" {
		t.Errorf("expected forwarded args to match restart, got %v", lastArgs)
	}

	// 5. POST restart nonexistent profile -> 404
	reqRestartNon := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/nonexistent-restart/restart", nil)
	recRestartNon := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recRestartNon, reqRestartNon)

	if recRestartNon.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for restart nonexistent, got %d", recRestartNon.Code)
	}
}

func TestRenameProfileEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)

	if err := profile.CreateProfile(profile.CreateOptions{Name: "prof-ren-old"}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}
	if err := profile.CreateProfile(profile.CreateOptions{Name: "prof-ren-target-exists"}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	// 1. Successful rename
	body := `{"new_name": "prof-ren-new"}`
	req := httptest.NewRequest(http.MethodPost, "/api/profiles/prof-ren-old/rename", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for rename, got %d: %s", rec.Code, rec.Body.String())
	}
	if profile.ProfileExists("prof-ren-old") {
		t.Errorf("old profile name should no longer exist")
	}
	if !profile.ProfileExists("prof-ren-new") {
		t.Errorf("new profile name should exist")
	}

	// 2. Conflict when new name already exists
	reqConf := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/prof-ren-new/rename", strings.NewReader(`{"new_name": "prof-ren-target-exists"}`))
	recConf := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recConf, reqConf)

	if recConf.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for existing target, got %d: %s", recConf.Code, recConf.Body.String())
	}

	// 3. Bad request on invalid target name
	reqInv := httptest.NewRequest(http.MethodPost, "/api/profiles/prof-ren-new/rename", strings.NewReader(`{"new_name": "invalid name!"}`))
	recInv := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recInv, reqInv)

	if recInv.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid target name, got %d", recInv.Code)
	}

	// 4. Nonexistent source profile
	reqNon := httptest.NewRequest(http.MethodPost, "/api/profiles/nonexistent-src/rename", strings.NewReader(`{"new_name": "valid-name"}`))
	recNon := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recNon, reqNon)

	if recNon.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent source rename, got %d", recNon.Code)
	}

	// 5. Running profile rename blocked -> 409 Conflict
	restorePIDs := profile.SetGetProfilePIDsFn(func(name string) ([]int, error) {
		if name == "prof-ren-target-exists" {
			return []int{88888}, nil
		}
		return nil, nil
	})
	defer restorePIDs()

	reqRun := httptest.NewRequest(http.MethodPost, "/api/profiles/prof-ren-target-exists/rename", strings.NewReader(`{"new_name": "prof-ren-target-moved"}`))
	recRun := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recRun, reqRun)

	if recRun.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for running profile rename, got %d", recRun.Code)
	}
}

func TestSSEActionBroadcastMutations(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Broker().Stop()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	reqCtx, reqCancel := context.WithCancel(context.Background())
	defer reqCancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, ts.URL+"/events", nil)
	if err != nil {
		t.Fatalf("failed to create req: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect SSE: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Consume "init" event
	_, _ = reader.ReadString('\n') // event: init
	_, _ = reader.ReadString('\n') // data: ...
	_, _ = reader.ReadString('\n') // empty line

	// 1. Test Create SSE action
	createReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/profiles", strings.NewReader(`{"name": "sse-create-prof", "auth_only": true}`))
	createResp, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Fatalf("failed to create: %v", err)
	}
	createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 created, got %d", createResp.StatusCode)
	}

	line1, _ := reader.ReadString('\n')
	if strings.TrimSpace(line1) != "event: action" {
		t.Fatalf("expected 'event: action' for create, got %q", line1)
	}
	line2, _ := reader.ReadString('\n')
	if !strings.Contains(line2, "create") || !strings.Contains(line2, "sse-create-prof") {
		t.Fatalf("expected create data in SSE, got %q", line2)
	}
	_, _ = reader.ReadString('\n') // empty line

	// Broker also emits "event: profiles" on change
	lineProf1, _ := reader.ReadString('\n')
	if strings.TrimSpace(lineProf1) != "event: profiles" {
		t.Fatalf("expected 'event: profiles', got %q", lineProf1)
	}
	_, _ = reader.ReadString('\n') // data: ...
	_, _ = reader.ReadString('\n') // empty line

	// 2. Test Delete SSE action
	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/profiles/sse-create-prof", nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 delete, got %d", delResp.StatusCode)
	}

	lineDel1, _ := reader.ReadString('\n')
	if strings.TrimSpace(lineDel1) != "event: action" {
		t.Fatalf("expected 'event: action' for delete, got %q", lineDel1)
	}
	lineDel2, _ := reader.ReadString('\n')
	if !strings.Contains(lineDel2, "delete") || !strings.Contains(lineDel2, "sse-create-prof") {
		t.Fatalf("expected delete data in SSE, got %q", lineDel2)
	}
}

func TestSharingMutationEndpoints(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create a profile to test sharing mutations
	createProfReq := httptest.NewRequest(http.MethodPost, "/api/v1/profiles", strings.NewReader(`{"name": "share-prof"}`))
	createProfRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createProfRec, createProfReq)
	if createProfRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for profile creation, got %d: %s", createProfRec.Code, createProfRec.Body.String())
	}

	// 1. GET all sharing should return 5 resources
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/share-prof/sharing", nil)
	getRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for get sharing, got %d", getRec.Code)
	}
	var allResp APIResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &allResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	statuses, ok := allResp.Data.([]any)
	if !ok || len(statuses) != 5 {
		t.Fatalf("expected 5 sharing resources, got %v", allResp.Data)
	}

	// 2. GET individual resource git and alias github
	gitReq := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/share-prof/sharing/git", nil)
	gitRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(gitRec, gitReq)
	if gitRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for get git sharing, got %d", gitRec.Code)
	}

	ghReq := httptest.NewRequest(http.MethodGet, "/api/profiles/share-prof/sharing/github", nil)
	ghRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(ghRec, ghReq)
	if ghRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for get github sharing, got %d", ghRec.Code)
	}

	// 3. POST isolate mcp
	postIsoReq := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/share-prof/sharing/mcp", strings.NewReader(`{"action": "isolate"}`))
	postIsoRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(postIsoRec, postIsoReq)
	if postIsoRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for isolate mcp, got %d: %s", postIsoRec.Code, postIsoRec.Body.String())
	}
	var isoResp APIResponse
	_ = json.Unmarshal(postIsoRec.Body.Bytes(), &isoResp)
	isoMap, _ := isoResp.Data.(map[string]any)
	if isoMap["mode"] != "isolated" {
		t.Errorf("expected mode isolated for mcp, got %v", isoMap["mode"])
	}

	// 4. PUT share mcp
	putShareReq := httptest.NewRequest(http.MethodPut, "/api/profiles/share-prof/sharing/mcp", strings.NewReader(`{"mode": "shared"}`))
	putShareRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(putShareRec, putShareReq)
	if putShareRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for share mcp, got %d: %s", putShareRec.Code, putShareRec.Body.String())
	}
	var shareResp APIResponse
	_ = json.Unmarshal(putShareRec.Body.Bytes(), &shareResp)
	shareMap, _ := shareResp.Data.(map[string]any)
	if shareMap["mode"] != "shared" {
		t.Errorf("expected mode shared for mcp, got %v", shareMap["mode"])
	}

	// 5. POST toggle skills
	toggleReq1 := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/share-prof/sharing/skills", strings.NewReader(`{"action": "toggle"}`))
	toggleRec1 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(toggleRec1, toggleReq1)
	if toggleRec1.Code != http.StatusOK {
		t.Fatalf("expected 200 for toggle skills, got %d: %s", toggleRec1.Code, toggleRec1.Body.String())
	}
	var togResp1 APIResponse
	_ = json.Unmarshal(toggleRec1.Body.Bytes(), &togResp1)
	togMap1, _ := togResp1.Data.(map[string]any)
	if togMap1["mode"] != "isolated" {
		t.Errorf("expected mode isolated after toggle from shared, got %v", togMap1["mode"])
	}

	toggleReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/share-prof/sharing/skills", strings.NewReader(`{"action": "toggle"}`))
	toggleRec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(toggleRec2, toggleReq2)
	if toggleRec2.Code != http.StatusOK {
		t.Fatalf("expected 200 for second toggle skills, got %d: %s", toggleRec2.Code, toggleRec2.Body.String())
	}
	var togResp2 APIResponse
	_ = json.Unmarshal(toggleRec2.Body.Bytes(), &togResp2)
	togMap2, _ := togResp2.Data.(map[string]any)
	if togMap2["mode"] != "shared" {
		t.Errorf("expected mode shared after second toggle, got %v", togMap2["mode"])
	}

	// 6. Config seed route
	seedReq := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/share-prof/sharing/config/seed", nil)
	seedRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(seedRec, seedReq)
	if seedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for config seed route, got %d: %s", seedRec.Code, seedRec.Body.String())
	}

	// 7. Batch sharing (map format)
	batchReq := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/share-prof/sharing", strings.NewReader(`{"mcp": "isolated", "git": "isolated"}`))
	batchRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(batchRec, batchReq)
	if batchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for batch sharing, got %d: %s", batchRec.Code, batchRec.Body.String())
	}

	// 8. Batch sharing (array format)
	batchArrReq := httptest.NewRequest(http.MethodPut, "/api/profiles/share-prof/sharing", strings.NewReader(`[{"resource": "mcp", "action": "share"}, {"resource": "git", "action": "share"}]`))
	batchArrRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(batchArrRec, batchArrReq)
	if batchArrRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for batch array sharing, got %d: %s", batchArrRec.Code, batchArrRec.Body.String())
	}

	// 9. Error cases
	// Profile not found
	errReq1 := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/nonexistent/sharing/mcp", strings.NewReader(`{"action": "share"}`))
	errRec1 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(errRec1, errReq1)
	if errRec1.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent profile, got %d", errRec1.Code)
	}

	// Invalid resource
	errReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/share-prof/sharing/invalid-res", strings.NewReader(`{"action": "share"}`))
	errRec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(errRec2, errReq2)
	if errRec2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid resource, got %d", errRec2.Code)
	}

	// Missing action/mode
	errReq3 := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/share-prof/sharing/mcp", strings.NewReader(`{}`))
	errRec3 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(errRec3, errReq3)
	if errRec3.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing action, got %d", errRec3.Code)
	}
}

func TestSharingMutationSSE(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create profile
	createProfReq := httptest.NewRequest(http.MethodPost, "/api/v1/profiles", strings.NewReader(`{"name": "sse-share-prof"}`))
	createProfRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createProfRec, createProfReq)
	if createProfRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for profile creation, got %d", createProfRec.Code)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Connect SSE client
	resp, err := http.Get(ts.URL + "/api/v1/events")
	if err != nil {
		t.Fatalf("failed to connect to SSE: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Read initial init event
	_, _ = reader.ReadString('\n') // event: init
	_, _ = reader.ReadString('\n') // data: ...
	_, _ = reader.ReadString('\n') // empty line

	// Trigger sharing mutation
	shareReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/profiles/sse-share-prof/sharing/mcp", strings.NewReader(`{"action": "isolate"}`))
	shareReq.Header.Set("Content-Type", "application/json")
	shareResp, err := http.DefaultClient.Do(shareReq)
	if err != nil {
		t.Fatalf("failed to trigger sharing mutation: %v", err)
	}
	shareResp.Body.Close()
	if shareResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", shareResp.StatusCode)
	}

	// Verify SSE broadcast
	line1, _ := reader.ReadString('\n')
	if strings.TrimSpace(line1) != "event: action" {
		t.Fatalf("expected 'event: action' for sharing mutation, got %q", line1)
	}
	line2, _ := reader.ReadString('\n')
	if !strings.Contains(line2, "sharing") || !strings.Contains(line2, "sse-share-prof") {
		t.Fatalf("expected sharing data in SSE, got %q", line2)
	}
}

type testPrimeClient struct{}

func (m *testPrimeClient) RetrieveUserQuotaSummary(port int, csrf string) (*quota.QuotaSummaryResponse, error) {
	return &quota.QuotaSummaryResponse{
		Response: quota.QuotaResponse{
			Groups: []quota.QuotaGroup{
				{
					Buckets: []quota.QuotaBucket{
						{
							BucketID:          "gemini-weekly",
							DisplayName:       "Gemini Weekly",
							RemainingFraction: 1.0,
							ResetTime:         "2026-10-01T00:00:00Z",
						},
					},
				},
			},
		},
	}, nil
}

func (m *testPrimeClient) StartCascade(port int, csrf string) (string, error) {
	return "casc-api-test", nil
}

func (m *testPrimeClient) SendUserCascadeMessage(port int, csrf string, cascadeID string, prompt string, model string) error {
	return nil
}

func TestPrimeEndpoints(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create test profile
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/profiles", strings.NewReader(`{"name": "prime-api-prof"}`))
	createRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createRec.Code)
	}

	cleanup := prime.SetTestHooks(
		func(profile string) ([]quota.ActiveServer, error) {
			if profile == "prime-api-prof" {
				return []quota.ActiveServer{
					{Profile: "prime-api-prof", PID: 5566, Port: 7788, CSRF: "tok-api"},
				}, nil
			}
			return nil, nil
		},
		func(profile string) (*quota.HeadlessInstance, error) {
			return nil, fmt.Errorf("headless mock")
		},
		func(timeout time.Duration) prime.QuotaClient {
			return &testPrimeClient{}
		},
		0,
	)
	defer cleanup()

	// 1. GET /api/v1/profiles/nonexistent/prime -> 404
	get404Req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/nonexistent/prime", nil)
	get404Rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(get404Rec, get404Req)
	if get404Rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent profile prime, got %d", get404Rec.Code)
	}

	// 2. GET /api/v1/profiles/prime-api-prof/prime -> 200
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/prime-api-prof/prime", nil)
	getRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for prime status, got %d: %s", getRec.Code, getRec.Body.String())
	}
	var getResp APIResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatal(err)
	}
	if !getResp.Success {
		t.Errorf("expected success: true")
	}

	// 3. GET /api/v1/prime -> 200 list
	getAllReq := httptest.NewRequest(http.MethodGet, "/api/v1/prime", nil)
	getAllRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(getAllRec, getAllReq)
	if getAllRec.Code != http.StatusOK {
		t.Errorf("expected 200 for get all prime, got %d", getAllRec.Code)
	}

	// 4. POST /api/v1/profiles/prime-api-prof/prime (check dry-run) -> 200
	postCheckReq := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/prime-api-prof/prime", strings.NewReader(`{"check": true, "no_jitter": true}`))
	postCheckRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(postCheckRec, postCheckReq)
	if postCheckRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for check dry-run prime, got %d: %s", postCheckRec.Code, postCheckRec.Body.String())
	}

	// 5. POST /api/v1/profiles/prime-api-prof/prime (actual prime) -> 200
	postPrimeReq := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/prime-api-prof/prime", strings.NewReader(`{"force": true, "no_jitter": true}`))
	postPrimeRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(postPrimeRec, postPrimeReq)
	if postPrimeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for actual prime, got %d: %s", postPrimeRec.Code, postPrimeRec.Body.String())
	}

	// 6. POST /api/v1/prime (batch check) -> 200
	postBatchReq := httptest.NewRequest(http.MethodPost, "/api/v1/prime", strings.NewReader(`{"profiles": ["prime-api-prof"], "check": true}`))
	postBatchRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(postBatchRec, postBatchReq)
	if postBatchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for batch prime, got %d: %s", postBatchRec.Code, postBatchRec.Body.String())
	}

	// 7. POST /api/v1/profiles/prime-api-prof/prime (warm_5h check) -> 200
	postWarmReq := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/prime-api-prof/prime", strings.NewReader(`{"warm_5h": true, "check": true, "no_jitter": true}`))
	postWarmRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(postWarmRec, postWarmReq)
	if postWarmRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for warm_5h check prime, got %d: %s", postWarmRec.Code, postWarmRec.Body.String())
	}
	var apiResp struct {
		Success bool                     `json:"success"`
		Data    prime.ProfilePrimeResult `json:"data"`
	}
	if err := json.Unmarshal(postWarmRec.Body.Bytes(), &apiResp); err != nil {
		t.Fatalf("failed to decode warm_5h response: %v", err)
	}
	if apiResp.Data.Profile != "prime-api-prof" {
		t.Errorf("expected profile prime-api-prof in warm_5h response, got %s", apiResp.Data.Profile)
	}
}

func TestPrimeProgressSSE(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create test profile
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/profiles", strings.NewReader(`{"name": "prime-sse-prof"}`))
	createRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createRec.Code)
	}

	cleanup := prime.SetTestHooks(
		func(profile string) ([]quota.ActiveServer, error) {
			if profile == "prime-sse-prof" {
				return []quota.ActiveServer{
					{Profile: "prime-sse-prof", PID: 1234, Port: 8765, CSRF: "tok-sse"},
				}, nil
			}
			return nil, nil
		},
		func(profile string) (*quota.HeadlessInstance, error) {
			return nil, fmt.Errorf("headless mock")
		},
		func(timeout time.Duration) prime.QuotaClient {
			return &testPrimeClient{}
		},
		0,
	)
	defer cleanup()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Connect SSE client
	resp, err := http.Get(ts.URL + "/api/v1/events")
	if err != nil {
		t.Fatalf("failed to connect to SSE: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Read initial init event
	_, _ = reader.ReadString('\n') // event: init
	_, _ = reader.ReadString('\n') // data: ...
	_, _ = reader.ReadString('\n') // empty line

	// Trigger prime
	primeReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/profiles/prime-sse-prof/prime", strings.NewReader(`{"force": true, "no_jitter": true}`))
	primeReq.Header.Set("Content-Type", "application/json")
	primeResp, err := http.DefaultClient.Do(primeReq)
	if err != nil {
		t.Fatalf("failed to trigger prime: %v", err)
	}
	primeResp.Body.Close()
	if primeResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", primeResp.StatusCode)
	}

	// Read events until we see event: prime
	sawPrimeEvent := false
	sawActionEvent := false
	for i := 0; i < 20; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "event: prime" {
			sawPrimeEvent = true
		}
		if trimmed == "event: action" {
			sawActionEvent = true
			break
		}
	}

	if !sawPrimeEvent {
		t.Errorf("did not observe 'event: prime' in SSE stream")
	}
	if !sawActionEvent {
		t.Errorf("did not observe 'event: action' in SSE stream")
	}
}

func TestGatewayEndpointsInServer(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Mock upstream
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"response":{"candidates":[{"content":{"parts":[{"text":"Hello from gateway server!"}]}}]}}`)
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	gwClient := gateway.NewClient(
		gateway.WithEndpoints([]string{upstream.URL}),
		gateway.WithHTTPClient(upstream.Client()),
	)
	mockGW := gateway.NewGateway(
		gateway.WithGatewayClient(gwClient),
		gateway.WithTokenResolver(func(r *http.Request, p string) (string, error) {
			return "test-token", nil
		}),
	)
	srv.SetGateway(mockGW)

	// 1. Models catalog endpoints: /v1/models and /api/v1/models
	for _, path := range []string{"/v1/models", "/api/v1/models"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", path, rec.Code)
		}
		var models gateway.ModelListResponse
		if err := json.NewDecoder(rec.Body).Decode(&models); err != nil {
			t.Fatalf("failed to decode models from %s: %v", path, err)
		}
		if models.Object != "list" || len(models.Data) == 0 {
			t.Errorf("expected populated models list on %s, got %+v", path, models)
		}
	}

	// 2. Chat completions non-streaming: /v1/chat/completions and /api/v1/chat/completions
	for _, path := range []string{"/v1/chat/completions", "/api/v1/chat/completions"} {
		payload := `{"model": "gemini-2.5-pro", "messages": [{"role": "user", "content": "hi"}], "stream": false}`
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer custom-key")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d: %s", path, rec.Code, rec.Body.String())
		}
		var compResp gateway.ChatCompletionResponse
		if err := json.NewDecoder(rec.Body).Decode(&compResp); err != nil {
			t.Fatalf("failed to decode completion response from %s: %v", path, err)
		}
		if len(compResp.Choices) != 1 || compResp.Choices[0].Message.Content != "Hello from gateway server!" {
			t.Errorf("unexpected choices on %s: %+v", path, compResp.Choices)
		}
	}

	// 3. Anthropic Messages non-streaming: /v1/messages and /api/v1/messages
	for _, path := range []string{"/v1/messages", "/api/v1/messages"} {
		payload := `{"model": "claude-3-7-sonnet", "messages": [{"role": "user", "content": "hi anthropic"}], "max_tokens": 1024}`
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", "anthropic-key-test")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d: %s", path, rec.Code, rec.Body.String())
		}
		var msgResp gateway.AnthropicMessageResponse
		if err := json.NewDecoder(rec.Body).Decode(&msgResp); err != nil {
			t.Fatalf("failed to decode Anthropic message response from %s: %v", path, err)
		}
		if msgResp.Type != "message" || msgResp.Role != "assistant" {
			t.Errorf("unexpected message response format on %s: %+v", path, msgResp)
		}
		if len(msgResp.Content) != 1 || msgResp.Content[0].Text != "Hello from gateway server!" {
			t.Errorf("unexpected content on %s: %+v", path, msgResp.Content)
		}
	}
}

func TestGatewayStreamingInServer(t *testing.T) {
	srv, _ := setupTestServer(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"response":{"candidates":[{"content":{"parts":[{"text":"Streamed "}]}}]}}`)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"response":{"candidates":[{"content":{"parts":[{"text":"response!"}]}}]}}`)
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	gwClient := gateway.NewClient(
		gateway.WithEndpoints([]string{upstream.URL}),
		gateway.WithHTTPClient(upstream.Client()),
	)
	mockGW := gateway.NewGateway(
		gateway.WithGatewayClient(gwClient),
		gateway.WithTokenResolver(func(r *http.Request, p string) (string, error) {
			return "stream-token", nil
		}),
	)
	srv.SetGateway(mockGW)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload := `{"model": "gpt-4o", "messages": [{"role": "user", "content": "stream please"}], "stream": true}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/chat/completions", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to post to /v1/chat/completions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	var chunks []string
	sawDone := false
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "data:") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
				if val == "[DONE]" {
					sawDone = true
					break
				}
				var chunk gateway.ChatCompletionChunk
				if err := json.Unmarshal([]byte(val), &chunk); err == nil {
					if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
						chunks = append(chunks, chunk.Choices[0].Delta.Content)
					}
				}
			}
		}
		if err != nil {
			break
		}
	}

	if !sawDone {
		t.Errorf("did not observe [DONE] in SSE stream")
	}
	combined := strings.Join(chunks, "")
	if combined != "Streamed response!" {
		t.Errorf("unexpected combined streamed content: %q", combined)
	}
}

func TestGatewayRouterInServer(t *testing.T) {
	srv := NewServer(Config{Host: "127.0.0.1", Port: 0})

	router := gateway.NewRouter(gateway.WithRouterStrategy(gateway.StrategySmart))
	router.SyncProfiles([]string{"srv-dev1", "srv-dev2"})

	gw := gateway.NewGateway(gateway.WithGatewayRouter(router))
	srv.SetGateway(gw)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. GET /v1/router/status
	resp1, err := http.Get(ts.URL + "/v1/router/status")
	if err != nil {
		t.Fatalf("failed to GET /v1/router/status: %v", err)
	}
	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.StatusCode)
	}

	var status gateway.RouterStatus
	if err := json.NewDecoder(resp1.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode router status: %v", err)
	}
	if status.TotalProfiles != 2 || status.Strategy != "smart" {
		t.Errorf("unexpected router status: %+v", status)
	}

	// 2. GET /api/v1/router/status (alias)
	resp2, err := http.Get(ts.URL + "/api/v1/router/status")
	if err != nil {
		t.Fatalf("failed to GET /api/v1/router/status: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/router/status, got %d", resp2.StatusCode)
	}

	// 3. POST /v1/router/strategy
	stratPayload := `{"strategy": "round-robin"}`
	resp3, err := http.Post(ts.URL+"/v1/router/strategy", "application/json", strings.NewReader(stratPayload))
	if err != nil {
		t.Fatalf("failed to POST /v1/router/strategy: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp3.StatusCode)
	}

	if router.GetStrategy() != gateway.StrategyRoundRobin {
		t.Errorf("expected strategy to be round-robin, got %s", router.GetStrategy())
	}

	// 4. POST /v1/router/reset
	router.MarkRateLimited("srv-dev1", 429, "rate limited", 10*time.Minute)
	if router.GetStatus().CooldownProfiles != 1 {
		t.Fatalf("expected 1 cooldown profile")
	}

	resp4, err := http.Post(ts.URL+"/v1/router/reset", "application/json", nil)
	if err != nil {
		t.Fatalf("failed to POST /v1/router/reset: %v", err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp4.StatusCode)
	}

	if router.GetStatus().CooldownProfiles != 0 {
		t.Errorf("expected 0 cooldown profiles after reset, got %d", router.GetStatus().CooldownProfiles)
	}
}




