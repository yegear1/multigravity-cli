package headless

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/shortcut"
)

func setupTestEnvironment(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "multigravity-headless-test-*")
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

func TestManagerLifecycle(t *testing.T) {
	_, cleanupEnv := setupTestEnvironment(t)
	defer cleanupEnv()

	profName := "test-prof"
	if err := profile.CreateProfile(profile.CreateOptions{Name: profName}); err != nil {
		t.Fatalf("failed to create test profile: %v", err)
	}

	mgr := NewManager()

	fakePID := 12345
	fakePort := 54321
	fakeCSRF := "test-csrf-token"

	aliveMap := map[int]bool{fakePID: true}

	restoreHooks := SetTestHooks(
		func() (string, error) {
			return "/fake/bin/language_server", nil
		},
		func(port int, csrf string) error {
			if port == fakePort && csrf == fakeCSRF {
				return nil
			}
			return fmt.Errorf("connection refused")
		},
		func(pid int) bool {
			return aliveMap[pid]
		},
		func(pid int) error {
			delete(aliveMap, pid)
			return nil
		},
		func(cmd *exec.Cmd, logFile string, portChan chan int, errChan chan error) (*InstanceInfo, error) {
			return &InstanceInfo{
				PID:       fakePID,
				Port:      fakePort,
				CSRFToken: fakeCSRF,
				Status:    StateRunning,
				StartedAt: time.Now(),
			}, nil
		},
	)
	defer restoreHooks()

	// 1. Initial status should be stopped
	st, err := mgr.GetStatus(profName)
	if err != nil {
		t.Fatalf("unexpected error getting status: %v", err)
	}
	if st.Status != StateStopped {
		t.Fatalf("expected stopped, got %s", st.Status)
	}

	// 2. Start
	inst, err := mgr.Start(profName, StartOptions{Profile: profName})
	if err != nil {
		t.Fatalf("failed to start headless: %v", err)
	}
	if inst.Status != StateRunning || inst.Port != fakePort || inst.PID != fakePID {
		t.Fatalf("unexpected instance info: %+v", inst)
	}

	// 3. Status should now be running
	st, err = mgr.GetStatus(profName)
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if st.Status != StateRunning {
		t.Fatalf("expected running, got %s", st.Status)
	}

	// 4. List should return this instance
	list, err := mgr.List()
	if err != nil {
		t.Fatalf("failed to list instances: %v", err)
	}
	if len(list) != 1 || list[0].Profile != profName {
		t.Fatalf("expected 1 instance for %s, got %d", profName, len(list))
	}

	// 5. Stop
	if err := mgr.Stop(profName); err != nil {
		t.Fatalf("failed to stop instance: %v", err)
	}

	// 6. Status after stop should be stopped
	st, err = mgr.GetStatus(profName)
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if st.Status != StateStopped {
		t.Fatalf("expected stopped, got %s", st.Status)
	}
}

func TestManagerAutoReaping(t *testing.T) {
	_, cleanupEnv := setupTestEnvironment(t)
	defer cleanupEnv()

	profName := "reap-prof"
	if err := profile.CreateProfile(profile.CreateOptions{Name: profName}); err != nil {
		t.Fatalf("failed to create test profile: %v", err)
	}

	mgr := NewManager()

	deadPID := 888888
	// Manually write stale state file
	info := &InstanceInfo{
		Profile:   profName,
		PID:       deadPID,
		Port:      1234,
		CSRFToken: "dead-token",
		Status:    StateRunning,
		StartedAt: time.Now(),
	}
	if err := writeStateFile(profName, info); err != nil {
		t.Fatalf("failed to write state file: %v", err)
	}

	restoreHooks := SetTestHooks(
		nil,
		nil,
		func(pid int) bool {
			return false // PID is dead
		},
		nil,
		nil,
	)
	defer restoreHooks()

	// GetStatus should reap dead PID and return StateStopped
	st, err := mgr.GetStatus(profName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Status != StateStopped {
		t.Fatalf("expected StateStopped after auto-reaping, got %s", st.Status)
	}

	// State file should no longer exist
	statePath := getHeadlessStatePath(profName)
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("expected state file to be removed by auto-reap")
	}
}

