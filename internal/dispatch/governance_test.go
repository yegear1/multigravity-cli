package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverGovernance_EmptyAndNil(t *testing.T) {
	tempDir := t.TempDir()

	gov := DiscoverGovernance(tempDir)
	if gov != nil {
		t.Fatalf("expected nil governance context for empty directory, got %+v", gov)
	}

	gov = DiscoverGovernance("", "   ")
	if gov != nil {
		t.Fatalf("expected nil governance context for empty paths, got %+v", gov)
	}
}

func TestDiscoverGovernance_FullStructure(t *testing.T) {
	tempDir := t.TempDir()

	agentsContent := "# Agent Guidelines\n\nFollow Chesterton's fence.\n"
	if err := os.WriteFile(filepath.Join(tempDir, "AGENTS.md"), []byte(agentsContent), 0644); err != nil {
		t.Fatalf("failed to write AGENTS.md: %v", err)
	}

	agentDir := filepath.Join(tempDir, ".agent")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("failed to create .agent dir: %v", err)
	}

	taskContent := `# TASK.md
## Active Task
### 📌 Task [03.6]: Governance context injection
- Description: Inject directives into agent prompt.
## Completed Tasks Log
`
	if err := os.WriteFile(filepath.Join(agentDir, "TASK.md"), []byte(taskContent), 0644); err != nil {
		t.Fatalf("failed to write TASK.md: %v", err)
	}

	notesContent := "# NOTES.md\n\nActive contracts.\n"
	if err := os.WriteFile(filepath.Join(agentDir, "NOTES.md"), []byte(notesContent), 0644); err != nil {
		t.Fatalf("failed to write NOTES.md: %v", err)
	}

	gov := DiscoverGovernance(tempDir)
	if gov == nil {
		t.Fatalf("expected governance context to be found")
	}
	if !gov.Found() {
		t.Fatalf("expected gov.Found() to be true")
	}
	if gov.AgentsPath != filepath.Join(tempDir, "AGENTS.md") {
		t.Errorf("unexpected AgentsPath: %q", gov.AgentsPath)
	}
	if gov.TaskPath != filepath.Join(agentDir, "TASK.md") {
		t.Errorf("unexpected TaskPath: %q", gov.TaskPath)
	}
	if gov.NotesPath != filepath.Join(agentDir, "NOTES.md") {
		t.Errorf("unexpected NotesPath: %q", gov.NotesPath)
	}
	if !strings.Contains(gov.ActiveTaskSnippet, "Task [03.6]: Governance context injection") {
		t.Errorf("unexpected ActiveTaskSnippet: %q", gov.ActiveTaskSnippet)
	}
	if strings.Contains(gov.ActiveTaskSnippet, "Completed Tasks Log") {
		t.Errorf("ActiveTaskSnippet leaked beyond next heading: %q", gov.ActiveTaskSnippet)
	}
}

func TestDiscoverGovernance_FallbackRootFiles(t *testing.T) {
	tempDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tempDir, "TASK.md"), []byte("## Active Task\nFallback task\n"), 0644); err != nil {
		t.Fatalf("failed to write TASK.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "NOTES.md"), []byte("Notes fallback"), 0644); err != nil {
		t.Fatalf("failed to write NOTES.md: %v", err)
	}

	gov := DiscoverGovernance(tempDir)
	if gov == nil || !gov.Found() {
		t.Fatalf("expected fallback governance files to be discovered")
	}
	if gov.TaskPath != filepath.Join(tempDir, "TASK.md") {
		t.Errorf("unexpected TaskPath fallback: %q", gov.TaskPath)
	}
	if gov.NotesPath != filepath.Join(tempDir, "NOTES.md") {
		t.Errorf("unexpected NotesPath fallback: %q", gov.NotesPath)
	}
	if !strings.Contains(gov.ActiveTaskSnippet, "Fallback task") {
		t.Errorf("unexpected ActiveTaskSnippet: %q", gov.ActiveTaskSnippet)
	}
}

func TestSynthesizePrompt_IdempotencyAndStructure(t *testing.T) {
	gov := &GovernanceContext{
		AgentsPath:        "/repo/AGENTS.md",
		TaskPath:          "/repo/.agent/TASK.md",
		NotesPath:         "/repo/.agent/NOTES.md",
		ActiveTaskSnippet: "### Task [01.1]: Fix bug",
	}

	base := "fix(gateway): resolve timeout"
	synth := SynthesizePrompt(base, gov)

	if !strings.Contains(synth, "[PROJECT GOVERNANCE CONTEXT]") {
		t.Errorf("expected governance context heading in synthesized prompt")
	}
	if !strings.Contains(synth, "AGENTS.md") || !strings.Contains(synth, ".agent/TASK.md") || !strings.Contains(synth, ".agent/NOTES.md") {
		t.Errorf("expected governance file references in prompt")
	}
	if !strings.Contains(synth, "[ACTIVE TASK CONTEXT]") || !strings.Contains(synth, "Task [01.1]: Fix bug") {
		t.Errorf("expected active task snippet in synthesized prompt")
	}
	if !strings.Contains(synth, "[TASK INSTRUCTION]\n"+base) {
		t.Errorf("expected base prompt under task instruction heading")
	}

	// Idempotency: synthesize again on already synthesized prompt
	second := SynthesizePrompt(synth, gov)
	if second != synth {
		t.Errorf("expected SynthesizePrompt to be idempotent, got diff:\n%s\nvs\n%s", second, synth)
	}

	// Edge cases: empty prompt or nil/empty gov
	if SynthesizePrompt("", gov) != "" {
		t.Errorf("expected empty prompt for empty input")
	}
	if SynthesizePrompt(base, nil) != base {
		t.Errorf("expected untouched prompt when gov is nil")
	}
	if SynthesizePrompt(base, &GovernanceContext{}) != base {
		t.Errorf("expected untouched prompt when gov is empty")
	}
}

