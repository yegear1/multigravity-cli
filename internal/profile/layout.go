package profile

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

// EnsureProfileLayout creates the directory hierarchy and symlinks default shared assets
func EnsureProfileLayout(profileDir string) error {
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(config.GetExtensionsDir(profileDir), 0755); err != nil {
		return err
	}

	userHome, _ := os.UserHomeDir()
	realHome := os.Getenv("REAL_HOME")
	if realHome == "" {
		realHome = userHome
	}

	switch runtime.GOOS {
	case "darwin":
		_ = os.MkdirAll(filepath.Join(profileDir, "Library", "Application Support"), 0755)
		keychains := filepath.Join(realHome, "Library", "Keychains")
		profileKeychains := filepath.Join(profileDir, "Library", "Keychains")
		if _, err := os.Stat(keychains); err == nil {
			if _, err := os.Lstat(profileKeychains); os.IsNotExist(err) {
				_ = os.Symlink(keychains, profileKeychains)
			}
		}
	case "windows":
		_ = os.MkdirAll(filepath.Join(profileDir, "AppData", "Roaming", "Antigravity"), 0755)
		_ = os.MkdirAll(filepath.Join(profileDir, "AppData", "Local"), 0755)
	default: // linux, etc.
		_ = os.MkdirAll(filepath.Join(profileDir, ".config", "Antigravity"), 0755)
		_ = os.MkdirAll(filepath.Join(profileDir, ".cache"), 0755)
		_ = os.MkdirAll(filepath.Join(profileDir, ".local", "share"), 0755)
		_ = os.MkdirAll(filepath.Join(profileDir, ".local", "state"), 0755)
	}

	LinkDevDotfiles(profileDir, realHome)
	LinkMCPConfig(profileDir, realHome)
	LinkSkillsConfig(profileDir, realHome)
	LinkUserConfig(profileDir, realHome)
	LinkGHConfig(profileDir, realHome)

	return nil
}

// LinkDevDotfiles symlinks .gitconfig, .ssh, .gnupg, and .git-credentials if not isolated
func LinkDevDotfiles(profileDir, hostHome string) {
	if hasSentinel(profileDir, config.SentinelIsolatedDotfiles) {
		return
	}

	symlinkIfMissing(filepath.Join(hostHome, ".gitconfig"), filepath.Join(profileDir, ".gitconfig"))
	symlinkIfMissing(filepath.Join(hostHome, ".ssh"), filepath.Join(profileDir, ".ssh"))
	symlinkIfMissing(filepath.Join(hostHome, ".gnupg"), filepath.Join(profileDir, ".gnupg"))
	symlinkIfMissing(filepath.Join(hostHome, ".git-credentials"), filepath.Join(profileDir, ".git-credentials"))
}

// LinkMCPConfig symlinks host mcp_config.json and antigravity/mcp schemas if not isolated
func LinkMCPConfig(profileDir, hostHome string) {
	if hasSentinel(profileDir, config.SentinelIsolatedMCP) {
		return
	}

	hostMCP := filepath.Join(hostHome, ".gemini", "config", "mcp_config.json")
	targetMCP := filepath.Join(profileDir, ".gemini", "config", "mcp_config.json")
	symlinkIfMissing(hostMCP, targetMCP)

	hostSchemas := filepath.Join(hostHome, ".gemini", "antigravity", "mcp")
	targetSchemas := filepath.Join(profileDir, ".gemini", "antigravity", "mcp")
	symlinkIfMissing(hostSchemas, targetSchemas)
}

// LinkSkillsConfig symlinks host skills and plugins if not isolated
func LinkSkillsConfig(profileDir, hostHome string) {
	if hasSentinel(profileDir, config.SentinelIsolatedSkills) {
		return
	}

	hostSkills := filepath.Join(hostHome, ".gemini", "config", "skills")
	targetSkills := filepath.Join(profileDir, ".gemini", "config", "skills")
	symlinkIfMissing(hostSkills, targetSkills)

	hostPlugins := filepath.Join(hostHome, ".gemini", "config", "plugins")
	targetPlugins := filepath.Join(profileDir, ".gemini", "config", "plugins")
	symlinkIfMissing(hostPlugins, targetPlugins)
}

// LinkUserConfig symlinks host config.json if not isolated
func LinkUserConfig(profileDir, hostHome string) {
	targetCfg := filepath.Join(profileDir, ".gemini", "config", "config.json")
	hostCfg := filepath.Join(hostHome, ".gemini", "config", "config.json")

	if hasSentinel(profileDir, config.SentinelIsolatedConfig) {
		_, _ = SeedDefaultPermissions(targetCfg)
		return
	}

	_, _ = SeedDefaultPermissions(hostCfg)
	symlinkIfMissing(hostCfg, targetCfg)
}

// LinkGHConfig symlinks host GitHub CLI configuration if not isolated
func LinkGHConfig(profileDir, hostHome string) {
	if hasSentinel(profileDir, config.SentinelIsolatedGH) || hasSentinel(profileDir, config.SentinelIsolatedDotfiles) {
		return
	}

	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		hostGH := filepath.Join(appData, "GitHub CLI")
		targetGH := filepath.Join(profileDir, "AppData", "Roaming", "GitHub CLI")
		symlinkIfMissing(hostGH, targetGH)
	} else {
		hostGH := filepath.Join(hostHome, ".config", "gh")
		targetGH := filepath.Join(profileDir, ".config", "gh")
		symlinkIfMissing(hostGH, targetGH)
	}
}

func hasSentinel(profileDir, sentinel string) bool {
	_, err := os.Stat(filepath.Join(profileDir, sentinel))
	return err == nil
}

func symlinkIfMissing(src, dest string) {
	if _, err := os.Stat(src); err != nil {
		return // Source does not exist on host
	}
	if _, err := os.Lstat(dest); err == nil {
		return // Destination already exists (symlink or file)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return
	}

	_ = os.Symlink(src, dest)
}
