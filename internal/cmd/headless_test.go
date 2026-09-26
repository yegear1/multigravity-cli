package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/headless"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/shortcut"
)

func setupHeadlessCmdTestEnv(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "multigravity-cmd-hl-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	origHome := os.Getenv("MULTIGRAVITY_HOME")
	_ = os.Setenv("MULTIGRAVITY_HOME", tmpDir)

	scDir := filepath.Join(tmpDir, "shortcuts")
	shortcut.SetCustomDirs(
		filepath.Join(scDir, "launchers"),
		filepath.Join(scDir, "applications"),
		filepath.Join(scDir, "Applications"),
		filepath.Join(scDir, "StartMenu"),
	)

	cleanup := func() {
		shortcut.SetCustomDirs("", "", "", "")
		if origHome != "" {
			_ = os.Setenv("MULTIGRAVITY_HOME", origHome)
		} else {
			_ = os.Unsetenv("MULTIGRAVITY_HOME")
		}
		_ = os.RemoveAll(tmpDir)
	}
	return tmpDir, cleanup
}

func TestHeadlessCLI_Lifecycle(t *testing.T) {
	_, cleanup := setupHeadlessCmdTestEnv(t)
	defer cleanup()

	profName := "cli-hl-prof"
	if err := profile.CreateProfile(profile.CreateOptions{Name: profName}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	fakePID := 9876
	fakePort := 43210
	fakeCSRF := "cmd-fake-csrf"
	aliveMap := map[int]bool{fakePID: true}

	restoreHooks := headless.SetTestHooks(
		func() (string, error) {
			return "/fake/language_server", nil
		},
		func(port int, csrf string) error {
			return nil
		},
		func(pid int) bool {
			return aliveMap[pid]
		},
		func(pid int) error {
			delete(aliveMap, pid)
			return nil
		},
		func(cmd *exec.Cmd, logFile string, portChan chan int, errChan chan error) (*headless.InstanceInfo, error) {
			return &headless.InstanceInfo{
				PID:       fakePID,
				Port:      fakePort,
				CSRFToken: fakeCSRF,
				Status:    headless.StateRunning,
				StartedAt: time.Now(),
			}, nil
		},
	)
	defer restoreHooks()

	cmd := newHeadlessCmd()

	// 1. Initial status via JSON
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"status", profName, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("status command failed: %v", err)
	}
	var st headless.InstanceInfo
	if err := json.Unmarshal(buf.Bytes(), &st); err != nil {
		t.Fatalf("failed to parse status JSON: %v, output: %s", err, buf.String())
	}
	if st.Status != headless.StateStopped {
		t.Fatalf("expected StateStopped, got %s", st.Status)
	}

	// 2. Start via JSON
	buf.Reset()
	cmd.SetArgs([]string{"start", profName, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("start command failed: %v", err)
	}
	var startInfo headless.InstanceInfo
	if err := json.Unmarshal(buf.Bytes(), &startInfo); err != nil {
		t.Fatalf("failed to parse start JSON: %v", err)
	}
	if startInfo.Status != headless.StateRunning || startInfo.Port != fakePort {
		t.Fatalf("unexpected start info: %+v", startInfo)
	}

	// 3. List via JSON
	buf.Reset()
	cmd.SetArgs([]string{"list", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}
	var list []*headless.InstanceInfo
	if err := json.Unmarshal(buf.Bytes(), &list); err != nil {
		t.Fatalf("failed to parse list JSON: %v", err)
	}
	if len(list) != 1 || list[0].Profile != profName {
		t.Fatalf("expected 1 instance in list, got %d", len(list))
	}

	// 4. Status text format
	buf.Reset()
	cmd.SetArgs([]string{"status", profName})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("status text command failed: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("RUNNING")) {
		t.Fatalf("expected output to contain RUNNING, got: %s", buf.String())
	}

	// 5. Stop via JSON
	buf.Reset()
	cmd.SetArgs([]string{"stop", profName, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stop command failed: %v", err)
	}
	var stopRes map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &stopRes); err != nil {
		t.Fatalf("failed to parse stop JSON: %v", err)
	}
	if stopRes["status"] != "stopped" {
		t.Fatalf("expected status stopped, got: %v", stopRes)
	}
}

func TestHeadlessCLI_RunPrompt(t *testing.T) {
	_, cleanup := setupHeadlessCmdTestEnv(t)
	defer cleanup()

	profName := "cli-run-prof"
	if err := profile.CreateProfile(profile.CreateOptions{Name: profName}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	restoreRunnerHooks := headless.SetRunnerTestHooks(
		func() (string, error) {
			return "/fake/agy", nil
		},
		func(ctx context.Context, bin string, args []string, env []string, dir string) ([]byte, int, error) {
			out := `{"response": "Test answer from agent", "usage": {"total_tokens": 55}, "duration_seconds": 0.5}`
			return []byte(out), 0, nil
		},
	)
	defer restoreRunnerHooks()

	cmd := newHeadlessCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"run", profName, "Explain goroutines", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run prompt command failed: %v", err)
	}

	var res headless.AgentRunResult
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse run result JSON: %v, raw: %s", err, buf.String())
	}
	if res.Response != "Test answer from agent" || res.TotalTokens != 55 {
		t.Fatalf("unexpected agent run result: %+v", res)
	}
}
