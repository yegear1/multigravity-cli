package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetectAgentType(t *testing.T) {
	tests := []struct {
		cmd      string
		expected string
	}{
		{"claude", "claude"},
		{"/usr/local/bin/claude", "claude"},
		{"claude.exe", "claude"},
		{"aider", "aider"},
		{"python -m aider", "custom"},
		{"opencode", "opencode"},
		{"open-code", "opencode"},
		{"agy", "agy"},
		{"antigravity", "agy"},
		{"bash", "custom"},
		{"sh", "custom"},
	}

	for _, tc := range tests {
		got := DetectAgentType(tc.cmd)
		if got != tc.expected {
			t.Errorf("DetectAgentType(%q) = %q; want %q", tc.cmd, got, tc.expected)
		}
	}
}

func TestBuildAgentEnv(t *testing.T) {
	tempHome := t.TempDir()
	profDir := filepath.Join(tempHome, "AntigravityProfiles", "test-agent-prof")
	if err := os.MkdirAll(profDir, 0755); err != nil {
		t.Fatalf("failed to create profile dir: %v", err)
	}

	t.Setenv("MULTIGRAVITY_HOME", filepath.Join(tempHome, "AntigravityProfiles"))
	t.Setenv("REAL_HOME", tempHome)

	opts := CreateSessionOptions{
		Profile:    "test-agent-prof",
		Command:    "claude",
		GatewayURL: "http://127.0.0.1:8080",
		GatewayKey: "secret-test-key",
		WorktreeID: "wt-agent-123",
		Env: map[string]string{
			"CUSTOM_VAR": "custom_val",
		},
	}

	env, err := BuildAgentEnv(opts)
	if err != nil {
		t.Fatalf("BuildAgentEnv failed: %v", err)
	}

	envMap := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Verify HOME / USERPROFILE isolation
	if envMap["HOME"] != profDir && envMap["USERPROFILE"] != profDir {
		t.Errorf("expected HOME or USERPROFILE to be %q, got HOME=%q USERPROFILE=%q", profDir, envMap["HOME"], envMap["USERPROFILE"])
	}

	// Verify Gateway URL injection
	if envMap["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:8080/v1" {
		t.Errorf("expected ANTHROPIC_BASE_URL=http://127.0.0.1:8080/v1, got %q", envMap["ANTHROPIC_BASE_URL"])
	}
	if envMap["ANTHROPIC_API_KEY"] != "secret-test-key" {
		t.Errorf("expected ANTHROPIC_API_KEY=secret-test-key, got %q", envMap["ANTHROPIC_API_KEY"])
	}

	// Verify Custom vars & Multigravity metadata
	if envMap["CUSTOM_VAR"] != "custom_val" {
		t.Errorf("expected CUSTOM_VAR=custom_val, got %q", envMap["CUSTOM_VAR"])
	}
	if envMap["MULTIGRAVITY_PROFILE"] != "test-agent-prof" {
		t.Errorf("expected MULTIGRAVITY_PROFILE=test-agent-prof, got %q", envMap["MULTIGRAVITY_PROFILE"])
	}
	if envMap["MULTIGRAVITY_WORKTREE"] != "wt-agent-123" {
		t.Errorf("expected MULTIGRAVITY_WORKTREE=wt-agent-123, got %q", envMap["MULTIGRAVITY_WORKTREE"])
	}
	if envMap["MULTIGRAVITY_AGENT_TYPE"] != "claude" {
		t.Errorf("expected MULTIGRAVITY_AGENT_TYPE=claude, got %q", envMap["MULTIGRAVITY_AGENT_TYPE"])
	}
}

func TestManagerSessionLifecycle(t *testing.T) {
	mgr := NewManager()

	opts := CreateSessionOptions{
		Command: "sh",
		Args:    []string{"-c", "echo 'hello from pty'; read line; echo \"got: $line\""},
		Rows:    24,
		Cols:    80,
	}

	inst, err := mgr.StartSession(opts)
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}

	info := inst.GetInfo()
	if info.Status != StatusRunning {
		t.Fatalf("expected session status running, got %s", info.Status)
	}
	if info.PID <= 0 {
		t.Fatalf("expected valid PID > 0, got %d", info.PID)
	}

	// Subscribe to output
	ch, unsub, err := mgr.SubscribeSession(info.ID)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}
	defer unsub()

	// Wait for initial echo output
	gotInitial := false
	timeout := time.After(3 * time.Second)
waitEcho:
	for {
		select {
		case chunk, ok := <-ch:
			if !ok {
				break waitEcho
			}
			if strings.Contains(chunk.Data, "hello from pty") {
				gotInitial = true
				break waitEcho
			}
		case <-timeout:
			break waitEcho
		}
	}

	if !gotInitial {
		// Also check GetOutput
		out, _ := mgr.GetSessionOutput(info.ID, 0)
		if !strings.Contains(string(out), "hello from pty") {
			t.Fatalf("did not receive expected initial output, got: %q", string(out))
		}
	}

	// Test Resize
	if err := mgr.ResizeSession(info.ID, 30, 100); err != nil {
		t.Errorf("failed to resize session: %v", err)
	}
	info = inst.GetInfo()
	if info.Rows != 30 || info.Cols != 100 {
		t.Errorf("expected rows=30, cols=100, got rows=%d, cols=%d", info.Rows, info.Cols)
	}

	// Write input to session
	if err := mgr.WriteSessionInput(info.ID, []byte("interactive input\n")); err != nil {
		t.Fatalf("failed to write input: %v", err)
	}

	// Wait for response or completion
	inst.Wait(3 * time.Second)

	info = inst.GetInfo()
	if info.Status != StatusExited {
		t.Errorf("expected session to exit, got %s", info.Status)
	}
	if info.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", info.ExitCode)
	}

	// Verify complete output
	out, err := mgr.GetSessionOutput(info.ID, 0)
	if err != nil {
		t.Fatalf("failed to get session output: %v", err)
	}
	outStr := string(out)
	if !strings.Contains(outStr, "got: interactive input") {
		t.Errorf("expected output to contain 'got: interactive input', got %q", outStr)
	}

	// Test ListSessions
	sessions := mgr.ListSessions(SessionFilter{})
	if len(sessions) != 1 {
		t.Errorf("expected 1 session in list, got %d", len(sessions))
	}

	// Test PruneSessions
	pruned := mgr.PruneSessions(1 * time.Nanosecond)
	if pruned != 1 {
		t.Errorf("expected 1 session pruned, got %d", pruned)
	}
	sessions = mgr.ListSessions(SessionFilter{})
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after prune, got %d", len(sessions))
	}
}
