package idelog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

func TestReadRedactsSecretsAndTailsMainLogOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	writeIDELogs(t, "work", ""+
		"old line\n"+
		"Spawning: language_server --csrf_token secret-csrf --host_bridge_token=secret-bridge\n"+
		"Starting app\n")

	snap, err := Read("work", 2)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Source != SourceIDE {
		t.Fatalf("source %q", snap.Source)
	}
	if strings.Contains(snap.Logs, "secret-csrf") || strings.Contains(snap.Logs, "secret-bridge") || strings.Contains(snap.Logs, "old line") {
		t.Fatalf("logs leaked or ignored tail: %q", snap.Logs)
	}
	if !strings.Contains(snap.Logs, "--csrf_token [redacted]") || !strings.Contains(snap.Logs, "--host_bridge_token=[redacted]") {
		t.Fatalf("redaction missing: %q", snap.Logs)
	}
	if !strings.Contains(snap.Logs, "Starting app") {
		t.Fatalf("kept line missing: %q", snap.Logs)
	}
	ls := filepath.Join(filepath.Dir(snap.Path), "language_server.log")
	lsData, err := os.ReadFile(ls)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(snap.Logs, string(lsData)) {
		t.Fatal("ide log included language_server.log")
	}
}

func TestReadMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	if _, err := Read("missing", 10); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected missing profile, got %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Read("work", 10); err == nil || !strings.Contains(err.Error(), "ide log not found") {
		t.Fatalf("expected missing log, got %v", err)
	}
}

func TestFollowAppendsSanitizedLines(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	pollInterval = 15 * time.Millisecond
	t.Cleanup(func() { pollInterval = 200 * time.Millisecond })

	path := writeIDELogs(t, "work", "boot --csrf_token first-secret\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := Follow(ctx, "work", 10)
	if err != nil {
		t.Fatal(err)
	}
	first := readChunk(t, ch)
	if strings.Contains(first, "first-secret") || !strings.Contains(first, "[redacted]") {
		t.Fatalf("initial chunk: %q", first)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("ready --host_bridge_token=second-secret\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	second := readChunk(t, ch)
	if strings.Contains(second, "second-secret") || !strings.Contains(second, "ready") {
		t.Fatalf("follow chunk: %q", second)
	}
	cancel()
}

func writeIDELogs(t *testing.T, profile, main string) string {
	t.Helper()
	dir := filepath.Join(config.GetUserDataDir(config.GetProfileDir(profile)), "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "main.log")
	if err := os.WriteFile(path, []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "language_server.log"), []byte("language-server-only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readChunk(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case chunk, ok := <-ch:
		if !ok {
			t.Fatal("follow closed")
		}
		return chunk
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for log chunk")
		return ""
	}
}
