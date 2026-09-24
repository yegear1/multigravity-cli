package prime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPromptsLoadingAndSelection(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("REAL_HOME", tmpDir)

	prompts := LoadPrompts()
	if len(prompts) != 40 {
		t.Fatalf("expected 40 default prompts, got %d", len(prompts))
	}

	promptsFile := filepath.Join(tmpDir, ".local", "share", "multigravity", "prompts.json")
	if _, err := os.Stat(promptsFile); err != nil {
		t.Fatalf("prompts.json should have been created: %v", err)
	}

	used := make(map[string]bool)
	p1 := SelectRandomPrompt(prompts, used)
	used[p1] = true
	p2 := SelectRandomPrompt(prompts, used)
	if p1 == p2 {
		t.Errorf("p1 and p2 should be different when p1 is marked used")
	}
}

func TestStateLegacyMigration(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("REAL_HOME", tmpDir)

	stateFile := filepath.Join(tmpDir, ".local", "share", "multigravity", "prime_state.json")
	_ = os.MkdirAll(filepath.Dir(stateFile), 0755)

	legacyJSON := `{
		"profiles": {
			"work": {
				"last_reset_time": "2026-09-30T00:00:00Z",
				"last_primed_at": "2026-09-23T00:00:00Z",
				"last_cascade_id": "casc-old-123",
				"last_prompt": "ping"
			}
		}
	}`
	if err := os.WriteFile(stateFile, []byte(legacyJSON), 0644); err != nil {
		t.Fatal(err)
	}

	state, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	workState, ok := state.Profiles["work"]
	if !ok {
		t.Fatalf("profile 'work' not found in state")
	}

	geminiB, hasGemini := workState["gemini"]
	if !hasGemini {
		t.Fatalf("legacy state was not migrated into 'gemini' bucket")
	}

	if geminiB.LastResetTime != "2026-09-30T00:00:00Z" {
		t.Errorf("expected reset time preserved, got %s", geminiB.LastResetTime)
	}
	if geminiB.LastCascadeID != "casc-old-123" {
		t.Errorf("expected cascade id preserved, got %s", geminiB.LastCascadeID)
	}

	// Test saving and reloading migrated state
	geminiB.LastModel = "gemini-3.6-flash-low"
	workState["gemini"] = geminiB
	state.Profiles["work"] = workState

	if err := SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	reloaded, err := LoadState()
	if err != nil {
		t.Fatalf("reloading state failed: %v", err)
	}
	if reloaded.Profiles["work"]["gemini"].LastModel != "gemini-3.6-flash-low" {
		t.Errorf("expected model saved and reloaded")
	}
}
