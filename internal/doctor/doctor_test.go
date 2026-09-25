package doctor

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunDoctor(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)

	// Mock fake app binary so app detection passes
	fakeApp := filepath.Join(tempHome, "fake-agy")
	if err := os.WriteFile(fakeApp, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MULTIGRAVITY_APP", fakeApp)

	var buf bytes.Buffer
	res, err := RunDoctor(&buf)
	if err != nil {
		t.Fatalf("unexpected error running doctor: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Checking multigravity environment...") {
		t.Errorf("expected header, got: %s", out)
	}
	if !strings.Contains(out, "Platform:") {
		t.Errorf("expected platform check, got: %s", out)
	}
	if !strings.Contains(out, "Antigravity / Agy: Found at") {
		t.Errorf("expected app check to find fake-agy, got: %s", out)
	}
	if !strings.Contains(out, "(writable)") {
		t.Errorf("expected profile storage writable, got: %s", out)
	}
	if res.Errors > 0 {
		t.Errorf("expected 0 errors with fake app, got %d errors", res.Errors)
	}
}

func TestRunDoctorAppNotFound(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)
	t.Setenv("MULTIGRAVITY_APP", filepath.Join(tempHome, "nonexistent-app"))
	t.Setenv("AGY_APP", "")
	t.Setenv("PATH", "") // empty PATH to ensure no system antigravity is found

	var buf bytes.Buffer
	res, err := RunDoctor(&buf)
	if err != nil {
		t.Fatalf("unexpected error running doctor: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Antigravity / Agy: Not found") {
		t.Errorf("expected app not found message, got: %s", out)
	}
	if res.Errors == 0 {
		t.Errorf("expected at least 1 error, got %d", res.Errors)
	}
}

func TestDoctorEmbeddedIconCheck(t *testing.T) {
	report, err := Diagnose()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runtime.GOOS == "darwin" {
		found := false
		for _, check := range report.Checks {
			if check.Name == "Application Icon" {
				found = true
				if check.Status != StatusOK {
					t.Errorf("expected StatusOK for Application Icon, got %s", check.Status)
				}
				if !strings.Contains(check.Message, "Embedded") {
					t.Errorf("expected Embedded in message, got %s", check.Message)
				}
			}
		}
		if !found {
			t.Errorf("Application Icon check not found in report")
		}
	}
}
