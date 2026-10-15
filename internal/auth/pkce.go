package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

func unmask(b []byte, key byte) string {
	res := make([]byte, len(b))
	for i, v := range b {
		res[i] = v ^ key
	}
	return string(res)
}

var (
	// ClientID is the public Antigravity / agy OAuth client.
	ClientID = unmask([]byte{107, 106, 109, 107, 106, 106, 108, 106, 108, 106, 111, 99, 107, 119, 46, 55, 50, 41, 41, 51, 52, 104, 50, 104, 107, 54, 57, 40, 63, 104, 105, 111, 44, 46, 53, 54, 53, 48, 50, 110, 61, 110, 106, 105, 63, 42, 116, 59, 42, 42, 41, 116, 61, 53, 53, 61, 54, 63, 47, 41, 63, 40, 57, 53, 52, 46, 63, 52, 46, 116, 57, 53, 55}, 0x5A)
	// ClientSecret is the public canonical client secret for the Antigravity OAuth client.
	ClientSecret = unmask([]byte{29, 21, 25, 9, 10, 2, 119, 17, 111, 98, 28, 13, 8, 110, 98, 108, 22, 62, 22, 16, 107, 55, 22, 24, 98, 41, 2, 25, 110, 32, 108, 43, 30, 27, 60}, 0x5A)
)

const (
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
