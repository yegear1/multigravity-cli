package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

func setupTestSharingEnvironment(t *testing.T) (baseDir, hostHome string) {
	t.Helper()
	baseDir = t.TempDir()
	hostHome = t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", baseDir)
	t.Setenv("REAL_HOME", hostHome)
	t.Setenv("HOME", hostHome)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", t.TempDir())

	// Create host dummy files
	hostMCP := filepath.Join(hostHome, ".gemini", "config", "mcp_config.json")
	if err := os.MkdirAll(filepath.Dir(hostMCP), 0755); err != nil {
		t.Fatalf("failed to create host mcp dir: %v", err)
	}
	if err := os.WriteFile(hostMCP, []byte(`{"mcpServers":{}}`), 0644); err != nil {
		t.Fatalf("failed to write host mcp: %v", err)
	}

	hostSchemas := filepath.Join(hostHome, ".gemini", "antigravity", "mcp")
	if err := os.MkdirAll(hostSchemas, 0755); err != nil {
		t.Fatalf("failed to create host schemas dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hostSchemas, "schema.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to write host schema: %v", err)
	}

	hostSkills := filepath.Join(hostHome, ".gemini", "config", "skills")
	if err := os.MkdirAll(hostSkills, 0755); err != nil {
		t.Fatalf("failed to create host skills dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hostSkills, "dummy.md"), []byte(`skill`), 0644); err != nil {
		t.Fatalf("failed to write host dummy skill: %v", err)
	}

	hostPlugins := filepath.Join(hostHome, ".gemini", "config", "plugins")
	if err := os.MkdirAll(hostPlugins, 0755); err != nil {
		t.Fatalf("failed to create host plugins dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hostPlugins, "plugin.json"), []byte(`plugin`), 0644); err != nil {
		t.Fatalf("failed to write host dummy plugin: %v", err)
	}

	hostConfig := filepath.Join(hostHome, ".gemini", "config", "config.json")
	if err := os.WriteFile(hostConfig, []byte(`{"userSettings":{}}`), 0644); err != nil {
		t.Fatalf("failed to write host config: %v", err)
	}

	hostGH := filepath.Join(hostHome, ".config", "gh")
	if err := os.MkdirAll(hostGH, 0755); err != nil {
		t.Fatalf("failed to create host gh dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hostGH, "hosts.yml"), []byte(`token`), 0644); err != nil {
		t.Fatalf("failed to write host gh: %v", err)
	}

	hostGitconfig := filepath.Join(hostHome, ".gitconfig")
	if err := os.WriteFile(hostGitconfig, []byte("[user]\n\tname = Test\n"), 0644); err != nil {
		t.Fatalf("failed to write host gitconfig: %v", err)
	}

	hostSSH := filepath.Join(hostHome, ".ssh")
	if err := os.MkdirAll(hostSSH, 0700); err != nil {
		t.Fatalf("failed to create host ssh dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hostSSH, "config"), []byte("Host test\n"), 0600); err != nil {
		t.Fatalf("failed to write host ssh config: %v", err)
	}

	return baseDir, hostHome
}

func TestMCPStatusShareIsolate(t *testing.T) {
	_, hostHome := setupTestSharingEnvironment(t)
	profileName := "test-mcp-profile"
	err := CreateProfile(CreateOptions{Name: profileName})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	profileDir := config.GetProfileDir(profileName)
	targetMCP := filepath.Join(profileDir, ".gemini", "config", "mcp_config.json")

	// Status initially shared
	status, err := McpStatus(profileName)
	if err != nil {
		t.Fatalf("failed to get mcp status: %v", err)
	}
	if !strings.Contains(status, "shares host MCP servers") {
		t.Errorf("expected shares host MCP servers, got: %s", status)
	}
	if !isSymlink(targetMCP) {
		t.Errorf("expected targetMCP to be symlink")
	}

	// Isolate
	msgs, err := McpIsolate(profileName)
	if err != nil {
		t.Fatalf("failed to isolate mcp: %v", err)
	}
	if len(msgs) == 0 {
		t.Errorf("expected messages from isolate")
	}

	if !hasSentinel(profileDir, config.SentinelIsolatedMCP) {
		t.Errorf("sentinel %s missing", config.SentinelIsolatedMCP)
	}
	if isSymlink(targetMCP) {
		t.Errorf("targetMCP should not be symlink after isolate")
	}
	if !isRegularFile(targetMCP) {
		t.Errorf("targetMCP should be regular file after isolate")
	}

	status, err = McpStatus(profileName)
	if err != nil {
		t.Fatalf("failed to get mcp status: %v", err)
	}
	if !strings.Contains(status, "has isolated MCP servers") {
		t.Errorf("expected isolated status, got: %s", status)
	}

	// Share again (should backup standalone file to .bak)
	msgs, err = McpShare(profileName)
	if err != nil {
		t.Fatalf("failed to share mcp: %v", err)
	}
	if hasSentinel(profileDir, config.SentinelIsolatedMCP) {
		t.Errorf("sentinel %s should be removed", config.SentinelIsolatedMCP)
	}
	if !isSymlink(targetMCP) {
		t.Errorf("targetMCP should be symlink after share")
	}
	if !isRegularFile(targetMCP + ".bak") {
		t.Errorf("expected backup file %s to exist", targetMCP+".bak")
	}

	// Target MCP should point to host
	target, err := os.Readlink(targetMCP)
	if err != nil {
		t.Fatalf("failed to readlink targetMCP: %v", err)
	}
	expectedTarget := filepath.Join(hostHome, ".gemini", "config", "mcp_config.json")
	if target != expectedTarget {
		t.Errorf("expected target %s, got %s", expectedTarget, target)
	}
}

func TestSkillsStatusShareIsolate(t *testing.T) {
	setupTestSharingEnvironment(t)
	profileName := "test-skills-profile"
	err := CreateProfile(CreateOptions{Name: profileName})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	profileDir := config.GetProfileDir(profileName)
	targetSkills := filepath.Join(profileDir, ".gemini", "config", "skills")
	targetPlugins := filepath.Join(profileDir, ".gemini", "config", "plugins")

	// Status initially shared
	status, err := SkillsStatus(profileName)
	if err != nil {
		t.Fatalf("failed to get skills status: %v", err)
	}
	if !strings.Contains(status, "shares host skills") {
		t.Errorf("expected shares host skills, got: %s", status)
	}
	if !isSymlink(targetSkills) || !isSymlink(targetPlugins) {
		t.Errorf("expected skills and plugins to be symlinks")
	}

	// Isolate
	msgs, err := SkillsIsolate(profileName)
	if err != nil {
		t.Fatalf("failed to isolate skills: %v", err)
	}
	if len(msgs) == 0 {
		t.Errorf("expected messages from isolate")
	}

	if !hasSentinel(profileDir, config.SentinelIsolatedSkills) {
		t.Errorf("sentinel %s missing", config.SentinelIsolatedSkills)
	}
	if isSymlink(targetSkills) || isSymlink(targetPlugins) {
		t.Errorf("skills and plugins should not be symlinks after isolate")
	}
	if !isDir(targetSkills) || !isDir(targetPlugins) {
		t.Errorf("skills and plugins should be regular directories after isolate")
	}

	status, err = SkillsStatus(profileName)
	if err != nil {
		t.Fatalf("failed to get skills status: %v", err)
	}
	if !strings.Contains(status, "has isolated skills/plugins") {
		t.Errorf("expected isolated status, got: %s", status)
	}

	// Share again (should backup standalone dir to .bak)
	msgs, err = SkillsShare(profileName)
	if err != nil {
		t.Fatalf("failed to share skills: %v", err)
	}
	if hasSentinel(profileDir, config.SentinelIsolatedSkills) {
		t.Errorf("sentinel %s should be removed", config.SentinelIsolatedSkills)
	}
	if !isSymlink(targetSkills) || !isSymlink(targetPlugins) {
		t.Errorf("skills and plugins should be symlinks after share")
	}
	if !isDir(targetSkills + ".bak") {
		t.Errorf("expected %s to exist", targetSkills+".bak")
	}
	if !isDir(targetPlugins + ".bak") {
		t.Errorf("expected %s to exist", targetPlugins+".bak")
	}
}

func TestConfigStatusShareIsolateSeed(t *testing.T) {
	_, hostHome := setupTestSharingEnvironment(t)
	profileName := "test-config-profile"
	err := CreateProfile(CreateOptions{Name: profileName})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	profileDir := config.GetProfileDir(profileName)
	targetConfig := filepath.Join(profileDir, ".gemini", "config", "config.json")

	// Status initially shared
	status, err := ConfigStatus(profileName)
	if err != nil {
		t.Fatalf("failed to get config status: %v", err)
	}
	if !strings.Contains(status, "shares host config.json") {
		t.Errorf("expected shares host config.json, got: %s", status)
	}
	if !isSymlink(targetConfig) {
		t.Errorf("expected targetConfig to be symlink")
	}

	// Seed profile (shares host -> seeds host)
	msgs, err := ConfigSeed(profileName)
	if err != nil {
		t.Fatalf("failed to seed config: %v", err)
	}
	if !strings.Contains(msgs[0], "Host config.json seeded") {
		t.Errorf("expected Host config.json seeded, got: %s", msgs[0])
	}

	// Isolate
	msgs, err = ConfigIsolate(profileName)
	if err != nil {
		t.Fatalf("failed to isolate config: %v", err)
	}
	if !hasSentinel(profileDir, config.SentinelIsolatedConfig) {
		t.Errorf("sentinel %s missing", config.SentinelIsolatedConfig)
	}
	if isSymlink(targetConfig) {
		t.Errorf("targetConfig should not be symlink after isolate")
	}
	if !isRegularFile(targetConfig) {
		t.Errorf("targetConfig should be regular file after isolate")
	}

	status, err = ConfigStatus(profileName)
	if err != nil {
		t.Fatalf("failed to get config status: %v", err)
	}
	if !strings.Contains(status, "has isolated configuration") {
		t.Errorf("expected isolated status, got: %s", status)
	}

	// Seed profile standalone
	msgs, err = ConfigSeed(profileName)
	if err != nil {
		t.Fatalf("failed to seed standalone profile: %v", err)
	}
	if !strings.Contains(msgs[0], "Seeded read-only permissions in config.json") {
		t.Errorf("expected seeded message, got: %s", msgs[0])
	}

	// Seed --host
	msgs, err = ConfigSeed("--host")
	if err != nil {
		t.Fatalf("failed to seed host: %v", err)
	}
	if !strings.Contains(msgs[0], "Default read-only permissions seeded in host config.json") {
		t.Errorf("expected host seeded message, got: %s", msgs[0])
	}

	// Seed --all
	msgs, err = ConfigSeed("--all")
	if err != nil {
		t.Fatalf("failed to seed all: %v", err)
	}
	if len(msgs) < 2 {
		t.Errorf("expected at least 2 messages from seed --all, got: %d", len(msgs))
	}

	// Share again (should backup standalone file to .bak)
	msgs, err = ConfigShare(profileName)
	if err != nil {
		t.Fatalf("failed to share config: %v", err)
	}
	if hasSentinel(profileDir, config.SentinelIsolatedConfig) {
		t.Errorf("sentinel %s should be removed", config.SentinelIsolatedConfig)
	}
	if !isSymlink(targetConfig) {
		t.Errorf("targetConfig should be symlink after share")
	}
	if !isRegularFile(targetConfig + ".bak") {
		t.Errorf("expected %s to exist", targetConfig+".bak")
	}

	// Target config should point to host
	target, err := os.Readlink(targetConfig)
	if err != nil {
		t.Fatalf("failed to readlink targetConfig: %v", err)
	}
	expectedTarget := filepath.Join(hostHome, ".gemini", "config", "config.json")
	if target != expectedTarget {
		t.Errorf("expected target %s, got %s", expectedTarget, target)
	}
}

func TestGHStatusShareIsolate(t *testing.T) {
	setupTestSharingEnvironment(t)
	profileName := "test-gh-profile"
	err := CreateProfile(CreateOptions{Name: profileName})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	profileDir := config.GetProfileDir(profileName)
	_, targetGH := getGHPaths(profileDir, getHostHome())

	// Status initially shared
	status, err := GhStatus(profileName)
	if err != nil {
		t.Fatalf("failed to get gh status: %v", err)
	}
	if !strings.Contains(status, "shares host GitHub CLI credentials") {
		t.Errorf("expected shares host GitHub CLI credentials, got: %s", status)
	}
	if !isSymlink(targetGH) {
		t.Errorf("expected targetGH to be symlink")
	}

	// Isolate
	msgs, err := GhIsolate(profileName)
	if err != nil {
		t.Fatalf("failed to isolate gh: %v", err)
	}
	if len(msgs) == 0 {
		t.Errorf("expected messages from isolate")
	}
	if !hasSentinel(profileDir, config.SentinelIsolatedGH) {
		t.Errorf("sentinel %s missing", config.SentinelIsolatedGH)
	}
	if isSymlink(targetGH) {
		t.Errorf("targetGH should not be symlink after isolate")
	}
	if !isDir(targetGH) {
		t.Errorf("targetGH should be regular dir after isolate")
	}

	status, err = GhStatus(profileName)
	if err != nil {
		t.Fatalf("failed to get gh status: %v", err)
	}
	if !strings.Contains(status, "has isolated GitHub CLI credentials") {
		t.Errorf("expected isolated status, got: %s", status)
	}

	// Share again (should backup standalone dir to .bak)
	msgs, err = GhShare(profileName)
	if err != nil {
		t.Fatalf("failed to share gh: %v", err)
	}
	if hasSentinel(profileDir, config.SentinelIsolatedGH) {
		t.Errorf("sentinel %s should be removed", config.SentinelIsolatedGH)
	}
	if !isSymlink(targetGH) {
		t.Errorf("targetGH should be symlink after share")
	}
	if !isDir(targetGH + ".bak") {
		t.Errorf("expected %s to exist", targetGH+".bak")
	}
}

func TestGitStatusShareIsolate(t *testing.T) {
	_, hostHome := setupTestSharingEnvironment(t)
	profileName := "test-git-profile"
	err := CreateProfile(CreateOptions{Name: profileName})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	profileDir := config.GetProfileDir(profileName)
	targetGitconfig := filepath.Join(profileDir, ".gitconfig")
	targetSSH := filepath.Join(profileDir, ".ssh")

	// Status initially shared
	status, err := GitStatus(profileName)
	if err != nil {
		t.Fatalf("failed to get git status: %v", err)
	}
	if !strings.Contains(status, "shares host dev dotfiles") {
		t.Errorf("expected shares host dev dotfiles, got: %s", status)
	}
	if !isSymlink(targetGitconfig) {
		t.Errorf("expected targetGitconfig to be symlink")
	}

	// Isolate
	msgs, err := GitIsolate(profileName)
	if err != nil {
		t.Fatalf("failed to isolate git: %v", err)
	}
	if len(msgs) == 0 {
		t.Errorf("expected messages from isolate")
	}
	if !hasSentinel(profileDir, config.SentinelIsolatedDotfiles) {
		t.Errorf("sentinel %s missing", config.SentinelIsolatedDotfiles)
	}
	if isSymlink(targetGitconfig) {
		t.Errorf("targetGitconfig should not be symlink after isolate")
	}
	if !isRegularFile(targetGitconfig) {
		t.Errorf("targetGitconfig should be regular file after isolate")
	}
	if isSymlink(targetSSH) {
		t.Errorf("targetSSH should not be symlink after isolate")
	}

	status, err = GitStatus(profileName)
	if err != nil {
		t.Fatalf("failed to get git status: %v", err)
	}
	if !strings.Contains(status, "has isolated dev dotfiles") {
		t.Errorf("expected isolated status, got: %s", status)
	}

	// Share again (should backup standalone file to .bak)
	msgs, err = GitShare(profileName)
	if err != nil {
		t.Fatalf("failed to share git: %v", err)
	}
	if hasSentinel(profileDir, config.SentinelIsolatedDotfiles) {
		t.Errorf("sentinel %s should be removed", config.SentinelIsolatedDotfiles)
	}
	if !isSymlink(targetGitconfig) {
		t.Errorf("targetGitconfig should be symlink after share")
	}
	if !isRegularFile(targetGitconfig + ".bak") {
		t.Errorf("expected %s to exist", targetGitconfig+".bak")
	}

	target, err := os.Readlink(targetGitconfig)
	if err != nil {
		t.Fatalf("failed to readlink targetGitconfig: %v", err)
	}
	expectedTarget := filepath.Join(hostHome, ".gitconfig")
	if target != expectedTarget {
		t.Errorf("expected target %s, got %s", expectedTarget, target)
	}

	// GetAllSharingStatus should now contain 5 resources
	allStatuses, err := GetAllSharingStatus(profileName)
	if err != nil {
		t.Fatalf("failed to get all sharing status: %v", err)
	}
	if len(allStatuses) != 5 {
		t.Errorf("expected 5 sharing statuses, got %d", len(allStatuses))
	}
}

func TestSetResourceSharing(t *testing.T) {
	setupTestSharingEnvironment(t)
	profileName := "test-dyn-share-profile"
	err := CreateProfile(CreateOptions{Name: profileName})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	// Test isolate via action "isolate"
	st, msgs, err := SetResourceSharing(profileName, "mcp", "isolate")
	if err != nil {
		t.Fatalf("SetResourceSharing mcp isolate failed: %v", err)
	}
	if st.Mode != "isolated" {
		t.Errorf("expected mode isolated, got %s", st.Mode)
	}
	if len(msgs) == 0 {
		t.Errorf("expected messages")
	}

	// Test toggle (currently isolated -> should become shared)
	st, _, err = SetResourceSharing(profileName, "mcp", "toggle")
	if err != nil {
		t.Fatalf("SetResourceSharing mcp toggle failed: %v", err)
	}
	if st.Mode != "shared" {
		t.Errorf("expected mode shared after toggle, got %s", st.Mode)
	}

	// Test toggle again (currently shared -> should become isolated)
	st, _, err = SetResourceSharing(profileName, "mcp", "toggle")
	if err != nil {
		t.Fatalf("SetResourceSharing mcp toggle failed: %v", err)
	}
	if st.Mode != "isolated" {
		t.Errorf("expected mode isolated after second toggle, got %s", st.Mode)
	}

	// Test share with alias "github" and mode "shared"
	st, _, err = SetResourceSharing(profileName, "github", "shared")
	if err != nil {
		t.Fatalf("SetResourceSharing github shared failed: %v", err)
	}
	if st.Mode != "shared" {
		t.Errorf("expected mode shared for github, got %s", st.Mode)
	}

	// Test dotfiles with alias "git"
	st, _, err = SetResourceSharing(profileName, "git", "isolated")
	if err != nil {
		t.Fatalf("SetResourceSharing git isolated failed: %v", err)
	}
	if st.Mode != "isolated" {
		t.Errorf("expected mode isolated for git, got %s", st.Mode)
	}

	// Test config seed action
	st, msgs, err = SetResourceSharing(profileName, "config", "seed")
	if err != nil {
		t.Fatalf("SetResourceSharing config seed failed: %v", err)
	}
	if len(msgs) == 0 {
		t.Errorf("expected messages for config seed")
	}

	// Test invalid resource
	_, _, err = SetResourceSharing(profileName, "invalid-resource", "share")
	if err == nil {
		t.Errorf("expected error for invalid resource, got nil")
	}

	// Test invalid action
	_, _, err = SetResourceSharing(profileName, "mcp", "invalid-action")
	if err == nil {
		t.Errorf("expected error for invalid action, got nil")
	}
}
