package agent

import (
	"os"
	"os/exec"
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
		{"codex", "codex"},
		{"/usr/local/bin/codex", "codex"},
		{"codex.exe", "codex"},
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

func TestBuildAgentEnv_Codex(t *testing.T) {
	tempHome := t.TempDir()
	profDir := filepath.Join(tempHome, "AntigravityProfiles", "test-codex-prof")
	if err := os.MkdirAll(profDir, 0755); err != nil {
		t.Fatalf("failed to create profile dir: %v", err)
	}

	t.Setenv("MULTIGRAVITY_HOME", filepath.Join(tempHome, "AntigravityProfiles"))
	t.Setenv("REAL_HOME", tempHome)

	opts := CreateSessionOptions{
		Profile:    "test-codex-prof",
		AgentType:  "codex",
		Command:    "codex",
		GatewayURL: "http://127.0.0.1:8080",
		GatewayKey: "codex-test-key",
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

	// Verify CODEX_HOME isolation to profile dir
	expectedCodexHome := filepath.Join(profDir, ".codex")
	if envMap["CODEX_HOME"] != expectedCodexHome {
		t.Errorf("expected CODEX_HOME=%q, got %q", expectedCodexHome, envMap["CODEX_HOME"])
	}

	// Verify Gateway URL injection for Codex (OpenAI-compatible)
	if envMap["OPENAI_BASE_URL"] != "http://127.0.0.1:8080/v1" {
		t.Errorf("expected OPENAI_BASE_URL=http://127.0.0.1:8080/v1, got %q", envMap["OPENAI_BASE_URL"])
	}
	if envMap["OPENAI_API_BASE"] != "http://127.0.0.1:8080/v1" {
		t.Errorf("expected OPENAI_API_BASE=http://127.0.0.1:8080/v1, got %q", envMap["OPENAI_API_BASE"])
	}
	if envMap["OPENAI_API_KEY"] != "codex-test-key" {
		t.Errorf("expected OPENAI_API_KEY=codex-test-key, got %q", envMap["OPENAI_API_KEY"])
	}

	// Verify Multigravity metadata
	if envMap["MULTIGRAVITY_AGENT_TYPE"] != "codex" {
		t.Errorf("expected MULTIGRAVITY_AGENT_TYPE=codex, got %q", envMap["MULTIGRAVITY_AGENT_TYPE"])
	}
}

func TestManagerSessionLifecycle(t *testing.T) {
	mgr := NewManager()

	cmdName := "sh"
	cmdArgs := []string{"-c", "echo 'hello from pty'; read line; echo \"got: $line\""}
	if os.PathSeparator == '\\' {
		cmdName = "cmd"
		cmdArgs = []string{"/c", "echo hello from pty& set /p line=& call echo got: %line%"}
	}

	opts := CreateSessionOptions{
		Command: cmdName,
		Args:    cmdArgs,
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
	if err := mgr.WriteSessionInput(info.ID, []byte("interactive input\r\n")); err != nil {
		t.Fatalf("failed to write input: %v", err)
	}

	// Wait for response or completion
	inst.Wait(5 * time.Second)
	time.Sleep(50 * time.Millisecond)

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

func TestSupportsNativeSandbox(t *testing.T) {
	if !SupportsNativeSandbox("codex") {
		t.Error("expected codex to support native sandbox")
	}
	if !SupportsNativeSandbox("Codex") {
		t.Error("expected Codex (case-insensitive) to support native sandbox")
	}
	for _, runner := range []string{"claude", "aider", "opencode", "agy", "custom", "bash"} {
		if SupportsNativeSandbox(runner) {
			t.Errorf("expected runner %q to not support native sandbox", runner)
		}
	}
}

func TestWrapWithAgentGuard(t *testing.T) {
	// Standard wrapping with default role
	cmd, args := WrapWithAgentGuard("claude", []string{"-p", "fix bug"}, "/repo/path", "")
	if cmd != "agent-guard" {
		t.Errorf("expected cmd 'agent-guard', got %q", cmd)
	}
	expectedArgs := []string{"wrap", "--role", "developer", "--repo", "/repo/path", "--", "claude", "-p", "fix bug"}
	if len(args) != len(expectedArgs) {
		t.Fatalf("expected %d args, got %d: %v", len(expectedArgs), len(args), args)
	}
	for i, a := range expectedArgs {
		if args[i] != a {
			t.Errorf("args[%d] = %q, want %q", i, args[i], a)
		}
	}

	// Custom role
	cmd, args = WrapWithAgentGuard("aider", []string{"--message", "hi"}, "/repo/path", "auditor")
	if cmd != "agent-guard" || args[2] != "auditor" {
		t.Errorf("expected custom role 'auditor', got: %v", args)
	}

	// Idempotent wrapping (do not wrap twice)
	cmd2, args2 := WrapWithAgentGuard(cmd, args, "/repo/path", "developer")
	if cmd2 != cmd || len(args2) != len(args) {
		t.Errorf("expected idempotent wrap, got %s %v", cmd2, args2)
	}
}

func TestValidatePathWithinRepo(t *testing.T) {
	repoDir := "/home/user/project"

	// Paths inside repo
	validPaths := []string{
		"file.txt",
		"src/main.go",
		"./src/main.go",
		"/home/user/project/file.txt",
		"/home/user/project/sub/dir/nested.go",
	}
	for _, p := range validPaths {
		if err := ValidatePathWithinRepo(repoDir, p); err != nil {
			t.Errorf("expected path %q to be valid inside %q, got err: %v", p, repoDir, err)
		}
	}

	// Paths outside repo
	invalidPaths := []string{
		"../outside.txt",
		"../../etc/passwd",
		"/etc/passwd",
		"/home/user/other-project",
		"/home/user/project/../../escape",
	}
	for _, p := range invalidPaths {
		if err := ValidatePathWithinRepo(repoDir, p); err == nil {
			t.Errorf("expected path %q to fail validation against %q, but got nil", p, repoDir)
		}
	}
}

func TestInterceptSubshellMutation(t *testing.T) {
	repoDir := "/home/user/project"

	// Valid subshell operations within repository root
	validCmds := []string{
		"echo hello",
		"cat README.md",
		"touch file.txt",
		"touch src/main.go",
		"echo 'data' > output.log",
		"echo 'more' >> logs/run.log",
		"rm -rf ./build",
		"cp src/a.go src/b.go",
		"mv old.txt new.txt",
		"mkdir -p internal/foo",
		"git status",
		"git -C ./submodule status",
		"sed -i 's/a/b/' file.txt",
	}
	for _, cmd := range validCmds {
		if err := InterceptSubshellMutation(repoDir, cmd); err != nil {
			t.Errorf("expected subshell command %q to be allowed, got: %v", cmd, err)
		}
	}

	// Blocked subshell mutations attempting writes outside repository root
	blockedCmds := []struct {
		cmd  string
		desc string
	}{
		{"touch /tmp/evil.txt", "absolute path mutation outside repo"},
		{"touch ../outside.txt", "relative traversal mutation outside repo"},
		{"rm -rf /var/log", "destructive rm outside repo"},
		{"rm ../file.txt", "destructive rm via path traversal"},
		{"cp file.txt /etc/hosts", "copying outside repo"},
		{"mv file.txt ../stolen.txt", "moving outside repo"},
		{"mkdir -p /root/secret", "creating directory outside repo"},
		{"echo 'hack' > /etc/shadow", "output redirection outside repo"},
		{"echo 'bad' >> ../other.log", "append redirection outside repo"},
		{"git -C /other/repo commit -m 'oops'", "git targeting outside repo"},
		{"git --git-dir=/other/repo/.git commit", "git dir outside repo"},
		{"sed -i 's/a/b/' ../outside.txt", "sed in-place outside repo"},
		{"echo OK; rm -rf /etc", "chained command containing mutation outside repo"},
		{"ls && touch /tmp/bad", "&& chained mutation outside repo"},
	}

	for _, tc := range blockedCmds {
		if err := InterceptSubshellMutation(repoDir, tc.cmd); err == nil {
			t.Errorf("expected command %q (%s) to be intercepted and blocked, but got nil", tc.cmd, tc.desc)
		}
	}
}

func TestStartSession_SandboxWrapping(t *testing.T) {
	mgr := NewManager()
	testRepo := t.TempDir()

	var capturedOpts CreateSessionOptions
	cleanup := SetTestHooks(func(opts CreateSessionOptions) *exec.Cmd {
		capturedOpts = opts
		return exec.Command("sh", "-c", "echo 'dummy'")
	})
	defer cleanup()

	// 1. Non-native runner (claude) with Sandbox: true gets wrapped with agent-guard
	inst, err := mgr.StartSession(CreateSessionOptions{
		AgentType: "claude",
		Command:   "claude",
		Args:      []string{"-p", "hello"},
		Cwd:       testRepo,
		RepoPath:  testRepo,
		Sandbox:   true,
	})
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}

	info := inst.GetInfo()
	if info.Command != "agent-guard" {
		t.Errorf("expected command 'agent-guard', got %q", info.Command)
	}
	expectedArgs := []string{"wrap", "--role", "developer", "--repo", testRepo, "--", "claude", "-p", "hello"}
	if len(info.Args) != len(expectedArgs) {
		t.Fatalf("expected %d args, got %d: %v", len(expectedArgs), len(info.Args), info.Args)
	}
	for i, a := range expectedArgs {
		if info.Args[i] != a {
			t.Errorf("info.Args[%d] = %q, want %q", i, info.Args[i], a)
		}
	}
	if capturedOpts.Command != "agent-guard" {
		t.Errorf("expected captured Command to be 'agent-guard', got %q", capturedOpts.Command)
	}

	// 2. Native runner (codex) with Sandbox: true does NOT get wrapped with agent-guard
	instCodex, err := mgr.StartSession(CreateSessionOptions{
		AgentType: "codex",
		Command:   "codex",
		Args:      []string{"exec", "--sandbox", "workspace-write", "fix bug"},
		Cwd:       testRepo,
		RepoPath:  testRepo,
		Sandbox:   true,
	})
	if err != nil {
		t.Fatalf("failed to start codex session: %v", err)
	}

	infoCodex := instCodex.GetInfo()
	if infoCodex.Command != "codex" {
		t.Errorf("expected codex command to NOT be wrapped with agent-guard, got %q", infoCodex.Command)
	}
	if capturedOpts.Command != "codex" {
		t.Errorf("expected captured Command to be 'codex', got %q", capturedOpts.Command)
	}
}

func TestSessionMultipleSubscribersAndResize(t *testing.T) {
	mgr := NewManager()

	cmdName := "sh"
	cmdArgs := []string{"-c", "echo 'multiplex-start'; read l; echo 'multiplex-end'"}
	if os.PathSeparator == '\\' {
		cmdName = "cmd"
		cmdArgs = []string{"/c", "echo multiplex-start& set /p l=& echo multiplex-end"}
	}

	inst, err := mgr.StartSession(CreateSessionOptions{
		Command: cmdName,
		Args:    cmdArgs,
		Rows:    24,
		Cols:    80,
	})
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}

	sub1, unsub1, err := mgr.SubscribeSession(inst.GetInfo().ID)
	if err != nil {
		t.Fatalf("sub1 failed: %v", err)
	}
	defer unsub1()

	sub2, unsub2, err := mgr.SubscribeSession(inst.GetInfo().ID)
	if err != nil {
		t.Fatalf("sub2 failed: %v", err)
	}
	defer unsub2()

	// Both subscribers should receive the initial chunk
	received1, received2 := false, false
	timeout := time.After(3 * time.Second)

waitLoop:
	for !received1 || !received2 {
		select {
		case chunk, ok := <-sub1:
			if ok && strings.Contains(chunk.Data, "multiplex-start") {
				received1 = true
			}
		case chunk, ok := <-sub2:
			if ok && strings.Contains(chunk.Data, "multiplex-start") {
				received2 = true
			}
		case <-timeout:
			break waitLoop
		}
	}

	if !received1 || !received2 {
		t.Errorf("expected both subscribers to receive output, sub1=%v sub2=%v", received1, received2)
	}

	// Test Resize across manager
	if err := mgr.ResizeSession(inst.GetInfo().ID, 50, 150); err != nil {
		t.Errorf("resize failed: %v", err)
	}
	if inst.GetInfo().Rows != 50 || inst.GetInfo().Cols != 150 {
		t.Errorf("expected rows=50 cols=150, got rows=%d cols=%d", inst.GetInfo().Rows, inst.GetInfo().Cols)
	}

	// Write input to finish
	_ = mgr.WriteSessionInput(inst.GetInfo().ID, []byte("ok\r\n"))
	inst.Wait(3 * time.Second)
}

func TestBuildAgentEnvTraceID(t *testing.T) {
	// 1. Explicit TraceID in CreateSessionOptions
	opts := CreateSessionOptions{
		Command: "echo",
		TraceID: "tr-session-abc",
	}
	env, err := BuildAgentEnv(opts)
	if err != nil {
		t.Fatalf("BuildAgentEnv failed: %v", err)
	}

	foundMaestri := false
	foundMg := false
	for _, e := range env {
		if e == "MAESTRI_TRACE_ID=tr-session-abc" {
			foundMaestri = true
		}
		if e == "MULTIGRAVITY_TRACE_ID=tr-session-abc" {
			foundMg = true
		}
	}
	if !foundMaestri {
		t.Errorf("expected MAESTRI_TRACE_ID=tr-session-abc in env, got: %v", env)
	}
	if !foundMg {
		t.Errorf("expected MULTIGRAVITY_TRACE_ID=tr-session-abc in env, got: %v", env)
	}

	// 2. Ingest from host environment fallback
	t.Setenv("MAESTRI_TRACE_ID", "tr-host-env-xyz")
	opts2 := CreateSessionOptions{
		Command: "echo",
	}
	env2, err := BuildAgentEnv(opts2)
	if err != nil {
		t.Fatalf("BuildAgentEnv failed: %v", err)
	}
	foundFallback := false
	for _, e := range env2 {
		if e == "MAESTRI_TRACE_ID=tr-host-env-xyz" {
			foundFallback = true
		}
	}
	if !foundFallback {
		t.Errorf("expected MAESTRI_TRACE_ID=tr-host-env-xyz in env from host, got: %v", env2)
	}
}
