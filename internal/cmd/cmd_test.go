package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err = root.Execute()
	return buf.String(), err
}

func TestCobraNewDeleteRename(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", t.TempDir())

	// Test new
	_, err := executeCommand(rootCmd, "new", "cli-test-profile", "--shared", "--isolated-dotfiles")
	if err != nil {
		t.Fatalf("expected new command to succeed, got: %v", err)
	}

	profileDir := filepath.Join(tempHome, "cli-test-profile")
	if !fileExists(profileDir) {
		t.Fatalf("expected profile dir %s to exist", profileDir)
	}

	// Test rename
	_, err = executeCommand(rootCmd, "rename", "cli-test-profile", "renamed-profile")
	if err != nil {
		t.Fatalf("expected rename command to succeed, got: %v", err)
	}

	renamedDir := filepath.Join(tempHome, "renamed-profile")
	if !fileExists(renamedDir) {
		t.Fatalf("expected renamed dir %s to exist", renamedDir)
	}
	if fileExists(profileDir) {
		t.Fatalf("expected old dir %s to not exist", profileDir)
	}

	// Test delete with force
	_, err = executeCommand(rootCmd, "delete", "renamed-profile", "--force")
	if err != nil {
		t.Fatalf("expected delete command to succeed, got: %v", err)
	}

	if fileExists(renamedDir) {
		t.Fatalf("expected deleted dir %s to not exist", renamedDir)
	}
}

func TestCobraStopRestartClean(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", t.TempDir())

	// Create profile for testing
	_, err := executeCommand(rootCmd, "new", "svc-profile")
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	// Test stop on idle profile
	out, err := executeCommand(rootCmd, "stop", "svc-profile")
	if err != nil {
		t.Fatalf("expected stop on idle profile to succeed, got: %v", err)
	}
	_ = out

	// Test stop with --force
	_, err = executeCommand(rootCmd, "stop", "svc-profile", "--force")
	if err != nil {
		t.Fatalf("expected stop --force to succeed, got: %v", err)
	}

	// Test restart
	fakeApp := filepath.Join(tempHome, "fake-antigravity")
	if runtime.GOOS == "windows" {
		fakeApp += ".exe"
	}
	_ = os.WriteFile(fakeApp, []byte("#!/bin/sh\necho ok"), 0755)
	t.Setenv("MULTIGRAVITY_APP", fakeApp)

	_, err = executeCommand(rootCmd, "restart", "svc-profile")
	if err != nil {
		t.Fatalf("expected restart to succeed, got: %v", err)
	}

	// Test clean single profile
	_, err = executeCommand(rootCmd, "clean", "svc-profile")
	if err != nil {
		t.Fatalf("expected clean single profile to succeed, got: %v", err)
	}

	// Test clean --all
	_, err = executeCommand(rootCmd, "clean", "--all")
	if err != nil {
		t.Fatalf("expected clean --all to succeed, got: %v", err)
	}
}

func TestCobraDirectProfileLaunch(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", t.TempDir())

	fakeApp := filepath.Join(tempHome, "fake-antigravity")
	if runtime.GOOS == "windows" {
		fakeApp += ".exe"
	}
	if err := os.WriteFile(fakeApp, []byte("#!/bin/sh\necho running"), 0755); err != nil {
		t.Fatalf("failed to write fake app: %v", err)
	}
	t.Setenv("MULTIGRAVITY_APP", fakeApp)

	// Create profile
	_, err := executeCommand(rootCmd, "new", "direct-target")
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	// Launch profile with arbitrary forward arguments
	out, err := executeCommand(rootCmd, "direct-target", "--reuse-window", "/some/path/to/folder")
	if err != nil {
		t.Fatalf("expected direct profile launch to succeed, got: %v, out: %s", err, out)
	}

	// Launch non-existent profile
	_, err = executeCommand(rootCmd, "nonexistent-profile")
	if err == nil {
		t.Fatalf("expected direct profile launch for nonexistent profile to fail")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("unexpected error message: %v", err)
	}

	// Invalid command / profile name
	_, err = executeCommand(rootCmd, "invalid name with spaces")
	if err == nil {
		t.Fatalf("expected error for invalid command/profile name")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
