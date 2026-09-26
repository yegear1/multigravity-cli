package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/profile"
)

func TestSnapshotEndpoints(t *testing.T) {
	srv, home := setupTestServer(t)
	if err := os.MkdirAll(filepath.Join(home, "work", ".gemini", "antigravity", "conversations"), 0755); err != nil {
		t.Fatal(err)
	}
	conv := filepath.Join(home, "work", ".gemini", "antigravity", "conversations", "chat.db")
	if err := os.WriteFile(conv, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	token := filepath.Join(home, "work", ".gemini", "jetski-standalone-oauth-token")
	if err := os.WriteFile(token, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	body := bytes.NewBufferString(`{"note":"api"}`)
	resp, err := http.Post(ts.URL+"/api/v1/profiles/work/snapshots", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status %d", resp.StatusCode)
	}
	var created struct {
		Success bool             `json:"success"`
		Data    profile.Snapshot `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if !created.Success || created.Data.Note != "api" {
		t.Fatalf("created: %+v", created)
	}

	if err := os.WriteFile(conv, []byte("v2"), 0644); err != nil {
		t.Fatal(err)
	}
	rollback, err := http.Post(ts.URL+"/api/v1/profiles/work/snapshots/"+created.Data.ID+"/rollback", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rollback.Body.Close()
	if rollback.StatusCode != http.StatusOK {
		t.Fatalf("rollback status %d", rollback.StatusCode)
	}
	got, err := os.ReadFile(conv)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "v1" {
		t.Fatalf("conversation: %s", got)
	}
	gotToken, err := os.ReadFile(token)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotToken) != "secret" {
		t.Fatalf("token: %s", gotToken)
	}

	missing, err := http.Post(ts.URL+"/api/v1/profiles/missing/snapshots", "application/json", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer missing.Body.Close()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", missing.StatusCode)
	}
}