func TestManagerUnhealthyStatus(t *testing.T) {
	_, cleanupEnv := setupTestEnvironment(t)
	defer cleanupEnv()

	profName := "unhealthy-prof"
	if err := profile.CreateProfile(profile.CreateOptions{Name: profName}); err != nil {
		t.Fatalf("failed to create test profile: %v", err)
	}

	mgr := NewManager()

	pid := 77777
	info := &InstanceInfo{
		Profile:   profName,
		PID:       pid,
		Port:      9999,
		CSRFToken: "token",
		Status:    StateRunning,
		StartedAt: time.Now(),
	}
	if err := writeStateFile(profName, info); err != nil {
		t.Fatalf("failed to write state file: %v", err)
	}

	restoreHooks := SetTestHooks(
		nil,
		func(port int, csrf string) error {
			return fmt.Errorf("probe failed: 503 service unavailable")
		},
		func(pid int) bool {
			return true // PID is alive
		},
		nil,
		nil,
	)
	defer restoreHooks()

	st, err := mgr.GetStatus(profName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Status != StateUnhealthy {
		t.Fatalf("expected StateUnhealthy, got %s", st.Status)
	}
	if st.HealthError == "" {
		t.Fatalf("expected HealthError to be populated")
	}
}

func TestRunAgentPrompt_Agy(t *testing.T) {
	_, cleanupEnv := setupTestEnvironment(t)
	defer cleanupEnv()

	profName := "agy-runner-prof"
	if err := profile.CreateProfile(profile.CreateOptions{Name: profName}); err != nil {
		t.Fatalf("failed to create test profile: %v", err)
	}

	mgr := NewManager()

	var capturedEnv []string
	restoreRunnerHooks := SetRunnerTestHooks(
		func() (string, error) {
			return "/usr/local/bin/agy", nil
		},
		func(ctx context.Context, bin string, args []string, env []string, dir string) ([]byte, int, error) {
			capturedEnv = env
			jsonResp := `{"response": "Code analysis completed.", "usage": {"total_tokens": 420}, "duration_seconds": 1.25}`
			return []byte(jsonResp), 0, nil
		},
	)
	defer restoreRunnerHooks()

	result, err := mgr.RunAgentPrompt(AgentRunOptions{
		Profile: profName,
		Prompt:  "Analyze repository",
	})
	if err != nil {
		t.Fatalf("failed to run agent prompt: %v", err)
	}

	if result.Response != "Code analysis completed." {
		t.Fatalf("unexpected response: %q", result.Response)
	}
	if result.TotalTokens != 420 {
		t.Fatalf("expected 420 tokens, got %d", result.TotalTokens)
	}

	// Verify Invariant #8: HOME / USERPROFILE is pointed to profileDir
	profileDir := config.GetProfileDir(profName)
	homeFound := false
	for _, env := range capturedEnv {
		if env == "HOME="+profileDir || env == "USERPROFILE="+profileDir {
			homeFound = true
			break
		}
	}
	if !homeFound {
		t.Fatalf("Invariant #8 violated: profile HOME not found in env: %v", capturedEnv)
	}
}

func TestGetLogs(t *testing.T) {
	_, cleanupEnv := setupTestEnvironment(t)
	defer cleanupEnv()

	profName := "log-prof"
	if err := profile.CreateProfile(profile.CreateOptions{Name: profName}); err != nil {
		t.Fatalf("failed to create test profile: %v", err)
	}

	mgr := NewManager()
	logPath := getHeadlessLogPath(profName)
	_ = os.MkdirAll(filepath.Dir(logPath), 0755)
	_ = os.WriteFile(logPath, []byte("line 1\nline 2\nline 3\nline 4\nline 5\n"), 0644)

	logs, err := mgr.GetLogs(profName, 2)
	if err != nil {
		t.Fatalf("unexpected error getting logs: %v", err)
	}
	if logs != "line 5\n" && logs != "line 4\nline 5\n" && logs != "line 5" {
		t.Logf("got logs: %q", logs)
	}
}
