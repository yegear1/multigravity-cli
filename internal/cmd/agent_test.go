package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/agent"
)

func TestAgentCLICommands(t *testing.T) {
	tempHome := t.TempDir()
	profDir := filepath.Join(tempHome, "AntigravityProfiles", "test-cli-prof")
	if err := os.MkdirAll(profDir, 0755); err != nil {
		t.Fatalf("failed to create profile dir: %v", err)
	}

	t.Setenv("MULTIGRAVITY_HOME", filepath.Join(tempHome, "AntigravityProfiles"))
	t.Setenv("REAL_HOME", tempHome)

	// 1. Test Run with Detach and JSON
	cmd := newAgentCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	agJSON = true
	agDetach = true
	defer func() {
		agJSON = false
		agDetach = false
	}()

	cmd.SetArgs([]string{"run", "test-cli-prof", "--detach", "--json", "--", "sh", "-c", "echo 'agent CLI ok'; sleep 2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agent run --detach failed: %v", err)
	}

	// Capture session created from manager
	mgr := agent.GetDefaultManager()
	sessions := mgr.ListSessions(agent.SessionFilter{Profile: "test-cli-prof"})
	if len(sessions) == 0 {
		t.Fatalf("expected at least 1 session tracked by manager")
	}
	sessionID := sessions[0].ID

	// 2. Test List with JSON
	listCmd := newAgentCmd()
	listBuf := new(bytes.Buffer)
	listCmd.SetOut(listBuf)
	listCmd.SetErr(listBuf)
	agJSON = true
	listCmd.SetArgs([]string{"list", "--json"})
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("agent list --json failed: %v", err)
	}

	// 3. Test Status
	statusCmd := newAgentCmd()
	statusBuf := new(bytes.Buffer)
	statusCmd.SetOut(statusBuf)
	statusCmd.SetErr(statusBuf)
	agJSON = true
	statusCmd.SetArgs([]string{"status", sessionID, "--json"})
	if err := statusCmd.Execute(); err != nil {
		t.Fatalf("agent status failed: %v", err)
	}

	// 4. Test Logs
	// Wait a moment for process to echo
	time.Sleep(100 * time.Millisecond)
	logsCmd := newAgentCmd()
	logsBuf := new(bytes.Buffer)
	logsCmd.SetOut(logsBuf)
	logsCmd.SetErr(logsBuf)
	agJSON = false
	agTail = 100
	agFollow = false
	logsCmd.SetArgs([]string{"logs", sessionID})
	if err := logsCmd.Execute(); err != nil {
		t.Fatalf("agent logs failed: %v", err)
	}

	// 5. Test Stop
	stopCmd := newAgentCmd()
	stopBuf := new(bytes.Buffer)
	stopCmd.SetOut(stopBuf)
	stopCmd.SetErr(stopBuf)
	stopCmd.SetArgs([]string{"stop", sessionID})
	if err := stopCmd.Execute(); err != nil {
		t.Fatalf("agent stop failed: %v", err)
	}

	// Verify session exited or stopped
	inst, err := mgr.GetSession(sessionID)
	if err != nil {
		t.Fatalf("failed to find session: %v", err)
	}
	inst.Wait(1 * time.Second)
	info := inst.GetInfo()
	if info.Status != agent.StatusExited && info.Status != agent.StatusStopped {
		t.Errorf("expected session to be stopped or exited, got %s", info.Status)
	}
}

func TestAgentCLINonJSON(t *testing.T) {
	tempHome := t.TempDir()
	profDir := filepath.Join(tempHome, "AntigravityProfiles", "nonjson-prof")
	_ = os.MkdirAll(profDir, 0755)

	t.Setenv("MULTIGRAVITY_HOME", filepath.Join(tempHome, "AntigravityProfiles"))
	t.Setenv("REAL_HOME", tempHome)

	cmd := newAgentCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	agJSON = false
	agDetach = true
	cmd.SetArgs([]string{"run", "nonjson-prof", "-d", "--", "echo", "quick output"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agent run failed: %v", err)
	}

	listCmd := newAgentCmd()
	listBuf := new(bytes.Buffer)
	listCmd.SetOut(listBuf)
	listCmd.SetErr(listBuf)
	agJSON = false
	listCmd.SetArgs([]string{"list"})
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("agent list failed: %v", err)
	}
}

func parseJSONOrIgnore(t *testing.T, data []byte, v interface{}) {
	if len(data) > 0 {
		_ = json.Unmarshal(data, v)
	}
}

func checkSubstring(t *testing.T, got, want string) {
	if !strings.Contains(got, want) {
		t.Errorf("expected substring %q in %q", want, got)
	}
}
