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

func TestCobraColorAndSharingCommands(t *testing.T) {
	tempHome := t.TempDir()
	hostHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)
	t.Setenv("REAL_HOME", hostHome)
	t.Setenv("HOME", hostHome)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", t.TempDir())

	// Create host dummy files
	hostMCP := filepath.Join(hostHome, ".gemini", "config", "mcp_config.json")
	_ = os.MkdirAll(filepath.Dir(hostMCP), 0755)
	_ = os.WriteFile(hostMCP, []byte(`{}`), 0644)

	hostSkills := filepath.Join(hostHome, ".gemini", "config", "skills")
	_ = os.MkdirAll(hostSkills, 0755)

	hostConfig := filepath.Join(hostHome, ".gemini", "config", "config.json")
	_ = os.WriteFile(hostConfig, []byte(`{}`), 0644)

	hostGH := filepath.Join(hostHome, ".config", "gh")
	_ = os.MkdirAll(hostGH, 0755)

	// 1. Test new with --color
	_, err := executeCommand(rootCmd, "new", "themed-profile", "--color", "blue")
	if err != nil {
		t.Fatalf("expected new with --color to succeed: %v", err)
	}

	// 2. Test color command: get color
	out, err := executeCommand(rootCmd, "color", "themed-profile")
	if err != nil {
		t.Fatalf("expected color get to succeed: %v", err)
	}
	if !strings.Contains(out, "#1e40af") {
		t.Errorf("expected color output to contain #1e40af, got: %s", out)
	}

	// 3. Test color command: change color
	out, err = executeCommand(rootCmd, "color", "themed-profile", "red")
	if err != nil {
		t.Fatalf("expected color change to succeed: %v", err)
	}
	if !strings.Contains(out, "Set theme color") {
		t.Errorf("expected confirmation message, got: %s", out)
	}

	// 4. Test color command: clear color
	out, err = executeCommand(rootCmd, "color", "themed-profile", "--clear")
	if err != nil {
		t.Fatalf("expected color clear to succeed: %v", err)
	}
	if !strings.Contains(out, "Cleared color customizations") {
		t.Errorf("expected cleared message, got: %s", out)
	}

	// 5. Test mcp status and isolate/share
	out, err = executeCommand(rootCmd, "mcp", "status", "themed-profile")
	if err != nil {
		t.Fatalf("expected mcp status to succeed: %v", err)
	}
	if !strings.Contains(out, "shares host MCP servers") {
		t.Errorf("unexpected mcp status: %s", out)
	}

	_, err = executeCommand(rootCmd, "mcp", "isolate", "themed-profile")
	if err != nil {
		t.Fatalf("expected mcp isolate to succeed: %v", err)
	}
	_, err = executeCommand(rootCmd, "mcp", "share", "themed-profile")
	if err != nil {
		t.Fatalf("expected mcp share to succeed: %v", err)
	}

	// 6. Test skills status and isolate/share
	out, err = executeCommand(rootCmd, "skills", "status", "themed-profile")
	if err != nil {
		t.Fatalf("expected skills status to succeed: %v", err)
	}
	_, err = executeCommand(rootCmd, "skills", "isolate", "themed-profile")
	if err != nil {
		t.Fatalf("expected skills isolate to succeed: %v", err)
	}
	_, err = executeCommand(rootCmd, "skills", "share", "themed-profile")
	if err != nil {
		t.Fatalf("expected skills share to succeed: %v", err)
	}

	// 7. Test config status, isolate, share, seed, allow-readonly
	out, err = executeCommand(rootCmd, "config", "status", "themed-profile")
	if err != nil {
		t.Fatalf("expected config status to succeed: %v", err)
	}
	_, err = executeCommand(rootCmd, "config", "isolate", "themed-profile")
	if err != nil {
		t.Fatalf("expected config isolate to succeed: %v", err)
	}
	_, err = executeCommand(rootCmd, "config", "seed", "themed-profile")
	if err != nil {
		t.Fatalf("expected config seed to succeed: %v", err)
	}
	_, err = executeCommand(rootCmd, "allow-readonly", "--host")
	if err != nil {
		t.Fatalf("expected allow-readonly --host to succeed: %v", err)
	}
	_, err = executeCommand(rootCmd, "config", "share", "themed-profile")
	if err != nil {
		t.Fatalf("expected config share to succeed: %v", err)
	}

	// 8. Test gh status, isolate, share
	out, err = executeCommand(rootCmd, "gh", "status", "themed-profile")
	if err != nil {
		t.Fatalf("expected gh status to succeed: %v", err)
	}
	_, err = executeCommand(rootCmd, "gh", "isolate", "themed-profile")
	if err != nil {
		t.Fatalf("expected gh isolate to succeed: %v", err)
	}
	_, err = executeCommand(rootCmd, "gh", "share", "themed-profile")
	if err != nil {
		t.Fatalf("expected gh share to succeed: %v", err)
	}
}

func TestCobraCloneTemplateExportImportStats(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", t.TempDir())

	// Create base profile
	_, err := executeCommand(rootCmd, "new", "original-prof")
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	// 1. Test clone
	out, err := executeCommand(rootCmd, "clone", "original-prof", "cloned-prof")
	if err != nil {
		t.Fatalf("clone command failed: %v (out: %s)", err, out)
	}
	if !fileExists(filepath.Join(tempHome, "cloned-prof")) {
		t.Fatalf("expected cloned profile dir to exist")
	}

	// 2. Test template save, list, delete
	out, err = executeCommand(rootCmd, "template", "save", "original-prof", "my-tpl")
	if err != nil {
		t.Fatalf("template save failed: %v (out: %s)", err, out)
	}

	out, err = executeCommand(rootCmd, "template", "list")
	if err != nil || !strings.Contains(out, "my-tpl") {
		t.Fatalf("template list failed or missing my-tpl: %v (out: %s)", err, out)
	}

	// Create new profile from template
	out, err = executeCommand(rootCmd, "new", "tpl-born", "--template", "my-tpl")
	if err != nil {
		t.Fatalf("new with --template failed: %v (out: %s)", err, out)
	}
	if !fileExists(filepath.Join(tempHome, "tpl-born")) {
		t.Fatalf("expected tpl-born profile dir to exist")
	}

	// Delete template
	out, err = executeCommand(rootCmd, "template", "delete", "my-tpl")
	if err != nil {
		t.Fatalf("template delete failed: %v (out: %s)", err, out)
	}

	// 3. Test export
	archivePath := filepath.Join(t.TempDir(), "exported.tar.gz")
	out, err = executeCommand(rootCmd, "export", "original-prof", archivePath, "--include-cache")
	if err != nil {
		t.Fatalf("export failed: %v (out: %s)", err, out)
	}
	if !fileExists(archivePath) {
		t.Fatalf("expected export archive %s to exist", archivePath)
	}

	// 4. Test import
	out, err = executeCommand(rootCmd, "import", archivePath, "restored-prof")
	if err != nil {
		t.Fatalf("import failed: %v (out: %s)", err, out)
	}
	if !fileExists(filepath.Join(tempHome, "restored-prof")) {
		t.Fatalf("expected restored profile dir to exist")
	}

	// 5. Test stats
	out, err = executeCommand(rootCmd, "stats")
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if !strings.Contains(out, "PROFILE") || !strings.Contains(out, "original-prof") || !strings.Contains(out, "Total usage:") {
		t.Fatalf("stats output format unexpected: %s", out)
	}
}

