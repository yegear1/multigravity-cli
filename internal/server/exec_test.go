package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/headless"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

func TestExecEndpoint(t *testing.T) {
	srv, home := setupTestServer(t)
	outside := filepath.Join(filepath.Dir(home), "exec-shortcuts")
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", outside)

	for _, name := range []string{"api-exec-a", "api-exec-b"} {
		if err := profile.CreateProfile(profile.CreateOptions{Name: name}); err != nil {
			t.Fatalf("failed to create profile %s: %v", name, err)
		}
	}

	restore := headless.SetRunnerTestHooks(
		func() (string, error) { return "/fake/agy", nil },
		func(ctx context.Context, bin string, args []string, env []string, dir string) ([]byte, int, error) {
			name := filepath.Base(dir)
			return []byte(`{"response":"api-` + name + `","usage":{"total_tokens":2}}`), 0, nil
		},
	)
	defer restore()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/exec", strings.NewReader(`{"all":true,"prompt":"ping","workers":2}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}

	var apiResp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &apiResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !apiResp.Success {
		t.Fatalf("expected success, body: %s", rec.Body.String())
	}

	raw, err := json.Marshal(apiResp.Data)
	if err != nil {
		t.Fatal(err)
	}
	var report headless.ExecReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if report.Succeeded != 2 || report.TotalTokens != 4 || report.Prompt != "ping" {
		t.Fatalf("unexpected report: %+v", report)
	}

	bad := httptest.NewRequest(http.MethodPost, "/api/exec", strings.NewReader(`{}`))
	badRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(badRec, bad)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing prompt, got %d", badRec.Code)
	}
}
