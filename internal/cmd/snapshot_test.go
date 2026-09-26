package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/profile"
)

func TestSnapshotCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", t.TempDir())

	if err := profile.CreateProfile(profile.CreateOptions{Name: "snap"}); err != nil {
		t.Fatal(err)
	}
	conv := filepath.Join(home, "snap", ".gemini", "antigravity", "conversations", "a.db")
	if err := os.MkdirAll(filepath.Dir(conv), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conv, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	token := filepath.Join(home, "snap", ".gemini", "installation_id")
	if err := os.WriteFile(token, []byte("install-1"), 0600); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand(rootCmd, "snapshot", "create", "snap", "--note", "cli", "--json")
	if err != nil {
		t.Fatalf("create: %v\n%s", err, out)
	}
	var created profile.Snapshot
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatal(err)
	}
	if created.Note != "cli" || created.Profile != "snap" {
		t.Fatalf("created: %+v", created)
	}

	out, err = executeCommand(rootCmd, "snapshot", "list", "snap", "--json")
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	var listed []profile.Snapshot
	if err := json.Unmarshal([]byte(out), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("list: %+v", listed)
	}

	if err := os.WriteFile(conv, []byte("two"), 0644); err != nil {
		t.Fatal(err)
	}
	out, err = executeCommand(rootCmd, "snapshot", "rollback", "snap", created.ID)
	if err != nil {
		t.Fatalf("rollback: %v\n%s", err, out)
	}
	if !strings.Contains(out, created.ID) {
		t.Fatalf("output: %s", out)
	}
	got, err := os.ReadFile(conv)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "one" {
		t.Fatalf("conversation: %s", got)
	}
	gotToken, err := os.ReadFile(token)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotToken) != "install-1" {
		t.Fatalf("token: %s", gotToken)
	}
}
