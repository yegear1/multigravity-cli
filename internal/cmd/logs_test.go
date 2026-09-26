package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/idelog"
)

func TestLogsCommandJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	dir := filepath.Join(config.GetUserDataDir(config.GetProfileDir("work")), "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "Starting app\nSpawning: ls --csrf_token hidden-value\n"
	if err := os.WriteFile(filepath.Join(dir, "main.log"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCommand(rootCmd, "logs", "work", "--json", "--tail", "10")
	if err != nil {
		t.Fatal(err)
	}
	var snap idelog.Snapshot
	if err := json.Unmarshal([]byte(out), &snap); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if snap.Source != idelog.SourceIDE || snap.Profile != "work" {
		t.Fatalf("snapshot: %+v", snap)
	}
	if strings.Contains(snap.Logs, "hidden-value") || !strings.Contains(snap.Logs, "[redacted]") {
		t.Fatalf("logs: %q", snap.Logs)
	}
}
