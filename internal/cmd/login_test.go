package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/auth"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

func TestLoginCommandJSON(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", t.TempDir())
	if err := profile.CreateProfile(profile.CreateOptions{Name: "ada"}); err != nil {
		t.Fatal(err)
	}

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "ya29-cli",
			"refresh_token": "1//cli-refresh",
			"token_type":    "Bearer",
			"expires_in":    120,
		})
	}))
	defer tokenSrv.Close()
	infoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"email": "ada@example.com", "name": "Ada"})
	}))
	defer infoSrv.Close()

	prepareLoginOptions = func(opts *auth.Options) {
		opts.TokenURL = tokenSrv.URL
		opts.UserInfoURL = infoSrv.URL
		opts.OpenBrowser = func(raw string) error {
			u, _ := url.Parse(raw)
			state := u.Query().Get("state")
			redirect := u.Query().Get("redirect_uri")
			go func() {
				target := redirect + "?code=cli-code&state=" + url.QueryEscape(state)
				for i := 0; i < 25; i++ {
					resp, err := http.Get(target)
					if err == nil {
						resp.Body.Close()
						return
					}
					time.Sleep(20 * time.Millisecond)
				}
			}()
			return nil
		}
	}
	t.Cleanup(func() { prepareLoginOptions = nil })

	out, err := executeCommand(rootCmd, "login", "ada", "--json", "--timeout", "5s")
	if err != nil {
		t.Fatalf("login: %v\n%s", err, out)
	}
	if strings.Contains(out, "ya29") || strings.Contains(out, "cli-refresh") || strings.Contains(out, "access_token") {
		t.Fatalf("cli output leaked a token: %s", out)
	}
	var session auth.Session
	start := strings.Index(out, "{")
	if start < 0 {
		t.Fatalf("no json in output: %s", out)
	}
	if err := json.Unmarshal([]byte(out[start:]), &session); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if !session.Authenticated || session.Email != "ada@example.com" || session.Vault != auth.VaultRel {
		t.Fatalf("session %+v", session)
	}
	if !auth.HasVault(config.GetProfileDir("ada")) {
		t.Fatal("expected profile vault")
	}

	statusOut, err := executeCommand(rootCmd, "login", "status", "ada", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(statusOut, "cli-refresh") {
		t.Fatalf("status leaked refresh token: %s", statusOut)
	}

	if _, err := executeCommand(rootCmd, "login", "logout", "ada"); err != nil {
		t.Fatal(err)
	}
	if auth.HasVault(config.GetProfileDir("ada")) {
		t.Fatal("logout left the vault in place")
	}
}