func TestSynthesizePrompt_RunnerCompatibility(t *testing.T) {
	gov := &GovernanceContext{
		AgentsPath:        "/repo/AGENTS.md",
		TaskPath:          "/repo/.agent/TASK.md",
		ActiveTaskSnippet: "Active item",
	}

	basePrompt := "implement feature X"
	synthPrompt := SynthesizePrompt(basePrompt, gov)

	runners := []struct {
		agentType string
		wantCmd   string
		checkArg  func(t *testing.T, args []string)
	}{
		{
			agentType: "claude",
			wantCmd:   "claude",
			checkArg: func(t *testing.T, args []string) {
				if len(args) != 2 || args[0] != "-p" || args[1] != synthPrompt {
					t.Errorf("claude args mismatch: %+v", args)
				}
			},
		},
		{
			agentType: "aider",
			wantCmd:   "aider",
			checkArg: func(t *testing.T, args []string) {
				if len(args) != 2 || args[0] != "--message" || args[1] != synthPrompt {
					t.Errorf("aider args mismatch: %+v", args)
				}
			},
		},
		{
			agentType: "opencode",
			wantCmd:   "opencode",
			checkArg: func(t *testing.T, args []string) {
				if len(args) != 2 || args[0] != "run" || args[1] != synthPrompt {
					t.Errorf("opencode args mismatch: %+v", args)
				}
			},
		},
		{
			agentType: "agy",
			wantCmd:   "agy",
			checkArg: func(t *testing.T, args []string) {
				if len(args) != 4 || args[0] != "-p" || args[1] != synthPrompt || args[2] != "--output-format" || args[3] != "json" {
					t.Errorf("agy args mismatch: %+v", args)
				}
			},
		},
		{
			agentType: "codex",
			wantCmd:   "codex",
			checkArg: func(t *testing.T, args []string) {
				if len(args) < 4 || args[0] != "exec" || args[len(args)-1] != synthPrompt {
					t.Errorf("codex args mismatch: %+v", args)
				}
			},
		},
	}

	for _, tc := range runners {
		t.Run(tc.agentType, func(t *testing.T) {
			cmdStr, args := ResolveAgentCommandWithCwd(tc.agentType, "", nil, synthPrompt, "/tmp/repo")
			if cmdStr != tc.wantCmd {
				t.Errorf("expected command %q, got %q", tc.wantCmd, cmdStr)
			}
			tc.checkArg(t, args)
		})
	}
}

func TestInjectGovernanceEnv(t *testing.T) {
	gov := &GovernanceContext{
		AgentsPath: "/repo/AGENTS.md",
		TaskPath:   "/repo/.agent/TASK.md",
		NotesPath:  "/repo/.agent/NOTES.md",
	}

	baseEnv := map[string]string{
		"CUSTOM_VAR": "val123",
	}

	injected := InjectGovernanceEnv(baseEnv, gov)
	if injected["CUSTOM_VAR"] != "val123" {
		t.Errorf("expected CUSTOM_VAR preserved, got %q", injected["CUSTOM_VAR"])
	}
	if injected["MULTIGRAVITY_GOVERNANCE"] != "true" {
		t.Errorf("expected MULTIGRAVITY_GOVERNANCE=true, got %q", injected["MULTIGRAVITY_GOVERNANCE"])
	}
	if injected["MULTIGRAVITY_GOVERNANCE_AGENTS"] != "/repo/AGENTS.md" {
		t.Errorf("unexpected MULTIGRAVITY_GOVERNANCE_AGENTS: %q", injected["MULTIGRAVITY_GOVERNANCE_AGENTS"])
	}
	if injected["MULTIGRAVITY_GOVERNANCE_TASK"] != "/repo/.agent/TASK.md" {
		t.Errorf("unexpected MULTIGRAVITY_GOVERNANCE_TASK: %q", injected["MULTIGRAVITY_GOVERNANCE_TASK"])
	}
	if injected["MULTIGRAVITY_GOVERNANCE_NOTES"] != "/repo/.agent/NOTES.md" {
		t.Errorf("unexpected MULTIGRAVITY_GOVERNANCE_NOTES: %q", injected["MULTIGRAVITY_GOVERNANCE_NOTES"])
	}

	// Respect existing overrides
	overrideEnv := map[string]string{
		"MULTIGRAVITY_GOVERNANCE": "custom-gov",
	}
	injectedOverride := InjectGovernanceEnv(overrideEnv, gov)
	if injectedOverride["MULTIGRAVITY_GOVERNANCE"] != "custom-gov" {
		t.Errorf("expected user override respected, got %q", injectedOverride["MULTIGRAVITY_GOVERNANCE"])
	}
}
