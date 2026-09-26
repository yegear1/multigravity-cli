package headless

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
	"github.com/ye-dev/multigravity-cli/internal/shortcut"
)

func setupExecHome(t *testing.T) {
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

func TestExecFanOutParallel(t *testing.T) {
	setupExecHome(t)

	for _, name := range []string{"exec-a", "exec-b"} {
		if err := profile.CreateProfile(profile.CreateOptions{Name: name}); err != nil {
			t.Fatalf("failed to create profile %s: %v", name, err)
		}
	}

	var inflight atomic.Int32
	var maxInflight atomic.Int32
	entered := make(chan struct{}, 2)
	release := make(chan struct{})

	restore := SetRunnerTestHooks(
		func() (string, error) { return "/usr/local/bin/agy", nil },
		func(ctx context.Context, bin string, args []string, env []string, dir string) ([]byte, int, error) {
			n := inflight.Add(1)
			for {
				old := maxInflight.Load()
				if n <= old || maxInflight.CompareAndSwap(old, n) {
					break
				}
			}
			entered <- struct{}{}
			<-release
			inflight.Add(-1)

			name := filepath.Base(dir)
			body := []byte(`{"response":"ok-` + name + `","usage":{"total_tokens":10},"duration_seconds":0.2}`)
			return body, 0, nil
		},
	)
	defer restore()

	mgr := NewManager()
	done := make(chan *ExecReport, 1)
	errCh := make(chan error, 1)
	go func() {
		report, err := mgr.Exec(ExecOptions{
			All:                        true,
			Prompt:                     "ping",
			DangerouslySkipPermissions: true,
		})
		if err != nil {
			errCh <- err
			return
		}
		done <- report
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case err := <-errCh:
			t.Fatalf("exec failed before both workers started: %v", err)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for parallel workers")
		}
	}
	if maxInflight.Load() < 2 {
		t.Fatalf("expected both profiles in flight, max=%d", maxInflight.Load())
	}
	close(release)

	var report *ExecReport
	select {
	case report = <-done:
	case err := <-errCh:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for exec report")
	}

	if report.Workers != 2 || report.Succeeded != 2 || report.Failed != 0 || report.TotalTokens != 20 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Results[0].Profile != "exec-a" || report.Results[1].Profile != "exec-b" {
		t.Fatalf("expected stable profile order, got %+v", report.Results)
	}
	if report.Results[0].Response != "ok-exec-a" || report.Results[1].Response != "ok-exec-b" {
		t.Fatalf("unexpected responses: %+v", report.Results)
	}
	series, err := quota.LoadSeries("exec-a", quota.HistoryQuery{Source: quota.SourceHeadless, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if series.Summary.TotalTokens != 10 {
		t.Fatalf("expected headless token sample, got %+v", series.Summary)
	}
}

func TestExecWorkersCap(t *testing.T) {
	setupExecHome(t)

	for _, name := range []string{"cap-a", "cap-b"} {
		if err := profile.CreateProfile(profile.CreateOptions{Name: name}); err != nil {
			t.Fatalf("failed to create profile %s: %v", name, err)
		}
	}

	var inflight atomic.Int32
	var maxInflight atomic.Int32
	entered := make(chan struct{}, 2)
	release := make(chan struct{})

	restore := SetRunnerTestHooks(
		func() (string, error) { return "/usr/local/bin/agy", nil },
		func(ctx context.Context, bin string, args []string, env []string, dir string) ([]byte, int, error) {
			n := inflight.Add(1)
			for {
				old := maxInflight.Load()
				if n <= old || maxInflight.CompareAndSwap(old, n) {
					break
				}
			}
			entered <- struct{}{}
			<-release
			inflight.Add(-1)
			return []byte(`{"response":"ok","usage":{"total_tokens":1}}`), 0, nil
		},
	)
	defer restore()

	mgr := NewManager()
	done := make(chan error, 1)
	go func() {
		_, err := mgr.Exec(ExecOptions{
			Profiles: []string{"cap-a", "cap-b", "cap-a"},
			Prompt:   "ping",
			Workers:  1,
		})
		done <- err
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the single worker")
	}
	select {
	case <-entered:
		t.Fatal("second profile started while workers=1")
	case <-time.After(80 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for capped exec")
	}
	if maxInflight.Load() != 1 {
		t.Fatalf("expected max inflight 1, got %d", maxInflight.Load())
	}
}

func TestExecRejectsMissingPromptAndProfile(t *testing.T) {
	setupExecHome(t)

	mgr := NewManager()
	if _, err := mgr.Exec(ExecOptions{Profile: "missing", Prompt: "   "}); err == nil {
		t.Fatal("expected empty prompt to fail")
	}
	if _, err := mgr.Exec(ExecOptions{Profile: "missing", Prompt: "ping"}); err == nil {
		t.Fatal("expected missing profile to fail")
	}
	if _, err := mgr.Exec(ExecOptions{All: true, Prompt: "ping"}); err == nil {
		t.Fatal("expected --all with no profiles to fail")
	}
}

func TestExecRecordsPerProfileFailure(t *testing.T) {
	setupExecHome(t)

	for _, name := range []string{"ok-prof", "bad-prof"} {
		if err := profile.CreateProfile(profile.CreateOptions{Name: name}); err != nil {
			t.Fatalf("failed to create profile %s: %v", name, err)
		}
	}

	restore := SetRunnerTestHooks(
		func() (string, error) { return "/usr/local/bin/agy", nil },
		func(ctx context.Context, bin string, args []string, env []string, dir string) ([]byte, int, error) {
			if filepath.Base(dir) == "bad-prof" {
				return []byte(`{"response":"","usage":{"total_tokens":3}}`), 2, context.DeadlineExceeded
			}
			return []byte(`{"response":"fine","usage":{"total_tokens":4}}`), 0, nil
		},
	)
	defer restore()

	mgr := NewManager()
	report, err := mgr.Exec(ExecOptions{
		Profiles: []string{"ok-prof", "bad-prof"},
		Prompt:   "ping",
	})
	if err != nil {
		t.Fatalf("fan-out should return the aggregate, got %v", err)
	}
	if report.Succeeded != 1 || report.Failed != 1 || report.TotalTokens != 4 {
		t.Fatalf("unexpected aggregate: %+v", report)
	}
	if report.Results[1].ExitCode != 2 || report.Results[1].Error == "" {
		t.Fatalf("expected failure captured on bad-prof: %+v", report.Results[1])
	}
}
