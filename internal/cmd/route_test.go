package cmd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func TestRouteCommand_NoServers(t *testing.T) {
	cleanup := quota.SetTestHooks(func(string) ([]quota.ActiveServer, error) {
		return nil, nil
	})
	defer cleanup()

	// Text mode
	out, err := executeCommand(rootCmd, "route")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(out, "No active Antigravity profile or Language Server found") {
		t.Fatalf("unexpected text output: %s", out)
	}

	// JSON mode
	outJSON, err := executeCommand(rootCmd, "route", "--json")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	var plan quota.RoutePlan
	if err := json.Unmarshal([]byte(outJSON), &plan); err != nil {
		t.Fatalf("failed to parse JSON: %v, output: %s", err, outJSON)
	}
	if plan.Recommended != nil {
		t.Fatalf("expected nil recommendation, got: %+v", plan.Recommended)
	}
	if len(plan.Candidates) != 0 {
		t.Fatalf("expected 0 candidates, got: %d", len(plan.Candidates))
	}
}

func TestRouteCommand_WithServersAndWeights(t *testing.T) {
	now := time.Now().UTC()
	resetTime := now.Add(2 * time.Hour).Format(time.RFC3339)

	mockServers := []quota.ActiveServer{
		{
			Profile:    "dev-primary",
			PID:        1001,
			Port:       50051,
			CSRF:       "super-secret-csrf-token",
			QuotaKnown: true,
			Data: &quota.QuotaSummaryResponse{
				Response: quota.QuotaResponse{
					Groups: []quota.QuotaGroup{{
						DisplayName: "Gemini Models",
						Buckets: []quota.QuotaBucket{
							{BucketID: "gemini-weekly", RemainingFraction: 0.75, ResetTime: now.Add(48 * time.Hour).Format(time.RFC3339)},
							{BucketID: "gemini-5h", RemainingFraction: 0.85, ResetTime: resetTime},
						},
					}},
				},
			},
		},
		{
			Profile:    "dev-secondary",
			PID:        1002,
			Port:       50052,
			CSRF:       "another-secret-token",
			QuotaKnown: true,
			Data: &quota.QuotaSummaryResponse{
				Response: quota.QuotaResponse{
					Groups: []quota.QuotaGroup{{
						DisplayName: "Gemini Models",
						Buckets: []quota.QuotaBucket{
							{BucketID: "gemini-weekly", RemainingFraction: 0.20, ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339)},
							{BucketID: "gemini-5h", RemainingFraction: 0.10, ResetTime: resetTime},
						},
					}},
				},
			},
		},
	}

	cleanup := quota.SetTestHooks(func(string) ([]quota.ActiveServer, error) {
		return mockServers, nil
	})
	defer cleanup()

	// 1. Text mode default (medium weight)
	out, err := executeCommand(rootCmd, "route")
	if err != nil {
		t.Fatalf("executeCommand(route) error: %v", err)
	}
	if !strings.Contains(out, "Recommended Route:") || !strings.Contains(out, "dev-primary") {
		t.Fatalf("expected dev-primary recommendation in text output: %s", out)
	}
	if !strings.Contains(out, "gemini-3.8-flash-medium") {
		t.Fatalf("expected medium model in text output: %s", out)
	}
	if !strings.Contains(out, "Tier: standard (tier 1)") && !strings.Contains(out, "standard (tier 1)") {
		t.Fatalf("expected tier specification in text output: %s", out)
	}
	if !strings.Contains(out, "Candidate Profiles:") {
		t.Fatalf("expected candidate profiles table in text output: %s", out)
	}

	// 2. JSON mode
	outJSON, err := executeCommand(rootCmd, "route", "--json")
	if err != nil {
		t.Fatalf("executeCommand(route --json) error: %v", err)
	}

	// Verify CSRF token is NOT leaked
	if strings.Contains(outJSON, "super-secret-csrf-token") || strings.Contains(outJSON, "another-secret-token") {
		t.Fatal("SECURITY VIOLATION: CSRF token leaked in route JSON output!")
	}

	var plan quota.RoutePlan
	if err := json.Unmarshal([]byte(outJSON), &plan); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v, raw output: %s", err, outJSON)
	}
	if plan.Recommended == nil {
		t.Fatal("expected recommendation, got nil")
	}
	if plan.Recommended.Profile != "dev-primary" {
		t.Fatalf("expected dev-primary, got %s", plan.Recommended.Profile)
	}
	if plan.Recommended.Model != "gemini-3.8-flash-medium" {
		t.Fatalf("expected gemini-3.8-flash-medium, got %s", plan.Recommended.Model)
	}
	if len(plan.Candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(plan.Candidates))
	}

	// 3. Weight flags
	outLight, err := executeCommand(rootCmd, "route", "--weight", "light", "--json")
	if err != nil {
		t.Fatalf("route --weight light error: %v", err)
	}
	var planLight quota.RoutePlan
	if err := json.Unmarshal([]byte(outLight), &planLight); err != nil {
		t.Fatalf("failed to parse light JSON: %v", err)
	}
	if planLight.Recommended == nil || planLight.Recommended.Model != "gemini-3.8-flash-low" {
		t.Fatalf("expected gemini-3.8-flash-low for light weight, got: %+v", planLight.Recommended)
	}

	outHeavy, err := executeCommand(rootCmd, "route", "--weight", "heavy", "--json")
	if err != nil {
		t.Fatalf("route --weight heavy error: %v", err)
	}
	var planHeavy quota.RoutePlan
	if err := json.Unmarshal([]byte(outHeavy), &planHeavy); err != nil {
		t.Fatalf("failed to parse heavy JSON: %v", err)
	}
	if planHeavy.Recommended == nil || planHeavy.Recommended.Model != "gemini-3.8-flash-high" {
		t.Fatalf("expected gemini-3.8-flash-high for heavy weight, got: %+v", planHeavy.Recommended)
	}

	// 4. Invalid weight flag
	_, err = executeCommand(rootCmd, "route", "--weight", "bogus")
	if err == nil {
		t.Fatal("expected error for bogus weight, got nil")
	}
	if !strings.Contains(err.Error(), "invalid weight") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRouteCommand_TaskComplexityClassification(t *testing.T) {
	now := time.Now().UTC()
	resetTime := now.Add(2 * time.Hour).Format(time.RFC3339)

	mockServers := []quota.ActiveServer{
		{
			Profile:    "dev-primary",
			PID:        1001,
			Port:       50051,
			CSRF:       "csrf-token-1",
			QuotaKnown: true,
			Data: &quota.QuotaSummaryResponse{
				Response: quota.QuotaResponse{
					Groups: []quota.QuotaGroup{{
						DisplayName: "Gemini Models",
						Buckets: []quota.QuotaBucket{
							{BucketID: "gemini-weekly", RemainingFraction: 0.80, ResetTime: now.Add(48 * time.Hour).Format(time.RFC3339)},
							{BucketID: "gemini-5h", RemainingFraction: 0.90, ResetTime: resetTime},
						},
					}},
				},
			},
		},
	}

	cleanup := quota.SetTestHooks(func(string) ([]quota.ActiveServer, error) {
		return mockServers, nil
	})
	defer cleanup()

	// 1. --task with micro keyword -> light weight, micro tier (0)
	outText, err := executeCommand(rootCmd, "route", "--task", "fix typo in docstring")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(outText, "Weight:   light") {
		t.Fatalf("expected light weight in output: %s", outText)
	}
	if !strings.Contains(outText, "Tier:     micro (tier 0)") {
		t.Fatalf("expected Tier: micro (tier 0) in output: %s", outText)
	}

	// 2. --task with architecture keyword -> heavy weight, reasoning tier (2)
	outHeavyJSON, err := executeCommand(rootCmd, "route", "--task", "debug deadlock in concurrency module", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var planHeavy quota.RoutePlan
	if err := json.Unmarshal([]byte(outHeavyJSON), &planHeavy); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if planHeavy.Weight != "heavy" {
		t.Fatalf("expected heavy weight, got %s", planHeavy.Weight)
	}

	// 3. --paths > 3 -> heavy weight
	outPathsJSON, err := executeCommand(rootCmd, "route", "--paths", "a.go,b.go,c.go,d.go", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var planPaths quota.RoutePlan
	if err := json.Unmarshal([]byte(outPathsJSON), &planPaths); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if planPaths.Weight != "heavy" {
		t.Fatalf("expected heavy weight for >3 paths, got %s", planPaths.Weight)
	}

	// 4. --paths 1 file with line ranged --read -> light weight
	outReadJSON, err := executeCommand(rootCmd, "route", "--paths", "main.go", "--read", "main.go:1-50", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var planRead quota.RoutePlan
	if err := json.Unmarshal([]byte(outReadJSON), &planRead); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if planRead.Weight != "light" {
		t.Fatalf("expected light weight for single path with line-ranged read, got %s", planRead.Weight)
	}

	// 5. Explicit --weight overrides task classification
	outOverrideJSON, err := executeCommand(rootCmd, "route", "--weight", "light", "--task", "deadlock in multi-repo architecture", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var planOverride quota.RoutePlan
	if err := json.Unmarshal([]byte(outOverrideJSON), &planOverride); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if planOverride.Weight != "light" {
		t.Fatalf("expected explicit --weight light to override heavy task, got %s", planOverride.Weight)
	}
}
