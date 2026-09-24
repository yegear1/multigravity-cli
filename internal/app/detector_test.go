package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindAppOverride(t *testing.T) {
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "fake-antigravity")
	if runtime.GOOS == "windows" {
		fakeBin += ".exe"
	}
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\necho ok"), 0755); err != nil {
		t.Fatalf("failed to create fake bin: %v", err)
	}

	// Test MULTIGRAVITY_APP
	t.Setenv("MULTIGRAVITY_APP", fakeBin)
	t.Setenv("AGY_APP", "")

	found, err := FindApp()
	if err != nil {
		t.Fatalf("expected FindApp to succeed, got %v", err)
	}
	if found != fakeBin {
		t.Errorf("expected %q, got %q", fakeBin, found)
	}

	// Test AGY_APP
	t.Setenv("MULTIGRAVITY_APP", "")
	t.Setenv("AGY_APP", fakeBin)

	foundAgy, err := FindApp()
	if err != nil {
		t.Fatalf("expected FindApp to succeed with AGY_APP, got %v", err)
	}
	if foundAgy != fakeBin {
		t.Errorf("expected %q, got %q", fakeBin, foundAgy)
	}
}

func TestRequireAppNotFound(t *testing.T) {
	t.Setenv("MULTIGRAVITY_APP", "/nonexistent/path/to/bin")
	t.Setenv("AGY_APP", "")
	t.Setenv("PATH", "")

	_, err := RequireApp()
	if err == nil {
		t.Errorf("expected RequireApp to return error when binary is missing")
	}
}

func TestFindLanguageServer(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "resources", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create binDir: %v", err)
	}
	lsName := "language_server"
	if runtime.GOOS == "windows" {
		lsName += ".exe"
	}
	fakeLS := filepath.Join(binDir, lsName)
	if err := os.WriteFile(fakeLS, []byte("#!/bin/sh\necho ls"), 0755); err != nil {
		t.Fatalf("failed to write fakeLS: %v", err)
	}

	fakeApp := filepath.Join(tmpDir, "antigravity")
	if runtime.GOOS == "windows" {
		fakeApp += ".exe"
	}
	if err := os.WriteFile(fakeApp, []byte("#!/bin/sh\necho app"), 0755); err != nil {
		t.Fatalf("failed to write fakeApp: %v", err)
	}

	t.Setenv("MULTIGRAVITY_APP", fakeApp)
	foundLS, err := FindLanguageServer()
	if err != nil {
		t.Fatalf("expected FindLanguageServer to find binary, got %v", err)
	}
	if foundLS != fakeLS {
		t.Errorf("expected %q, got %q", fakeLS, foundLS)
	}
}
