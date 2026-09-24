package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/ye-dev/multigravity-cli/internal/chat"
	"github.com/ye-dev/multigravity-cli/internal/doctor"
	"github.com/ye-dev/multigravity-cli/internal/profile"
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

func TestCobraQuotaPrimeAI(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", tempHome)

	_, _ = executeCommand(rootCmd, "new", "ai-test-prof")

	// 1. Test quota on inactive profile
	out, err := executeCommand(rootCmd, "quota", "ai-test-prof")
	if err != nil {
		t.Fatalf("expected quota command to handle inactive profile gracefully: %v", err)
	}
	if !strings.Contains(out, "is not running") {
		t.Errorf("expected 'is not running' in quota output, got: %s", out)
	}

	// 2. Test ai quota subroute
	out, err = executeCommand(rootCmd, "ai", "quota", "ai-test-prof")
	if err != nil {
		t.Fatalf("expected ai quota command to succeed: %v", err)
	}
	if !strings.Contains(out, "is not running") {
		t.Errorf("expected 'is not running' in ai quota output, got: %s", out)
	}

	// 3. Test quota on nonexistent profile
	_, err = executeCommand(rootCmd, "quota", "nonexistent-prof")
	if err == nil {
		t.Fatalf("expected error for nonexistent profile in quota")
	}

	// 4. Test ai list
	out, err = executeCommand(rootCmd, "ai", "list", "ai-test-prof")
	if err != nil {
		t.Fatalf("ai list failed: %v", err)
	}
	if !strings.Contains(out, "has no saved AI chats") {
		t.Errorf("expected empty chats message, got: %s", out)
	}

	out, err = executeCommand(rootCmd, "ai", "list", "ai-test-prof", "--active")
	if err != nil || !strings.Contains(out, "has no active AI chats") {
		t.Errorf("expected active chats empty message, got: %s (err: %v)", out, err)
	}

	out, err = executeCommand(rootCmd, "ai", "list", "ai-test-prof", "--archived")
	if err != nil || !strings.Contains(out, "has no archived AI chats") {
		t.Errorf("expected archived chats empty message, got: %s (err: %v)", out, err)
	}

	out, err = executeCommand(rootCmd, "ai", "list", "ai-test-prof", "--json")
	if err != nil || !strings.Contains(out, "[]") {
		t.Errorf("expected empty json array for ai list --json, got: %s (err: %v)", out, err)
	}

	// 5. Test prime flag verification
	out, err = executeCommand(rootCmd, "prime", "--help")
	if err != nil {
		t.Fatalf("prime --help failed: %v", err)
	}
	if !strings.Contains(out, "--5h") || !strings.Contains(out, "--status") || !strings.Contains(out, "--no-jitter") {
		t.Errorf("prime flags missing in help: %s", out)
	}

	// 6. Test ai help
	out, err = executeCommand(rootCmd, "ai", "--help")
	if err != nil {
		t.Fatalf("ai --help failed: %v", err)
	}
	if !strings.Contains(out, "export") || !strings.Contains(out, "quota") || !strings.Contains(out, "prime") {
		t.Errorf("ai subcommands missing in help: %s", out)
	}
}

func TestDoctorCmd(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)

	fakeApp := filepath.Join(tempHome, "fake-agy")
	if err := os.WriteFile(fakeApp, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MULTIGRAVITY_APP", fakeApp)

	out, err := executeCommand(rootCmd, "doctor")
	if err != nil {
		t.Fatalf("expected doctor command to succeed: %v", err)
	}
	if !strings.Contains(out, "Checking multigravity environment...") {
		t.Errorf("expected header, got: %s", out)
	}
	if !strings.Contains(out, "Platform:") {
		t.Errorf("expected platform, got: %s", out)
	}
}

