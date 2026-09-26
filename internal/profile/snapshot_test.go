package profile

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotExcludesTokensAndRestoresConversations(t *testing.T) {
	home := setupTestHome(t)
	name := "work"
	if err := CreateProfile(CreateOptions{
		Name:             name,
		AuthOnly:         true,
		IsolatedDotfiles: true,
		IsolatedGH:       true,
	}); err != nil {
		t.Fatal(err)
	}
	pDir := filepath.Join(home, name)

	settings := filepath.Join(pDir, ".config", "Antigravity", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{"theme":"dark"}`), 0644); err != nil {
		t.Fatal(err)
	}
	conv := filepath.Join(pDir, ".gemini", "antigravity", "conversations", "chat.db")
	if err := os.MkdirAll(filepath.Dir(conv), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conv, []byte("conversation-v1"), 0644); err != nil {
		t.Fatal(err)
	}
	token := filepath.Join(pDir, ".gemini", "jetski-standalone-oauth-token")
	if err := os.WriteFile(token, []byte("token-a"), 0600); err != nil {
		t.Fatal(err)
	}
	oauth := filepath.Join(pDir, ".gemini", "antigravity-cli", "antigravity-oauth-token")
	if err := os.MkdirAll(filepath.Dir(oauth), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oauth, []byte("oauth-a"), 0600); err != nil {
		t.Fatal(err)
	}
	gh := filepath.Join(pDir, ".config", "gh", "hosts.yml")
	if err := os.MkdirAll(filepath.Dir(gh), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gh, []byte("github-token"), 0600); err != nil {
		t.Fatal(err)
	}
	ssh := filepath.Join(pDir, ".ssh", "id_ed25519")
	if err := os.MkdirAll(filepath.Dir(ssh), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ssh, []byte("private-key"), 0600); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(pDir, ".cache", "temp.log")
	if err := os.MkdirAll(filepath.Dir(cache), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache, []byte("cache"), 0644); err != nil {
		t.Fatal(err)
	}

	snap, err := CreateSnapshot(name, "before edit")
	if err != nil {
		t.Fatal(err)
	}
	if snap.ID == "" || !strings.Contains(strings.Join(snap.Includes, ","), includeConversations) {
		t.Fatalf("snapshot: %+v", snap)
	}

	names := tarNames(t, snap.Archive)
	joined := strings.Join(names, "\n")
	for _, forbidden := range []string{"jetski-standalone-oauth-token", "antigravity-oauth-token", "hosts.yml", "id_ed25519", "temp.log"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("archive contains %s:\n%s", forbidden, joined)
		}
	}
	if !strings.Contains(joined, "settings.json") || !strings.Contains(joined, "chat.db") || !strings.Contains(joined, ".auth_only") {
		t.Fatalf("archive missing profile data:\n%s", joined)
	}

	if err := os.WriteFile(settings, []byte(`{"theme":"light"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conv, []byte("conversation-v2"), 0644); err != nil {
		t.Fatal(err)
	}
	later := filepath.Join(filepath.Dir(conv), "later.db")
	if err := os.WriteFile(later, []byte("after"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(token, []byte("token-b"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := RollbackSnapshot(name, snap.ID); err != nil {
		t.Fatal(err)
	}
	gotSettings, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotSettings) != `{"theme":"dark"}` {
		t.Fatalf("settings: %s", gotSettings)
	}
	gotConv, err := os.ReadFile(conv)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotConv) != "conversation-v1" {
		t.Fatalf("conversation: %s", gotConv)
	}
	if _, err := os.Stat(later); !os.IsNotExist(err) {
		t.Fatalf("post-snapshot conversation still present: %v", err)
	}
	gotToken, err := os.ReadFile(token)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotToken) != "token-b" {
		t.Fatalf("token was rolled back or dropped: %s", gotToken)
	}
	gotOAuth, err := os.ReadFile(oauth)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotOAuth) != "oauth-a" {
		t.Fatalf("oauth vault: %s", gotOAuth)
	}
	gotKey, err := os.ReadFile(ssh)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotKey) != "private-key" {
		t.Fatalf("ssh key: %s", gotKey)
	}
}

func TestSnapshotWriteBlockedWhileRunning(t *testing.T) {
	home := setupTestHome(t)
	name := "live"
	if err := CreateProfile(CreateOptions{Name: name}); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(home, name, "marker.txt")
	if err := os.WriteFile(marker, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}

	old := getProfilePIDsFn
	t.Cleanup(func() { getProfilePIDsFn = old })
	getProfilePIDsFn = func(string) ([]int, error) { return []int{4242}, nil }

	if _, err := CreateSnapshot(name, ""); err == nil {
		t.Fatal("expected create to fail while running")
	} else if _, ok := err.(*ProfileBusyError); !ok {
		t.Fatalf("expected ProfileBusyError, got %v", err)
	}
	if entries, err := os.ReadDir(SnapshotDir(name)); err == nil && len(entries) != 0 {
		t.Fatalf("snapshot store was written: %v", entries)
	}

	getProfilePIDsFn = func(string) ([]int, error) { return nil, nil }
	snap, err := CreateSnapshot(name, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("v2"), 0644); err != nil {
		t.Fatal(err)
	}
	getProfilePIDsFn = func(string) ([]int, error) { return []int{7}, nil }
	if _, err := RollbackSnapshot(name, snap.ID); err == nil {
		t.Fatal("expected rollback to fail while running")
	}
	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "v2" {
		t.Fatalf("rollback mutated a running profile: %s", got)
	}
}

func TestSnapshotRejectsPathEscape(t *testing.T) {
	if _, err := cleanArchiveRel("../etc/passwd"); err == nil {
		t.Fatal("expected rejection")
	}
	if _, err := cleanArchiveRel("/tmp/x"); err == nil {
		t.Fatal("expected rejection")
	}
}

func tarNames(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	var names []string
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		names = append(names, hdr.Name)
	}
	return names
}
