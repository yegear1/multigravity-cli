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
	if !strings.Contains(out, "gemini-3.8-flash-tiered") {
		t.Fatalf("expected medium model in text output: %s", out)
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
	if plan.Recommended.Model != "gemini-3.8-flash-tiered" {
		t.Fatalf("expected gemini-3.8-flash-tiered, got %s", plan.Recommended.Model)
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
	if planLight.Recommended == nil || planLight.Recommended.Model != "gemini-2.5-flash" {
		t.Fatalf("expected gemini-2.5-flash for light weight, got: %+v", planLight.Recommended)
	}

	outHeavy, err := executeCommand(rootCmd, "route", "--weight", "heavy", "--json")
	if err != nil {
		t.Fatalf("route --weight heavy error: %v", err)
	}
	var planHeavy quota.RoutePlan
	if err := json.Unmarshal([]byte(outHeavy), &planHeavy); err != nil {
		t.Fatalf("failed to parse heavy JSON: %v", err)
	}
	if planHeavy.Recommended == nil || planHeavy.Recommended.Model != "gemini-2.5-pro" {
		t.Fatalf("expected gemini-2.5-pro for heavy weight, got: %+v", planHeavy.Recommended)
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
