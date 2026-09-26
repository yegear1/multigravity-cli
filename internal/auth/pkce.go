package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

const (
	// ClientID is the public Antigravity / agy OAuth client (PKCE, no client secret).
	ClientID = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"

	AuthEndpoint     = "https://accounts.google.com/o/oauth2/auth"
	TokenEndpoint    = "https://oauth2.googleapis.com/token"
	UserInfoEndpoint = "https://www.googleapis.com/oauth2/v2/userinfo"

	// VaultRel is the agy file-storage credential inside the profile HOME.
	VaultRel = ".gemini/antigravity-cli/antigravity-oauth-token"
	// JetskiRel is the language-server credential path at the profile .gemini root.
	JetskiRel = ".gemini/jetski-standalone-oauth-token"
	// AccountRel stores email and display name only. It never contains tokens.
	AccountRel = ".gemini/antigravity-cli/account.json"
)

var scopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
	"https://www.googleapis.com/auth/cclog",
	"https://www.googleapis.com/auth/experimentsandconfigs",
	"https://www.googleapis.com/auth/aicode",
	"openid",
}

// PKCE holds a one-time S256 challenge pair and the OAuth state nonce.
type PKCE struct {
	Verifier  string
	Challenge string
	State     string
}

// NewPKCE builds a fresh verifier, S256 challenge, and state nonce.
func NewPKCE() (PKCE, error) {
	verifier, err := randomB64(32)
	if err != nil {
		return PKCE{}, err
	}
	state, err := randomB64(16)
	if err != nil {
		return PKCE{}, err
	}
	sum := sha256.Sum256([]byte(verifier))
	return PKCE{
		Verifier:  verifier,
		Challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
		State:     state,
	}, nil
}

func randomB64(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate oauth randomness: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// AuthURL is the Google authorization URL for an ephemeral loopback redirect.
func AuthURL(redirectURI string, pkce PKCE) string {
	q := url.Values{}
	q.Set("client_id", ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(scopes, " "))
	q.Set("code_challenge", pkce.Challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("prompt", "consent")
	q.Set("access_type", "offline")
	q.Set("state", pkce.State)
	return AuthEndpoint + "?" + q.Encode()
}
