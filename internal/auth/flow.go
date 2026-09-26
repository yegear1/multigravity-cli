package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultLoginTimeout = 3 * time.Minute

// Options controls a headless OAuth login.
type Options struct {
	NoBrowser bool
	Timeout   time.Duration
	// NotifyURL receives the authorization URL before the callback wait.
	NotifyURL func(authURL string)
	// OpenBrowser overrides the platform browser launcher. Nil uses the default.
	OpenBrowser func(authURL string) error
	// TokenURL and UserInfoURL override the Google endpoints. Empty uses production.
	TokenURL    string
	UserInfoURL string
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}

type userInfo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Login runs OAuth2 PKCE against an ephemeral 127.0.0.1 callback and stores
// the credential only inside the profile directory.
func Login(ctx context.Context, profileName, profileDir string, opts Options) (Session, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultLoginTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	pkce, err := NewPKCE()
	if err != nil {
		return Session{}, err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Session{}, fmt.Errorf("failed to listen for oauth callback: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{
		Handler:           callbackHandler(pkce.State, codeCh, errCh),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errorsIsClosed(err) {
			select {
			case errCh <- err:
			default:
			}
		}
	}()
	defer func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutCancel()
		_ = srv.Shutdown(shutCtx)
	}()

	authURL := AuthURL(redirectURI, pkce)
	if opts.NotifyURL != nil {
		opts.NotifyURL(authURL)
	}
	if !opts.NoBrowser {
		open := opts.OpenBrowser
		if open == nil {
			open = openBrowser
		}
		if err := open(authURL); err != nil && opts.NotifyURL == nil {
			return Session{}, fmt.Errorf("failed to open browser: %w", err)
		}
	}

	var code string
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return Session{}, fmt.Errorf("timed out waiting for oauth callback")
		}
		return Session{}, ctx.Err()
	case err := <-errCh:
		return Session{}, err
	case code = <-codeCh:
	}

	tok, err := exchangeCode(ctx, opts.tokenURL(), code, pkce.Verifier, redirectURI)
	if err != nil {
		return Session{}, err
	}
	info, err := fetchUserInfo(ctx, opts.userInfoURL(), tok.AccessToken)
	if err != nil {
		return Session{}, err
	}

	expiry := time.Now().UTC().Add(time.Duration(tok.ExpiresIn) * time.Second).Format(time.RFC3339Nano)
	cred := credentialFile{AuthMethod: authMethodConsumer}
	cred.Token.AccessToken = tok.AccessToken
	cred.Token.TokenType = tok.TokenType
	if cred.Token.TokenType == "" {
		cred.Token.TokenType = "Bearer"
	}
	cred.Token.RefreshToken = tok.RefreshToken
	cred.Token.Expiry = expiry

	if err := saveCredential(profileDir, cred, accountFile{Email: info.Email, Name: info.Name}); err != nil {
		return Session{}, fmt.Errorf("failed to store profile credential: %w", err)
	}
	return Status(profileName, profileDir)
}

func callbackHandler(expected string, codeCh chan<- string, errCh chan<- error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if oauthErr := q.Get("error"); oauthErr != "" {
			http.Error(w, "Authorization failed.", http.StatusBadRequest)
			select {
			case errCh <- fmt.Errorf("authorization denied (%s)", oauthErr):
			default:
			}
			return
		}
		code := q.Get("code")
		state := q.Get("state")
		if code == "" || subtle.ConstantTimeCompare([]byte(state), []byte(expected)) != 1 {
			http.Error(w, "Authorization failed.", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!DOCTYPE html><meta charset=\"utf-8\"><title>Multigravity</title><p>Authentication complete. You can close this tab.</p>")
		select {
		case codeCh <- code:
		default:
		}
	})
	return mux
}

func (o Options) tokenURL() string {
	if o.TokenURL != "" {
		return o.TokenURL
	}
	return TokenEndpoint
}

func (o Options) userInfoURL() string {
	if o.UserInfoURL != "" {
		return o.UserInfoURL
	}
	return UserInfoEndpoint
}

func exchangeCode(ctx context.Context, endpoint, code, verifier, redirectURI string) (tokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", ClientID)
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return tokenResponse{}, fmt.Errorf("token exchange failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return tokenResponse{}, fmt.Errorf("token exchange failed: HTTP %d", resp.StatusCode)
	}
	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return tokenResponse{}, fmt.Errorf("token exchange returned invalid JSON")
	}
	if tok.Error != "" || tok.AccessToken == "" || tok.RefreshToken == "" {
		return tokenResponse{}, fmt.Errorf("token exchange did not return an access token and refresh token")
	}
	if tok.ExpiresIn <= 0 {
		tok.ExpiresIn = 3600
	}
	return tok, nil
}

func fetchUserInfo(ctx context.Context, endpoint, accessToken string) (userInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return userInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := httpClient.Do(req)
	if err != nil {
		return userInfo{}, fmt.Errorf("userinfo request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return userInfo{}, fmt.Errorf("userinfo request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return userInfo{}, fmt.Errorf("userinfo request failed: HTTP %d", resp.StatusCode)
	}
	var info userInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return userInfo{}, fmt.Errorf("userinfo returned invalid JSON")
	}
	return info, nil
}

func errorsIsClosed(err error) bool {
	return err == http.ErrServerClosed
}
