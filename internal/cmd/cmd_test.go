package cmd

import (
	"bytes"
	"os"
	"path/filepath"
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

	// Test stop with --force
	_, err = executeCommand(rootCmd, "stop", "svc-profile", "--force")
	if err != nil {
		t.Fatalf("expected stop --force to succeed, got: %v", err)
	}

	// Test restart
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

	_ = out
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
