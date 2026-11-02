package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/profile"
)

func setupAgentTestEnv(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "multigravity-agent-runner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	origHome := os.Getenv("MULTIGRAVITY_HOME")
	_ = os.Setenv("MULTIGRAVITY_HOME", tmpDir)

	cleanup := func() {
		if origHome != "" {
			_ = os.Setenv("MULTIGRAVITY_HOME", origHome)
		} else {
			_ = os.Unsetenv("MULTIGRAVITY_HOME")
		}
		_ = os.RemoveAll(tmpDir)
	}
	return tmpDir, cleanup
}

func TestNormalizeAgyArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "strips --headless flag",
			input:    []string{"--headless", "-p", "fix bug"},
			expected: []string{"-p", "fix bug"},
		},
		{
			name:     "strips --headless=true flag",
			input:    []string{"--headless=true", "-p", "fix bug", "--model", "gemini-3.8-flash-tiered"},
			expected: []string{"-p", "fix bug", "--model", "gemini-3.8-flash-tiered"},
		},
		{
			name:     "strips --headless=false flag",
			input:    []string{"-p", "test", "--headless=false"},
			expected: []string{"-p", "test"},
		},
		{
			name:     "normalizes model and effort combined",
			input:    []string{"--headless", "--model=gemini-2.5-flash-medium", "--effort=low"},
			expected: []string{"--model=gemini-2.5-flash-low"},
		},
		{
			name:     "normalizes model and effort separate",
			input:    []string{"--headless", "--model", "gemini-3.8-flash-tiered", "--effort", "medium"},
			expected: []string{"--model", "gemini-3.8-flash-medium"},
		},
		{
			name:     "leaves standard agy flags intact",
			input:    []string{"-p", "hello", "--dangerously-skip-permissions", "--disable-slash-commands"},
			expected: []string{"-p", "hello", "--dangerously-skip-permissions", "--disable-slash-commands"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeAgyArgs(tc.input)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("NormalizeAgyArgs(%v) = %v; want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestEnsureProfileReady(t *testing.T) {
	_, cleanupEnv := setupAgentTestEnv(t)
	defer cleanupEnv()

	profName := "worker-prof"
	if err := profile.CreateProfile(profile.CreateOptions{Name: profName}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	// 1. EnsureProfileReady on non-existent profile fails
	err := EnsureProfileReady("non-existent-prof")
	if err == nil {
		t.Fatal("expected error for non-existent profile")
	}

	// 2. Custom hook tracks awakening of inactive profile
	awakened := false
	restore := SetRunnerTestHooks(
		nil,
		nil,
		func(p string) error {
			if p == profName {
				awakened = true
			}
			return nil
		},
		nil,
	)
	defer restore()

	if err := EnsureProfileReady(profName); err != nil {
		t.Fatalf("unexpected error from EnsureProfileReady: %v", err)
	}
	if !awakened {
		t.Errorf("expected profile %q to be awakened", profName)
	}
}

func TestRunHeadlessSubagent_Agy(t *testing.T) {
	_, cleanupEnv := setupAgentTestEnv(t)
	defer cleanupEnv()

	profName := "agy-runner-prof"
	if err := profile.CreateProfile(profile.CreateOptions{Name: profName}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	ranAgy := false
	var capturedArgs []string
	restore := SetRunnerTestHooks(
		func() (string, error) {
			return "/fake/bin/agy", nil
		},
		func(ctx context.Context, bin string, args []string, env []string, dir string) ([]byte, int, error) {
			ranAgy = true
			capturedArgs = args
			respJSON := `{"response": "Task completed successfully", "usage": {"total_tokens": 150}, "duration_seconds": 1.25}`
			return []byte(respJSON), 0, nil
		},
		func(p string) error { return nil },
		nil,
	)
	defer restore()

	res, err := RunHeadlessSubagent(context.Background(), HeadlessSubagentOptions{
		Profile:                    profName,
		Prompt:                     "implement feature x",
		Model:                      "gemini-3.8-flash-medium",
		DangerouslySkipPermissions: true,
		DisableSlashCommands:       true,
	})
	if err != nil {
		t.Fatalf("RunHeadlessSubagent failed: %v", err)
	}

	if !ranAgy {
		t.Fatal("expected agy runner to be executed")
	}
	if res.UsedFallback {
		t.Fatal("did not expect fallback to cascade RPC")
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res.ExitCode)
	}
	if res.Response != "Task completed successfully" {
		t.Errorf("unexpected response: %q", res.Response)
	}
	if res.TotalTokens != 150 {
		t.Errorf("expected 150 total tokens, got %d", res.TotalTokens)
	}

	// Verify args contain normalized flags
	hasSkipPerms := false
	hasModel := false
	for _, a := range capturedArgs {
		if a == "--headless" {
			t.Errorf("unexpected --headless in agy arguments")
		}
		if a == "--dangerously-skip-permissions" {
			hasSkipPerms = true
		}
		if a == "gemini-3.8-flash-medium" {
			hasModel = true
		}
	}
	if !hasSkipPerms {
		t.Errorf("expected --dangerously-skip-permissions in agy args: %v", capturedArgs)
	}
	if !hasModel {
		t.Errorf("expected model in agy args: %v", capturedArgs)
	}
}

func TestRunHeadlessSubagent_CascadeFallback(t *testing.T) {
	_, cleanupEnv := setupAgentTestEnv(t)
	defer cleanupEnv()

	profName := "cascade-fallback-prof"
	if err := profile.CreateProfile(profile.CreateOptions{Name: profName}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	ranCascade := false
	restore := SetRunnerTestHooks(
		func() (string, error) {
			// agy not installed
			return "", errors.New("agy not found")
		},
		nil,
		func(p string) error { return nil },
		func(ctx context.Context, profileName, prompt, model string, timeout time.Duration) (string, int, error) {
			ranCascade = true
			if profileName != profName {
				return "", 1, fmt.Errorf("unexpected profile: %s", profileName)
			}
			return "Cascade RPC message sent to session 123", 0, nil
		},
	)
	defer restore()

	res, err := RunHeadlessSubagent(context.Background(), HeadlessSubagentOptions{
		Profile: profName,
		Prompt:  "research architecture",
		Model:   "gemini-2.5-pro",
	})
	if err != nil {
		t.Fatalf("RunHeadlessSubagent failed: %v", err)
	}

	if !ranCascade {
		t.Fatal("expected fallback to cascade RPC when agy is absent")
	}
	if !res.UsedFallback {
		t.Fatal("expected res.UsedFallback to be true")
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res.ExitCode)
	}
	if !strings.Contains(res.Response, "Cascade RPC message sent") {
		t.Errorf("unexpected cascade response: %s", res.Response)
	}
}
