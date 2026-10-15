package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func TestQuotaBurnCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	profileName := "prof-burn"
	if err := os.MkdirAll(filepath.Join(home, profileName), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := quota.RecordTokens(profileName, quota.SourceGateway, "gemini-3.8-flash-tiered", 150, 250, 400, false); err != nil {
		t.Fatal(err)
	}

	// 1. Single profile text output
	text, err := executeCommand(rootCmd, "quota", "burn", profileName, "--window", "1h")
	if err != nil {
		t.Fatalf("quota burn: %v", err)
	}
	if !strings.Contains(text, "Tokens Burned:     400 tokens") {
		t.Fatalf("unexpected burn output:\n%s", text)
	}

	// 2. Single profile JSON output
	jsonOut, err := executeCommand(rootCmd, "quota", "burn", profileName, "--json")
	if err != nil {
		t.Fatalf("quota burn --json: %v", err)
	}
	var rate quota.TokenBurnRate
	if err := json.Unmarshal([]byte(jsonOut), &rate); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v\n%s", err, jsonOut)
	}
	if rate.Profile != profileName || rate.TokensBurned != 400 || rate.SampleCount != 1 {
		t.Fatalf("unexpected burn rate payload: %+v", rate)
	}

	// 3. Fleet burn text and JSON
	fleetText, err := executeCommand(rootCmd, "quota", "burn")
	if err != nil {
		t.Fatalf("quota burn fleet: %v", err)
	}
	if !strings.Contains(fleetText, profileName) || !strings.Contains(fleetText, "400") {
		t.Fatalf("unexpected fleet burn output:\n%s", fleetText)
	}

	fleetJSON, err := executeCommand(rootCmd, "quota", "burn", "--json")
	if err != nil {
		t.Fatalf("quota burn fleet --json: %v", err)
	}
	var fleet map[string]quota.TokenBurnRate
	if err := json.Unmarshal([]byte(fleetJSON), &fleet); err != nil {
		t.Fatalf("failed to unmarshal fleet JSON: %v\n%s", err, fleetJSON)
	}
	if r, ok := fleet[profileName]; !ok || r.TokensBurned != 400 {
		t.Fatalf("expected prof-burn with 400 tokens in fleet payload: %+v", fleet)
	}
}

func TestQuotaThrottleCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	profileName := "prof-throttle"
	if err := os.MkdirAll(filepath.Join(home, profileName), 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(home, ".multigravity", "throttle-policy.json")
	restore := quota.SetTestThrottleConfigPath(cfgPath)
	defer restore()

	cfg := &quota.ThrottleConfig{
		DefaultPolicy: quota.ThrottlePolicy{
			MaxTokensPerHour:    25000,
			MaxTokensPerMinute:  1000,
			DepletionThreshold:  0.12,
			ThrottleDelayMs:     800,
			AutoThrottleEnabled: true,
		},
		Profiles: map[string]quota.ThrottlePolicy{
			profileName: {
				Profile:             profileName,
				MaxTokensPerHour:    50000,
				MaxTokensPerMinute:  2000,
				DepletionThreshold:  0.08,
				ThrottleDelayMs:     400,
				AutoThrottleEnabled: true,
			},
		},
	}
	if err := quota.SaveThrottleConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// 1. Fleet throttle config text
	text, err := executeCommand(rootCmd, "quota", "throttle")
	if err != nil {
		t.Fatalf("quota throttle: %v", err)
	}
	if !strings.Contains(text, "Default Fleet Throttle Policy:") || !strings.Contains(text, "prof-throttle") {
		t.Fatalf("unexpected throttle text output:\n%s", text)
	}

	// 2. Fleet throttle config JSON
	jsonOut, err := executeCommand(rootCmd, "quota", "throttle", "--json")
	if err != nil {
		t.Fatalf("quota throttle --json: %v", err)
	}
	var loadedCfg quota.ThrottleConfig
	if err := json.Unmarshal([]byte(jsonOut), &loadedCfg); err != nil {
		t.Fatalf("failed to unmarshal throttle JSON: %v\n%s", err, jsonOut)
	}
	if loadedCfg.DefaultPolicy.MaxTokensPerHour != 25000 || loadedCfg.DefaultPolicy.DepletionThreshold != 0.12 {
		t.Fatalf("unexpected loaded default policy: %+v", loadedCfg.DefaultPolicy)
	}

	// 3. Profile specific throttle command
	profText, err := executeCommand(rootCmd, "quota", "throttle", profileName)
	if err != nil {
		t.Fatalf("quota throttle profile: %v", err)
	}
	if !strings.Contains(profText, "prof-throttle") || !strings.Contains(profText, "400ms") {
		t.Fatalf("unexpected profile throttle text output:\n%s", profText)
	}

	profJSON, err := executeCommand(rootCmd, "quota", "throttle", profileName, "--json")
	if err != nil {
		t.Fatalf("quota throttle profile --json: %v", err)
	}
	var pol quota.ThrottlePolicy
	if err := json.Unmarshal([]byte(profJSON), &pol); err != nil {
		t.Fatalf("failed to unmarshal profile policy JSON: %v\n%s", err, profJSON)
	}
	if pol.MaxTokensPerHour != 50000 || pol.ThrottleDelayMs != 400 || pol.DepletionThreshold != 0.08 {
		t.Fatalf("unexpected profile policy: %+v", pol)
	}
}
