package catalog

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

func TestInspectHostAndPrivateProfile(t *testing.T) {
	home := t.TempDir()
	profiles := t.TempDir()
	t.Setenv("REAL_HOME", home)
	t.Setenv("MULTIGRAVITY_HOME", profiles)

	bin := filepath.Join(home, "fake-mcp")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	writeMCP(t, filepath.Join(home, ".gemini", "config", "mcp_config.json"), `{
		"mcpServers": {
			"filesystem": {
				"command": "`+bin+`",
				"args": ["--token", "super-secret-value"],
				"env": {"TOKEN": "super-secret-value"},
				"headers": {"Authorization": "Bearer super-secret-value"}
			},
			"remote": {
				"url": "https://example.com/mcp?token=super-secret-value"
			}
		}
	}`)
	writeSkill(t, filepath.Join(home, ".gemini", "config", "skills", "demo"), "demo-skill", "Reads the catalog")
	writePlugin(t, filepath.Join(home, ".gemini", "config", "plugins", "desk"), "desk-plugin")

	shared := filepath.Join(profiles, "shared")
	if err := os.MkdirAll(filepath.Join(shared, ".gemini", "config"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, ".gemini", "config", "mcp_config.json"), filepath.Join(shared, ".gemini", "config", "mcp_config.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, ".gemini", "config", "skills"), filepath.Join(shared, ".gemini", "config", "skills")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, ".gemini", "config", "plugins"), filepath.Join(shared, ".gemini", "config", "plugins")); err != nil {
		t.Fatal(err)
	}

	private := filepath.Join(profiles, "private")
	if err := os.MkdirAll(private, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(private, config.SentinelIsolatedMCP), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(private, config.SentinelIsolatedSkills), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	writeMCP(t, filepath.Join(private, ".gemini", "config", "mcp_config.json"), `{
		"mcpServers": {"local": {"command": "`+bin+`"}}
	}`)
	if err := os.MkdirAll(filepath.Join(private, ".gemini", "config", "skills", "bare"), 0755); err != nil {
		t.Fatal(err)
	}

	report, err := Inspect("")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(report)
	if strings.Contains(string(raw), "super-secret-value") {
		t.Fatalf("catalog leaked a secret: %s", raw)
	}
	if !report.Healthy {
		t.Fatalf("expected healthy host catalog, errors=%d warnings=%d checks=%v", report.Errors, report.Warnings, report.Checks)
	}
	if len(report.Servers) != 3 {
		t.Fatalf("servers = %+v", report.Servers)
	}
	byName := map[string]Server{}
	for _, srv := range report.Servers {
		byName[srv.Name] = srv
	}
	if byName["filesystem"].Source != "host" || byName["filesystem"].Transport != "stdio" || byName["filesystem"].Command != bin {
		t.Fatalf("filesystem = %+v", byName["filesystem"])
	}
	if byName["remote"].URL != "https://example.com/mcp" || byName["remote"].Transport != "http" {
		t.Fatalf("remote = %+v", byName["remote"])
	}
	if byName["local"].Source != "private" {
		t.Fatalf("local = %+v", byName["local"])
	}
	skills := map[string]Skill{}
	for _, skill := range report.Skills {
		skills[skill.Source+"/"+skill.Name] = skill
	}
	if skills["host/demo-skill"].Description != "Reads the catalog" || skills["private/bare"].Status != StatusWarning {
		t.Fatalf("skills = %+v", report.Skills)
	}
	if len(report.Plugins) != 1 || report.Plugins[0].Name != "desk-plugin" {
		t.Fatalf("plugins = %+v", report.Plugins)
	}
	modes := map[string]ProfileBinding{}
	for _, binding := range report.Profiles {
		modes[binding.Profile] = binding
	}
	if modes["shared"].MCP != "shared" || modes["private"].MCP != "isolated" || modes["private"].Skills != "isolated" {
		t.Fatalf("bindings = %+v", report.Profiles)
	}

	var buf bytes.Buffer
	Run(&buf, report)
	out := buf.String()
	if strings.Contains(out, "super-secret-value") {
		t.Fatalf("text leaked a secret: %s", out)
	}
	if !strings.Contains(out, "Checking MCP servers and skills...") || !strings.Contains(out, "warning") {
		t.Fatalf("text = %s", out)
	}
	if !strings.Contains(out, "private") || !strings.Contains(out, "missing SKILL.md") {
		t.Fatalf("expected private skill warning in text: %s", out)
	}
}

func TestInspectBrokenTools(t *testing.T) {
	home := t.TempDir()
	profiles := t.TempDir()
	t.Setenv("REAL_HOME", home)
	t.Setenv("MULTIGRAVITY_HOME", profiles)

	writeMCP(t, filepath.Join(home, ".gemini", "config", "mcp_config.json"), `{"mcpServers":{"ghost":{"command":"definitely-missing-bin"}}}`)
	if err := os.MkdirAll(filepath.Join(home, ".gemini", "config"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "missing-skills"), filepath.Join(home, ".gemini", "config", "skills")); err != nil {
		t.Fatal(err)
	}

	broken := filepath.Join(profiles, "broken")
	if err := os.MkdirAll(filepath.Join(broken, ".gemini", "config"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "missing.json"), filepath.Join(broken, ".gemini", "config", "mcp_config.json")); err != nil {
		t.Fatal(err)
	}

	report, err := Inspect("")
	if err != nil {
		t.Fatal(err)
	}
	if report.Healthy || report.Errors < 2 {
		t.Fatalf("expected errors for missing command path? warning plus dangling, report=%+v", report)
	}
	if report.Servers[0].Status != StatusWarning {
		t.Fatalf("missing PATH command should warn, got %+v", report.Servers[0])
	}
	foundDangling := false
	for _, check := range report.Checks {
		if strings.Contains(check.Message, "dangling symlink") {
			foundDangling = true
		}
	}
	if !foundDangling {
		t.Fatalf("expected dangling checks, got %+v", report.Checks)
	}

	one, err := Inspect("broken")
	if err != nil {
		t.Fatal(err)
	}
	if one.Scope != "broken" || one.Errors == 0 {
		t.Fatalf("profile inspect = %+v", one)
	}
}

func TestInspectInvalidJSONAndMissingProfile(t *testing.T) {
	home := t.TempDir()
	profiles := t.TempDir()
	t.Setenv("REAL_HOME", home)
	t.Setenv("MULTIGRAVITY_HOME", profiles)
	writeMCP(t, filepath.Join(home, ".gemini", "config", "mcp_config.json"), `{not-json super-secret-value`)

	report, err := Inspect("")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(report)
	if strings.Contains(string(raw), "super-secret-value") || report.Errors == 0 {
		t.Fatalf("invalid json report leaked or stayed healthy: %s", raw)
	}

	if _, err := Inspect("nope"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing profile err = %v", err)
	}
	if _, err := Inspect("bad name"); err == nil {
		t.Fatal("expected invalid profile name")
	}
}

func TestInspectAbsoluteCommandMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("REAL_HOME", home)
	t.Setenv("MULTIGRAVITY_HOME", t.TempDir())
	missing := filepath.Join(home, "gone")
	writeMCP(t, filepath.Join(home, ".gemini", "config", "mcp_config.json"), `{"mcpServers":{"gone":{"command":"`+missing+`"}}}`)
	report, err := Inspect("")
	if err != nil {
		t.Fatal(err)
	}
	if report.Healthy || report.Servers[0].Status != StatusError {
		t.Fatalf("absolute missing command = %+v healthy=%v", report.Servers, report.Healthy)
	}
}

func writeMCP(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeSkill(t *testing.T, dir, name, desc string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: " + desc + "\n---\n# Skill\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func writePlugin(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"name":"`+name+`"}`), 0644); err != nil {
		t.Fatal(err)
	}
}
