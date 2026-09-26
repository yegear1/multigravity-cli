package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/headless"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/shortcut"
)

func setupExecCLIHome(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "profiles")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MULTIGRAVITY_HOME", home)
	sc := filepath.Join(root, "shortcuts")
	shortcut.SetCustomDirs(
		filepath.Join(sc, "launchers"),
		filepath.Join(sc, "applications"),
		filepath.Join(sc, "Applications"),
		filepath.Join(sc, "StartMenu"),
	)
	t.Cleanup(func() {
		shortcut.SetCustomDirs("", "", "", "")
	})
}

func TestExecCLI_JSON(t *testing.T) {
	setupExecCLIHome(t)

	for _, name := range []string{"cli-exec-a", "cli-exec-b"} {
		if err := profile.CreateProfile(profile.CreateOptions{Name: name}); err != nil {
			t.Fatalf("failed to create profile %s: %v", name, err)
		}
	}

	restore := headless.SetRunnerTestHooks(
		func() (string, error) { return "/fake/agy", nil },
		func(ctx context.Context, bin string, args []string, env []string, dir string) ([]byte, int, error) {
			name := filepath.Base(dir)
			out := `{"response":"answer-` + name + `","usage":{"total_tokens":7},"duration_seconds":0.1}`
			return []byte(out), 0, nil
		},
	)
	defer restore()

	cmd := newExecCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--all", "Explain goroutines", "--json", "--workers", "2"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec command failed: %v\n%s", err, buf.String())
	}

	var report headless.ExecReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse exec JSON: %v, raw: %s", err, buf.String())
	}
	if report.Succeeded != 2 || report.Failed != 0 || report.TotalTokens != 14 || report.Workers != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Prompt != "Explain goroutines" {
		t.Fatalf("unexpected prompt: %q", report.Prompt)
	}
}

func TestExecCLI_PartialFailureExit(t *testing.T) {
	setupExecCLIHome(t)

	if err := profile.CreateProfile(profile.CreateOptions{Name: "cli-exec-fail"}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	restore := headless.SetRunnerTestHooks(
		func() (string, error) { return "/fake/agy", nil },
		func(ctx context.Context, bin string, args []string, env []string, dir string) ([]byte, int, error) {
			return []byte(`{"response":"nope"}`), 1, context.Canceled
		},
	)
	defer restore()

	cmd := newExecCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"cli-exec-fail", "ping", "--json"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected non-zero exit when a profile run fails")
	}

	var report headless.ExecReport
	if jsonErr := json.Unmarshal(buf.Bytes(), &report); jsonErr != nil {
		t.Fatalf("expected JSON before the error: %v, raw: %s", jsonErr, buf.String())
	}
	if report.Failed != 1 {
		t.Fatalf("expected failed=1, got %+v", report)
	}
}
