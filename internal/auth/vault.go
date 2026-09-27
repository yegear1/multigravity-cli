package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const authMethodConsumer = "consumer"

// Session is the public login result. It never carries access or refresh tokens.
type Session struct {
	Profile       string `json:"profile"`
	Authenticated bool   `json:"authenticated"`
	Email         string `json:"email,omitempty"`
	Name          string `json:"name,omitempty"`
	Expiry        string `json:"expiry,omitempty"`
	AccessExpired bool   `json:"access_expired,omitempty"`
	Vault         string `json:"vault,omitempty"`
}

type credentialFile struct {
	Token struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		Expiry       string `json:"expiry"`
	} `json:"token"`
	AuthMethod string `json:"auth_method"`
}

type accountFile struct {
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

// VaultPath returns the primary credential file inside a profile directory.
func VaultPath(profileDir string) string {
	return filepath.Join(profileDir, filepath.FromSlash(VaultRel))
}

// HasVault reports whether the profile has a file-based OAuth credential.
func HasVault(profileDir string) bool {
	info, err := os.Stat(VaultPath(profileDir))
	return err == nil && !info.IsDir()
}

func saveCredential(profileDir string, cred credentialFile, account accountFile) error {
	payload, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')

	vault := VaultPath(profileDir)
	jetski := filepath.Join(profileDir, filepath.FromSlash(JetskiRel))
	if err := writePrivateFile(vault, payload); err != nil {
		return err
	}
	if err := writePrivateFile(jetski, payload); err != nil {
		_ = os.Remove(vault)
		return err
	}

	meta, err := json.Marshal(account)
	if err != nil {
		_ = os.Remove(vault)
		_ = os.Remove(jetski)
		return err
	}
	meta = append(meta, '\n')
	accountPath := filepath.Join(profileDir, filepath.FromSlash(AccountRel))
	if err := writePrivateFile(accountPath, meta); err != nil {
		_ = os.Remove(vault)
		_ = os.Remove(jetski)
		return err
	}
	return nil
}

func writePrivateFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create credential directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".vault-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_ = os.Remove(path)
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

// Status reads the profile credential without returning secrets.
// The login vault wins; the IDE jetski file counts when the vault is absent.
func Status(profileName, profileDir string) (Session, error) {
	session := Session{Profile: profileName}
	cred, rel, err := loadCredential(profileDir)
	if errors.Is(err, ErrNotAuthenticated) {
		return session, nil
	}
	if err != nil {
		return Session{}, err
	}
	session.Authenticated = true
	session.Vault = rel
	session.Expiry = cred.Token.Expiry
	session.AccessExpired = accessExpired(cred.Token.Expiry, time.Now())

	accountPath := filepath.Join(profileDir, filepath.FromSlash(AccountRel))
	if metaRaw, err := os.ReadFile(accountPath); err == nil {
		var account accountFile
		if json.Unmarshal(metaRaw, &account) == nil {
			session.Email = account.Email
			session.Name = account.Name
		}
	}
	return session, nil
}

// Logout deletes the profile vault files. Missing files are not an error when
// nothing was stored; ErrNotAuthenticated is returned in that case.
func Logout(profileDir string) error {
	paths := []string{
		VaultPath(profileDir),
		filepath.Join(profileDir, filepath.FromSlash(JetskiRel)),
		filepath.Join(profileDir, filepath.FromSlash(AccountRel)),
	}
	removed := false
	for _, path := range paths {
		err := os.Remove(path)
		if err == nil {
			removed = true
			continue
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		return err
	}
	if !removed {
		return ErrNotAuthenticated
	}
	return nil
}

// ErrNotAuthenticated is returned when logout finds no profile credential.
var ErrNotAuthenticated = errors.New("profile is not authenticated")

func loadCredential(profileDir string) (credentialFile, string, error) {
	var invalid bool
	for _, rel := range []string{VaultRel, JetskiRel} {
		path := filepath.Join(profileDir, filepath.FromSlash(rel))
		raw, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return credentialFile{}, "", err
		}
		var cred credentialFile
		if err := json.Unmarshal(raw, &cred); err != nil {
			invalid = true
			continue
		}
		if cred.Token.AccessToken == "" && cred.Token.RefreshToken == "" {
			continue
		}
		return cred, rel, nil
	}
	if invalid {
		return credentialFile{}, "", fmt.Errorf("profile credential is not valid JSON")
	}
	return credentialFile{}, "", ErrNotAuthenticated
}

func accessExpired(expiry string, now time.Time) bool {
	if strings.TrimSpace(expiry) == "" {
		return false
	}
	exp, err := time.Parse(time.RFC3339Nano, expiry)
	if err != nil {
		exp, err = time.Parse(time.RFC3339, expiry)
	}
	if err != nil {
		return false
	}
	return !now.Before(exp)
}

func writeCredentialTokens(profileDir string, cred credentialFile) error {
	payload, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	vault := VaultPath(profileDir)
	jetski := filepath.Join(profileDir, filepath.FromSlash(JetskiRel))
	if err := writePrivateFile(vault, payload); err != nil {
		return err
	}
	if err := writePrivateFile(jetski, payload); err != nil {
		_ = os.Remove(vault)
		return err
	}
	return nil
}
