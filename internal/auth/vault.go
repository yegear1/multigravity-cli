package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// Status reads the profile vault without returning secrets.
func Status(profileName, profileDir string) (Session, error) {
	session := Session{Profile: profileName, Vault: VaultRel}
	if !HasVault(profileDir) {
		session.Vault = ""
		return session, nil
	}
	session.Authenticated = true

	raw, err := os.ReadFile(VaultPath(profileDir))
	if err != nil {
		return Session{}, err
	}
	var cred credentialFile
	if err := json.Unmarshal(raw, &cred); err != nil {
		return Session{}, fmt.Errorf("profile vault is not valid credential JSON")
	}
	session.Expiry = cred.Token.Expiry
	if exp, err := time.Parse(time.RFC3339Nano, cred.Token.Expiry); err == nil {
		session.AccessExpired = time.Now().After(exp)
	} else if exp, err := time.Parse(time.RFC3339, cred.Token.Expiry); err == nil {
		session.AccessExpired = time.Now().After(exp)
	}

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
