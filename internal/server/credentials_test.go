package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/auth"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/gateway"
)

func TestCompletionUsesProfileCredential(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	writeProfileCredential(t, "ada", auth.JetskiRel, "profile-access")
	writeProfileCredential(t, "vera", auth.VaultRel, "vault-access")
	if err := os.MkdirAll(config.GetProfileDir("blank"), 0o755); err != nil {
		t.Fatal(err)
	}

	var saw string
	hits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		saw = r.Header.Get("Authorization")
		if r.Header.Get("x-goog-user-project") != "" {
			t.Errorf("upstream received x-goog-user-project")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"ok\"}]}}]}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	srv := NewServer(Config{Host: "127.0.0.1", Port: 0})
	client := gateway.NewClient(
		gateway.WithEndpoints([]string{upstream.URL}),
		gateway.WithHTTPClient(upstream.Client()),
	)
	gw := gateway.NewGateway(
		gateway.WithGatewayClient(client),
		gateway.WithTokenResolver(resolveProfileAccessToken),
		gateway.WithGatewayRouter(srv.Gateway().Router()),
	)
	srv.SetGateway(gw)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	srv.Gateway().Router().SetFailover(false)
	blankResp, err := postCompletion(ts.URL, "blank", "Bearer client-key")
	if err != nil {
		t.Fatal(err)
	}
	defer blankResp.Body.Close()
	if blankResp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(blankResp.Body)
		t.Fatalf("blank profile status %d body %s", blankResp.StatusCode, body)
	}
	if hits != 0 {
		t.Fatal("unauthenticated profile called upstream")
	}

	adaResp, err := postCompletion(ts.URL, "ada", "Bearer client-key")
	if err != nil {
		t.Fatal(err)
	}
	defer adaResp.Body.Close()
	if adaResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(adaResp.Body)
		t.Fatalf("ada status %d body %s", adaResp.StatusCode, body)
	}
	if saw != "Bearer profile-access" || adaResp.Header.Get("X-Profile-Used") != "ada" {
		t.Fatalf("upstream auth %q profile %q", saw, adaResp.Header.Get("X-Profile-Used"))
	}

	veraResp, err := postCompletion(ts.URL, "vera", "")
	if err != nil {
		t.Fatal(err)
	}
	defer veraResp.Body.Close()
	if veraResp.StatusCode != http.StatusOK || saw != "Bearer vault-access" {
		body, _ := io.ReadAll(veraResp.Body)
		t.Fatalf("vera status %d auth %q body %s", veraResp.StatusCode, saw, body)
	}
}

func writeProfileCredential(t *testing.T, profile, rel, access string) {
	t.Helper()
	cred := map[string]any{
		"auth_method": "consumer",
		"token": map[string]string{
			"access_token":  access,
			"refresh_token": "1//refresh",
			"token_type":    "Bearer",
			"expiry":        time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano),
		},
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(config.GetProfileDir(profile), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func postCompletion(baseURL, profile, authorization string) (*http.Response, error) {
	body := `{"model":"gemini-2.5-flash","stream":false,"messages":[{"role":"user","content":"ping"}]}`
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/chat/completions", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Profile", profile)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	return http.DefaultClient.Do(req)
}
