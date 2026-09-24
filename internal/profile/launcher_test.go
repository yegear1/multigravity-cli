package profile

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

func TestBuildLaunchCommand(t *testing.T) {
	tempHome := setupTestHome(t)
	profileName := "launch-test"
	if err := CreateProfile(CreateOptions{Name: profileName}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	fakeApp := filepath.Join(tempHome, "fake-antigravity")
	if runtime.GOOS == "windows" {
		fakeApp += ".exe"
	}
	if err := os.WriteFile(fakeApp, []byte("#!/bin/sh\necho antigravity"), 0755); err != nil {
		t.Fatalf("failed to write fake app: %v", err)
	}

	oldAppRequireFn := appRequireFn
	defer func() { appRequireFn = oldAppRequireFn }()
	appRequireFn = func() (string, error) {
		return fakeApp, nil
	}

	forwardArgs := []string{"--reuse-window", "/workspace/project"}
	cmd, err := BuildLaunchCommand(profileName, forwardArgs)
	if err != nil {
		t.Fatalf("BuildLaunchCommand failed: %v", err)
	}

	profileDir := config.GetProfileDir(profileName)

	// Verify arguments
	cmdArgsStr := strings.Join(cmd.Args, " ")
	if runtime.GOOS == "darwin" {
		if !strings.Contains(cmdArgsStr, "-n") || !strings.Contains(cmdArgsStr, "--args") {
			t.Errorf("expected darwin open -n --args, got: %s", cmdArgsStr)
		}
	} else if runtime.GOOS != "windows" {
		if !strings.Contains(cmdArgsStr, "--user-data-dir") || !strings.Contains(cmdArgsStr, "--extensions-dir") {
			t.Errorf("expected canonical electron flags, got: %s", cmdArgsStr)
		}
	}
	if !strings.Contains(cmdArgsStr, "--reuse-window") || !strings.Contains(cmdArgsStr, "/workspace/project") {
		t.Errorf("expected forwarded args, got: %s", cmdArgsStr)
	}

	// Verify environment isolation
	envMap := make(map[string]string)
	for _, env := range cmd.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	if envMap["HOME"] != profileDir {
		t.Errorf("expected HOME=%s, got %s", profileDir, envMap["HOME"])
	}

	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		expectedXDG := filepath.Join(profileDir, ".config")
		if envMap["XDG_CONFIG_HOME"] != expectedXDG {
			t.Errorf("expected XDG_CONFIG_HOME=%s, got %s", expectedXDG, envMap["XDG_CONFIG_HOME"])
		}
	}
}

func TestBuildLaunchCommandNonexistent(t *testing.T) {
	_ = setupTestHome(t)
	_, err := BuildLaunchCommand("nonexistent", nil)
	if err == nil {
		t.Fatalf("expected error when profile does not exist")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLaunchProfileWaitFlag(t *testing.T) {
	tempHome := setupTestHome(t)
	profileName := "wait-prof"
	if err := CreateProfile(CreateOptions{Name: profileName}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	fakeApp := filepath.Join(tempHome, "fake-antigravity")
	if runtime.GOOS == "windows" {
		fakeApp += ".exe"
	}
	_ = os.WriteFile(fakeApp, []byte("#!/bin/sh\necho antigravity"), 0755)

	oldAppRequireFn := appRequireFn
	oldRunCmdFn := runCmdFn
	defer func() {
		appRequireFn = oldAppRequireFn
		runCmdFn = oldRunCmdFn
	}()

	appRequireFn = func() (string, error) {
		return fakeApp, nil
	}

	var ranWait bool
	runCmdFn = func(c *exec.Cmd, wait bool) error {
		ranWait = wait
		return nil
	}

	if err := LaunchProfile(profileName, []string{"--wait", "file.txt"}); err != nil {
		t.Fatalf("LaunchProfile failed: %v", err)
	}
	if !ranWait {
		t.Errorf("expected runCmdFn to receive wait=true when --wait is passed")
	}

	if err := LaunchProfile(profileName, []string{"file.txt"}); err != nil {
		t.Fatalf("LaunchProfile failed: %v", err)
	}
	if ranWait {
		t.Errorf("expected runCmdFn to receive wait=false when no wait flag is passed")
	}
}
