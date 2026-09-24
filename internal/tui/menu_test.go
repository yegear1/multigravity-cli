package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/profile"
)

func setupTestEnv(t *testing.T) string {
	tempHome := t.TempDir()
	profilesDir := filepath.Join(tempHome, "profiles")
	_ = os.MkdirAll(profilesDir, 0755)
	shortcutsDir := filepath.Join(tempHome, "shortcuts")
	_ = os.MkdirAll(shortcutsDir, 0755)
	t.Setenv("MULTIGRAVITY_HOME", profilesDir)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", shortcutsDir)
	return profilesDir
}

func TestRunMenuNoProfiles(t *testing.T) {
	setupTestEnv(t)

	var in bytes.Buffer
	var out bytes.Buffer

	err := RunMenu(&in, &out, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	str := out.String()
	if !strings.Contains(str, "No profiles found.") {
		t.Errorf("expected 'No profiles found.', got: %s", str)
	}
}

func TestRunMenuQuit(t *testing.T) {
	setupTestEnv(t)

	if err := profile.CreateProfile(profile.CreateOptions{Name: "test-p1"}); err != nil {
		t.Fatal(err)
	}

	// Test "q"
	in := bytes.NewBufferString("q\n")
	var out bytes.Buffer
	err := RunMenu(in, &out, nil)
	if err != nil {
		t.Fatalf("unexpected error on quit: %v", err)
	}
	if !strings.Contains(out.String(), "MULTIGRAVITY PROFILES") {
		t.Errorf("expected header, got: %s", out.String())
	}

	// Test empty line
	in = bytes.NewBufferString("\n")
	out.Reset()
	err = RunMenu(in, &out, nil)
	if err != nil {
		t.Fatalf("unexpected error on empty input: %v", err)
	}
}

func TestRunMenuSelectNumber(t *testing.T) {
	setupTestEnv(t)

	if err := profile.CreateProfile(profile.CreateOptions{Name: "p1"}); err != nil {
		t.Fatal(err)
	}
	if err := profile.CreateProfile(profile.CreateOptions{Name: "p2"}); err != nil {
		t.Fatal(err)
	}

	var launched string
	mockLauncher := func(name string, args []string) error {
		launched = name
		return nil
	}

	in := bytes.NewBufferString("2\n")
	var out bytes.Buffer
	err := RunMenu(in, &out, mockLauncher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if launched != "p2" {
		t.Errorf("expected profile 'p2' to be launched, got: %s", launched)
	}
	if !strings.Contains(out.String(), "Launching profile: p2 ...") {
		t.Errorf("expected launch message in output: %s", out.String())
	}
}

func TestRunMenuSelectName(t *testing.T) {
	setupTestEnv(t)

	if err := profile.CreateProfile(profile.CreateOptions{Name: "my-project"}); err != nil {
		t.Fatal(err)
	}

	var launched string
	mockLauncher := func(name string, args []string) error {
		launched = name
		return nil
	}

	in := bytes.NewBufferString("my-project\n")
	var out bytes.Buffer
	err := RunMenu(in, &out, mockLauncher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if launched != "my-project" {
		t.Errorf("expected 'my-project' launched, got: %s", launched)
	}
}

func TestRunMenuCreateNew(t *testing.T) {
	setupTestEnv(t)

	if err := profile.CreateProfile(profile.CreateOptions{Name: "initial"}); err != nil {
		t.Fatal(err)
	}

	in := bytes.NewBufferString("n\nbrand-new\n")
	var out bytes.Buffer
	err := RunMenu(in, &out, nil)
	if err != nil {
		t.Fatalf("unexpected error on creating new profile: %v", err)
	}

	if !profile.ProfileExists("brand-new") {
		t.Errorf("expected 'brand-new' profile to be created")
	}
}

func TestRunMenuInvalidSelection(t *testing.T) {
	setupTestEnv(t)

	if err := profile.CreateProfile(profile.CreateOptions{Name: "p1"}); err != nil {
		t.Fatal(err)
	}

	in := bytes.NewBufferString("99\n")
	var out bytes.Buffer
	err := RunMenu(in, &out, nil)
	if err == nil {
		t.Fatalf("expected error for selection 99")
	}
	if !strings.Contains(err.Error(), "invalid selection: 99") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestIsInteractive(t *testing.T) {
	// A closed/invalid fd or pipe in testing should return false or boolean without panic
	_ = IsInteractive(0, 1)
}
