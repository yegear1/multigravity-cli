package server

import (
	"bufio"
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

