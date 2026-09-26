package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewPKCE(t *testing.T) {
	pkce, err := NewPKCE()
	if err != nil {
		t.Fatal(err)
	}
	if len(pkce.Verifier) < 43 || len(pkce.Challenge) == 0 || pkce.State == "" {
		t.Fatalf("unexpected pkce material: verifier=%d challenge=%d state=%d", len(pkce.Verifier), len(pkce.Challenge), len(pkce.State))
	}
	sum := sha256.Sum256([]byte(pkce.Verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if pkce.Challenge != want {
		t.Fatalf("challenge is not S256 of verifier")
	}
	if strings.Contains(pkce.Verifier, "=") || strings.Contains(pkce.Challenge, "+") || strings.Contains(pkce.Challenge, "/") {
		t.Fatalf("pkce values must be raw url-safe base64")
	}

	raw := AuthURL("http://127.0.0.1:9/callback", pkce)
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("client_id") != ClientID || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") != pkce.Challenge {
		t.Fatalf("auth url missing pkce parameters: %s", raw)
	}
	if q.Get("redirect_uri") != "http://127.0.0.1:9/callback" || q.Get("state") != pkce.State {
		t.Fatalf("auth url missing redirect or state")
	}
	if !strings.Contains(q.Get("scope"), "openid") || q.Get("access_type") != "offline" {
		t.Fatalf("auth url missing scope or offline access")
	}
}

func callbackAfterListen(raw, query string) {
	u, err := url.Parse(raw)
	if err != nil {
		return
	}
	redirect := u.Query().Get("redirect_uri")
	target := redirect + "?" + query
	for i := 0; i < 25; i++ {
		resp, err := http.Get(target)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestLoginStoresVaultOnly(t *testing.T) {
	profileDir := t.TempDir()
	hostHome := t.TempDir()

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("token method %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		if form.Get("grant_type") != "authorization_code" || form.Get("client_id") != ClientID || form.Get("code") != "good-code" || form.Get("code_verifier") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !strings.HasPrefix(form.Get("redirect_uri"), "http://127.0.0.1:") || !strings.HasSuffix(form.Get("redirect_uri"), "/callback") {
			t.Errorf("redirect %s", form.Get("redirect_uri"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "ya29-test-access",
			"refresh_token": "1//test-refresh",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer tokenSrv.Close()

	infoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ya29-test-access" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"email": "ada@example.com", "name": "Ada"})
	}))
	defer infoSrv.Close()

	session, err := Login(context.Background(), "ada", profileDir, Options{
		Timeout:     5 * time.Second,
		TokenURL:    tokenSrv.URL,
		UserInfoURL: infoSrv.URL,
		OpenBrowser: func(raw string) error {
			u, err := url.Parse(raw)
			if err != nil {
				return err
			}
			q := u.Query()
			redirect := q.Get("redirect_uri")
			if !strings.HasPrefix(redirect, "http://127.0.0.1:") {
				t.Errorf("callback is not loopback: %s", redirect)
			}
			state := q.Get("state")
			go func() {
				callbackAfterListen(raw, "code=nope&state=wrong")
				callbackAfterListen(raw, "code=good-code&state="+url.QueryEscape(state))
			}()
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !session.Authenticated || session.Email != "ada@example.com" || session.Name != "Ada" || session.Vault != VaultRel {
		t.Fatalf("session %+v", session)
	}
	encoded, _ := json.Marshal(session)
	if strings.Contains(string(encoded), "ya29") || strings.Contains(string(encoded), "refresh") || strings.Contains(string(encoded), "access_token") {
		t.Fatalf("session JSON leaked a token: %s", encoded)
	}

	vaultRaw, err := os.ReadFile(VaultPath(profileDir))
	if err != nil {
		t.Fatal(err)
	}
	jetskiRaw, err := os.ReadFile(filepath.Join(profileDir, filepath.FromSlash(JetskiRel)))
	if err != nil {
		t.Fatal(err)
	}
	if string(vaultRaw) != string(jetskiRaw) {
		t.Fatal("jetski copy diverged from the profile vault")
	}
	if !strings.Contains(string(vaultRaw), "1//test-refresh") || !strings.Contains(string(vaultRaw), `"auth_method":"consumer"`) {
		t.Fatalf("vault payload = %s", vaultRaw)
	}
	info, err := os.Stat(VaultPath(profileDir))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("vault mode %v is group/world accessible", info.Mode().Perm())
	}

	accountRaw, err := os.ReadFile(filepath.Join(profileDir, filepath.FromSlash(AccountRel)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(accountRaw), "refresh") || strings.Contains(string(accountRaw), "ya29") {
		t.Fatalf("account file contains a token: %s", accountRaw)
	}
	if _, err := os.Stat(filepath.Join(hostHome, ".gemini")); !os.IsNotExist(err) {
		t.Fatal("host home received a credential directory")
	}

	again, err := Status("ada", profileDir)
	if err != nil || again.Email != "ada@example.com" || again.AccessExpired {
		t.Fatalf("status %+v err %v", again, err)
	}

	if err := Logout(profileDir); err != nil {
		t.Fatal(err)
	}
	if HasVault(profileDir) {
		t.Fatal("vault survived logout")
	}
	if _, err := os.Stat(filepath.Join(profileDir, filepath.FromSlash(JetskiRel))); !os.IsNotExist(err) {
		t.Fatal("jetski file survived logout")
	}
	empty, err := Status("ada", profileDir)
	if err != nil || empty.Authenticated {
		t.Fatalf("expected signed-out status, got %+v %v", empty, err)
	}
	if err := Logout(profileDir); err != ErrNotAuthenticated {
		t.Fatalf("second logout: %v", err)
	}
}

func TestLoginRejectsDeniedCallback(t *testing.T) {
	profileDir := t.TempDir()
	_, err := Login(context.Background(), "ada", profileDir, Options{
		Timeout: 5 * time.Second,
		OpenBrowser: func(raw string) error {
			go callbackAfterListen(raw, "error=access_denied")
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Fatalf("expected denial, got %v", err)
	}
	if HasVault(profileDir) {
		t.Fatal("denied login wrote a vault")
	}
}