func TestCompletionCmd(t *testing.T) {
	// 1. Test completion help (no args)
	out, err := executeCommand(rootCmd, "completion")
	if err != nil {
		t.Fatalf("expected completion help to succeed: %v", err)
	}
	if !strings.Contains(out, "To enable autocompletion") {
		t.Errorf("expected autocompletion help text, got: %s", out)
	}

	// 2. Test bash completion
	out, err = executeCommand(rootCmd, "completion", "bash")
	if err != nil {
		t.Fatalf("expected bash completion generation to succeed: %v", err)
	}
	if !strings.Contains(out, "bash completion") && !strings.Contains(out, "multigravity") {
		t.Errorf("expected bash completion script, got: %s", out)
	}

	// 3. Test zsh completion
	out, err = executeCommand(rootCmd, "completion", "zsh")
	if err != nil {
		t.Fatalf("expected zsh completion generation to succeed: %v", err)
	}
	if !strings.Contains(out, "compdef") && !strings.Contains(out, "multigravity") {
		t.Errorf("expected zsh completion script, got: %s", out)
	}

	// 4. Test fish completion
	out, err = executeCommand(rootCmd, "completion", "fish")
	if err != nil {
		t.Fatalf("expected fish completion generation to succeed: %v", err)
	}
	if len(out) == 0 {
		t.Errorf("expected non-empty fish completion script")
	}

	// 5. Test powershell completion
	out, err = executeCommand(rootCmd, "completion", "powershell")
	if err != nil {
		t.Fatalf("expected powershell completion generation to succeed: %v", err)
	}
	if !strings.Contains(out, "Register-ArgumentCompleter") && !strings.Contains(out, "multigravity") {
		t.Errorf("expected powershell completion script, got: %s", out)
	}

	// 6. Test invalid shell
	_, err = executeCommand(rootCmd, "completion", "unknown-shell")
	if err == nil {
		t.Fatalf("expected error for unknown shell")
	}
	if !strings.Contains(err.Error(), "unsupported shell type") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRootCmdInteractiveAndCompletion(t *testing.T) {
	tempHome := t.TempDir()
	profilesDir := filepath.Join(tempHome, "profiles")
	_ = os.MkdirAll(profilesDir, 0755)
	shortcutsDir := filepath.Join(tempHome, "shortcuts")
	_ = os.MkdirAll(shortcutsDir, 0755)
	t.Setenv("MULTIGRAVITY_HOME", profilesDir)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", shortcutsDir)

	if err := profile.CreateProfile(profile.CreateOptions{Name: "c-prof1"}); err != nil {
		t.Fatal(err)
	}
	if err := profile.CreateProfile(profile.CreateOptions{Name: "c-prof2"}); err != nil {
		t.Fatal(err)
	}

	// 1. Test profileArgsCompletion
	comps, directive := profileArgsCompletion(rootCmd, []string{}, "")
	if len(comps) != 2 || comps[0] != "c-prof1" || comps[1] != "c-prof2" {
		t.Errorf("unexpected profile completions: %v", comps)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("unexpected directive: %v", directive)
	}

	// 2. Test cleanCmd.ValidArgsFunction
	cleanComps, _ := cleanCmd.ValidArgsFunction(cleanCmd, []string{}, "")
	if len(cleanComps) != 3 || cleanComps[2] != "--all" {
		t.Errorf("unexpected clean completions: %v", cleanComps)
	}

	// 3. Test mcpCmd.ValidArgsFunction
	mcpActions, _ := mcpCmd.ValidArgsFunction(mcpCmd, []string{}, "")
	if len(mcpActions) != 3 || mcpActions[0] != "status" {
		t.Errorf("unexpected mcp action completions: %v", mcpActions)
	}
	mcpProfiles, _ := mcpCmd.ValidArgsFunction(mcpCmd, []string{"status"}, "")
	if len(mcpProfiles) != 2 {
		t.Errorf("unexpected mcp profile completions: %v", mcpProfiles)
	}

	// 4. Test non-interactive rootCmd without args (should show help and return error)
	oldInteractive := isInteractiveTerminal
	defer func() { isInteractiveTerminal = oldInteractive }()
	isInteractiveTerminal = func() bool { return false }

	out, err := executeCommand(rootCmd)
	if err == nil {
		t.Fatalf("expected error on non-interactive rootCmd without args")
	}
	if !strings.Contains(out, "Usage:") {
		t.Errorf("expected usage output, got: %s", out)
	}

	// 5. Test interactive rootCmd without args (mock interactive terminal returning quit "q")
	isInteractiveTerminal = func() bool { return true }
	rootCmd.SetIn(bytes.NewBufferString("q\n"))
	out, err = executeCommand(rootCmd)
	if err != nil {
		t.Fatalf("expected interactive menu to quit cleanly: %v", err)
	}
	if !strings.Contains(out, "MULTIGRAVITY PROFILES") {
		t.Errorf("expected TUI menu header in output, got: %s", out)
	}
}

func TestCommandAliases(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", t.TempDir())

	// 1. Test create (alias for new)
	_, err := executeCommand(rootCmd, "create", "alias-test", "--shared")
	if err != nil {
		t.Fatalf("expected create alias to succeed: %v", err)
	}

	// 2. Test ls (alias for list)
	out, err := executeCommand(rootCmd, "ls", "--raw")
	if err != nil {
		t.Fatalf("expected ls alias to succeed: %v", err)
	}
	if !strings.Contains(out, "alias-test") {
		t.Errorf("expected alias-test in ls output: %s", out)
	}

	// 3. Test cp (alias for clone)
	_, err = executeCommand(rootCmd, "cp", "alias-test", "cloned-alias")
	if err != nil {
		t.Fatalf("expected cp alias to succeed: %v", err)
	}

	// 4. Test mv (alias for rename)
	_, err = executeCommand(rootCmd, "mv", "cloned-alias", "renamed-alias")
	if err != nil {
		t.Fatalf("expected mv alias to succeed: %v", err)
	}

	// 5. Test rm (alias for delete)
	_, err = executeCommand(rootCmd, "rm", "renamed-alias", "--force")
	if err != nil {
		t.Fatalf("expected rm alias to succeed: %v", err)
	}
}

func TestUpdateCommand(t *testing.T) {
	t.Setenv("MULTIGRAVITY_REPO", "non-existent-repo-for-test-xyz123")
	_, err := executeCommand(rootCmd, "update")
	if err == nil {
		t.Fatalf("expected update to fail on non-existent repo")
	}
}

func TestJSONContractOutputs(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", t.TempDir())

	// 1. Create a profile for testing
	_, err := executeCommand(rootCmd, "new", "json-test-prof", "--shared")
	if err != nil {
		t.Fatalf("failed to create test profile: %v", err)
	}

	// 2. Test list --json
	out, err := executeCommand(rootCmd, "list", "--json")
	if err != nil {
		t.Fatalf("list --json failed: %v", err)
	}
	var profiles []profile.ProfileInfo
	if err := json.Unmarshal([]byte(out), &profiles); err != nil {
		t.Fatalf("failed to parse list --json output: %v, raw output: %s", err, out)
	}
	if len(profiles) != 1 || profiles[0].Name != "json-test-prof" {
		t.Errorf("expected 1 profile named json-test-prof, got: %+v", profiles)
	}

	// 3. Test stats --json
	out, err = executeCommand(rootCmd, "stats", "--json")
	if err != nil {
		t.Fatalf("stats --json failed: %v", err)
	}
	var statsReport profile.ProfileStatsReport
	if err := json.Unmarshal([]byte(out), &statsReport); err != nil {
		t.Fatalf("failed to parse stats --json output: %v, raw output: %s", err, out)
	}
	if len(statsReport.Profiles) != 1 || statsReport.Profiles[0].Name != "json-test-prof" {
		t.Errorf("expected 1 profile stat for json-test-prof, got: %+v", statsReport)
	}

	// 4. Test doctor --json
	out, err = executeCommand(rootCmd, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor --json failed: %v", err)
	}
	var docReport doctor.DiagnosticReport
	if err := json.Unmarshal([]byte(out), &docReport); err != nil {
		t.Fatalf("failed to parse doctor --json output: %v, raw output: %s", err, out)
	}
	if docReport.Platform == "" || len(docReport.Checks) == 0 {
		t.Errorf("expected valid doctor report, got: %+v", docReport)
	}

	// 5. Test quota --json (without running server, should return empty array JSON)
	out, err = executeCommand(rootCmd, "quota", "--json")
	if err != nil {
		t.Fatalf("quota --json failed: %v", err)
	}
	var quotaServers []interface{}
	if err := json.Unmarshal([]byte(out), &quotaServers); err != nil {
		t.Fatalf("failed to parse quota --json output: %v, raw output: %s", err, out)
	}

	// 6. Test mcp status <profile> --json
	out, err = executeCommand(rootCmd, "mcp", "status", "json-test-prof", "--json")
	if err != nil {
		t.Fatalf("mcp status --json failed: %v", err)
	}
	var mcpStatus profile.SharingStatus
	if err := json.Unmarshal([]byte(out), &mcpStatus); err != nil {
		t.Fatalf("failed to parse mcp status --json output: %v, raw output: %s", err, out)
	}
	if mcpStatus.Profile != "json-test-prof" || mcpStatus.Resource != "mcp" {
		t.Errorf("unexpected mcp status: %+v", mcpStatus)
	}

	// 7. Test ai list <profile> --json
	out, err = executeCommand(rootCmd, "ai", "list", "json-test-prof", "--json")
	if err != nil {
		t.Fatalf("ai list --json failed: %v", err)
	}
	var convs []chat.ConversationInfo
	if err := json.Unmarshal([]byte(out), &convs); err != nil {
		t.Fatalf("failed to parse ai list --json output: %v, raw output: %s", err, out)
	}
	if len(convs) != 0 {
		t.Errorf("expected 0 conversations for new profile, got: %+v", convs)
	}
}

func TestCobraAICmdFlags(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", t.TempDir())

	_ = profile.CreateProfile(profile.CreateOptions{Name: "ai-src"})
	_ = profile.CreateProfile(profile.CreateOptions{Name: "ai-dest"})

	geminiDir := filepath.Join(tempHome, "ai-src", ".gemini", "antigravity")
	convDir := filepath.Join(geminiDir, "conversations")
	_ = os.MkdirAll(convDir, 0755)
	_ = os.MkdirAll(filepath.Join(geminiDir, "annotations"), 0755)
	_ = os.MkdirAll(filepath.Join(geminiDir, "brain", "active-uuid"), 0755)

	dbPath := filepath.Join(convDir, "active-uuid.db")
	_ = os.WriteFile(dbPath, []byte("sqlite content"), 0644)
	_ = os.WriteFile(filepath.Join(geminiDir, "annotations", "active-uuid.pbtxt"), []byte("title: \"CLI Test Chat\""), 0644)
	_ = os.WriteFile(filepath.Join(geminiDir, "brain", "active-uuid", "doc.md"), []byte("# Note"), 0644)

	// Keep active-uuid.db open by this process
	f, err := os.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open active-uuid.db: %v", err)
	}
	defer f.Close()

	// 1. ai list --in-use
	out, err := executeCommand(rootCmd, "ai", "list", "ai-src", "--in-use")
	if err != nil {
		t.Fatalf("ai list --in-use failed: %v", err)
	}
	if !strings.Contains(out, "active-uuid") || !strings.Contains(out, "in use") {
		t.Errorf("expected active-uuid with in use status, got:\n%s", out)
	}

	// 2. ai list --in-use --json
	out, err = executeCommand(rootCmd, "ai", "list", "ai-src", "--in-use", "--json")
	if err != nil {
		t.Fatalf("ai list --in-use --json failed: %v", err)
	}
	var inUseConvs []chat.ConversationInfo
	if err := json.Unmarshal([]byte(out), &inUseConvs); err != nil {
		t.Fatalf("failed to parse ai list --in-use --json output: %v, raw output: %s", err, out)
	}
	if len(inUseConvs) != 1 || !inUseConvs[0].InUse || inUseConvs[0].ID != "active-uuid" {
		t.Errorf("expected 1 in-use conversation in json, got: %+v", inUseConvs)
	}

	// 3. ai export --skip-in-use
	exportTar := filepath.Join(tempHome, "ai-export.tar.gz")
	out, err = executeCommand(rootCmd, "ai", "export", "ai-src", exportTar, "--skip-in-use")
	if err != nil {
		t.Fatalf("ai export --skip-in-use failed: %v, output: %s", err, out)
	}

	// 4. ai sync --skip-in-use
	out, err = executeCommand(rootCmd, "ai", "sync", "ai-src", "ai-dest", "--skip-in-use")
	if err != nil {
		t.Fatalf("ai sync --skip-in-use failed: %v, output: %s", err, out)
	}
}



