package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

func getHostHome() string {
	realHome := os.Getenv("REAL_HOME")
	if realHome != "" {
		return realHome
	}
	h, _ := os.UserHomeDir()
	return h
}

func isSymlink(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink != 0
}

func isRegularFile(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.Mode().IsRegular()
}

func isDir(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// SharingStatus represents the sharing/isolation status of a profile resource
type SharingStatus struct {
	Profile     string `json:"profile"`
	Resource    string `json:"resource"`
	Mode        string `json:"mode"`
	Target      string `json:"target,omitempty"`
	Description string `json:"description"`
}

// --- MCP ---

func GetMcpStatus(profile string) (*SharingStatus, error) {
	if err := config.ValidateProfileName(profile); err != nil {
		return nil, err
	}
	profileDir := config.GetProfileDir(profile)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profile)
	}

	targetMCP := filepath.Join(profileDir, ".gemini", "config", "mcp_config.json")

	if hasSentinel(profileDir, config.SentinelIsolatedMCP) {
		return &SharingStatus{
			Profile:     profile,
			Resource:    "mcp",
			Mode:        "isolated",
			Description: fmt.Sprintf("Profile %q has isolated MCP servers (--isolated-mcp active).", profile),
		}, nil
	}
	if isSymlink(targetMCP) {
		target, _ := os.Readlink(targetMCP)
		return &SharingStatus{
			Profile:     profile,
			Resource:    "mcp",
			Mode:        "shared",
			Target:      target,
			Description: fmt.Sprintf("Profile %q shares host MCP servers -> %s", profile, target),
		}, nil
	}
	if isRegularFile(targetMCP) {
		return &SharingStatus{
			Profile:     profile,
			Resource:    "mcp",
			Mode:        "standalone",
			Description: fmt.Sprintf("Profile %q has a standalone local mcp_config.json.", profile),
		}, nil
	}
	return &SharingStatus{
		Profile:     profile,
		Resource:    "mcp",
		Mode:        "none",
		Description: fmt.Sprintf("Profile %q has no MCP servers configured.", profile),
	}, nil
}

func McpStatus(profile string) (string, error) {
	st, err := GetMcpStatus(profile)
	if err != nil {
		return "", err
	}
	return st.Description, nil
}

func McpShare(profile string) ([]string, error) {
	if err := config.ValidateProfileName(profile); err != nil {
		return nil, err
	}
	profileDir := config.GetProfileDir(profile)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profile)
	}

	var messages []string
	_ = os.Remove(filepath.Join(profileDir, config.SentinelIsolatedMCP))

	targetMCP := filepath.Join(profileDir, ".gemini", "config", "mcp_config.json")
	if isSymlink(targetMCP) {
		messages = append(messages, fmt.Sprintf("Profile %q is already sharing host MCP servers.", profile))
		return messages, nil
	}

	if isRegularFile(targetMCP) {
		bak := targetMCP + ".bak"
		_ = os.Rename(targetMCP, bak)
		messages = append(messages, "Backed up existing mcp_config.json to mcp_config.json.bak")
	}

	LinkMCPConfig(profileDir, getHostHome())
	messages = append(messages, fmt.Sprintf("Profile %q is now sharing host MCP servers.", profile))
	return messages, nil
}

func McpIsolate(profile string) ([]string, error) {
	if err := config.ValidateProfileName(profile); err != nil {
		return nil, err
	}
	profileDir := config.GetProfileDir(profile)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profile)
	}

	var messages []string
	_ = touchFile(filepath.Join(profileDir, config.SentinelIsolatedMCP))

	hostHome := getHostHome()
	hostMCP := filepath.Join(hostHome, ".gemini", "config", "mcp_config.json")
	targetMCP := filepath.Join(profileDir, ".gemini", "config", "mcp_config.json")
	targetSchemas := filepath.Join(profileDir, ".gemini", "antigravity", "mcp")
	hostSchemas := filepath.Join(hostHome, ".gemini", "antigravity", "mcp")

	if isSymlink(targetMCP) {
		_ = os.Remove(targetMCP)
		if isRegularFile(hostMCP) {
			_ = CopyFile(hostMCP, targetMCP)
			messages = append(messages, fmt.Sprintf("Copied host MCP config to standalone file for %q.", profile))
		}
	}

	if isSymlink(targetSchemas) {
		_ = os.Remove(targetSchemas)
		if isDir(hostSchemas) {
			_ = CopyDir(hostSchemas, targetSchemas)
		}
	}

	messages = append(messages, fmt.Sprintf("Profile %q is now isolated from host MCP updates.", profile))
	return messages, nil
}

// --- Skills ---

func GetSkillsStatus(profile string) (*SharingStatus, error) {
	if err := config.ValidateProfileName(profile); err != nil {
		return nil, err
	}
	profileDir := config.GetProfileDir(profile)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profile)
	}

	targetSkills := filepath.Join(profileDir, ".gemini", "config", "skills")
	targetPlugins := filepath.Join(profileDir, ".gemini", "config", "plugins")

	if hasSentinel(profileDir, config.SentinelIsolatedSkills) {
		return &SharingStatus{
			Profile:     profile,
			Resource:    "skills",
			Mode:        "isolated",
			Description: fmt.Sprintf("Profile %q has isolated skills/plugins (--isolated-skills active).", profile),
		}, nil
	}
	if isSymlink(targetSkills) || isSymlink(targetPlugins) {
		sTarget := "(none)"
		if isSymlink(targetSkills) {
			sTarget, _ = os.Readlink(targetSkills)
		}
		pTarget := "(none)"
		if isSymlink(targetPlugins) {
			pTarget, _ = os.Readlink(targetPlugins)
		}
		return &SharingStatus{
			Profile:     profile,
			Resource:    "skills",
			Mode:        "shared",
			Target:      sTarget,
			Description: fmt.Sprintf("Profile %q shares host skills -> %s (plugins -> %s)", profile, sTarget, pTarget),
		}, nil
	}
	if isDir(targetSkills) || isDir(targetPlugins) {
		return &SharingStatus{
			Profile:     profile,
			Resource:    "skills",
			Mode:        "standalone",
			Description: fmt.Sprintf("Profile %q has standalone local skills/plugins.", profile),
		}, nil
	}
	return &SharingStatus{
		Profile:     profile,
		Resource:    "skills",
		Mode:        "none",
		Description: fmt.Sprintf("Profile %q has no custom skills or plugins configured.", profile),
	}, nil
}

func SkillsStatus(profile string) (string, error) {
	st, err := GetSkillsStatus(profile)
	if err != nil {
		return "", err
	}
	return st.Description, nil
}


func SkillsShare(profile string) ([]string, error) {
	if err := config.ValidateProfileName(profile); err != nil {
		return nil, err
	}
	profileDir := config.GetProfileDir(profile)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profile)
	}

	var messages []string
	_ = os.Remove(filepath.Join(profileDir, config.SentinelIsolatedSkills))

	targetSkills := filepath.Join(profileDir, ".gemini", "config", "skills")
	targetPlugins := filepath.Join(profileDir, ".gemini", "config", "plugins")

	if isSymlink(targetSkills) && isSymlink(targetPlugins) {
		messages = append(messages, fmt.Sprintf("Profile %q is already sharing host skills and plugins.", profile))
		return messages, nil
	}

	if isDir(targetSkills) && !isSymlink(targetSkills) {
		_ = os.Rename(targetSkills, targetSkills+".bak")
		messages = append(messages, "Backed up existing skills directory to skills.bak")
	}
	if isDir(targetPlugins) && !isSymlink(targetPlugins) {
		_ = os.Rename(targetPlugins, targetPlugins+".bak")
		messages = append(messages, "Backed up existing plugins directory to plugins.bak")
	}

	LinkSkillsConfig(profileDir, getHostHome())
	messages = append(messages, fmt.Sprintf("Profile %q is now sharing host skills and plugins.", profile))
	return messages, nil
}

func SkillsIsolate(profile string) ([]string, error) {
	if err := config.ValidateProfileName(profile); err != nil {
		return nil, err
	}
	profileDir := config.GetProfileDir(profile)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profile)
	}

	var messages []string
	_ = touchFile(filepath.Join(profileDir, config.SentinelIsolatedSkills))

	hostHome := getHostHome()
	hostSkills := filepath.Join(hostHome, ".gemini", "config", "skills")
	hostPlugins := filepath.Join(hostHome, ".gemini", "config", "plugins")
	targetSkills := filepath.Join(profileDir, ".gemini", "config", "skills")
	targetPlugins := filepath.Join(profileDir, ".gemini", "config", "plugins")

	if isSymlink(targetSkills) {
		_ = os.Remove(targetSkills)
		if isDir(hostSkills) {
			_ = CopyDir(hostSkills, targetSkills)
			messages = append(messages, fmt.Sprintf("Copied host skills to standalone directory for %q.", profile))
		}
	}

	if isSymlink(targetPlugins) {
		_ = os.Remove(targetPlugins)
		if isDir(hostPlugins) {
			_ = CopyDir(hostPlugins, targetPlugins)
			messages = append(messages, fmt.Sprintf("Copied host plugins to standalone directory for %q.", profile))
		}
	}

	messages = append(messages, fmt.Sprintf("Profile %q is now isolated from host skills and plugins updates.", profile))
	return messages, nil
}

// --- Config ---

func GetConfigStatus(profile string) (*SharingStatus, error) {
	if err := config.ValidateProfileName(profile); err != nil {
		return nil, err
	}
	profileDir := config.GetProfileDir(profile)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profile)
	}

	targetConfig := filepath.Join(profileDir, ".gemini", "config", "config.json")

	if hasSentinel(profileDir, config.SentinelIsolatedConfig) {
		return &SharingStatus{
			Profile:     profile,
			Resource:    "config",
			Mode:        "isolated",
			Description: fmt.Sprintf("Profile %q has isolated configuration (--isolated-config active).", profile),
		}, nil
	}
	if isSymlink(targetConfig) {
		target, _ := os.Readlink(targetConfig)
		return &SharingStatus{
			Profile:     profile,
			Resource:    "config",
			Mode:        "shared",
			Target:      target,
			Description: fmt.Sprintf("Profile %q shares host config.json -> %s", profile, target),
		}, nil
	}
	if isRegularFile(targetConfig) {
		return &SharingStatus{
			Profile:     profile,
			Resource:    "config",
			Mode:        "standalone",
			Description: fmt.Sprintf("Profile %q has a standalone local config.json.", profile),
		}, nil
	}
	return &SharingStatus{
		Profile:     profile,
		Resource:    "config",
		Mode:        "none",
		Description: fmt.Sprintf("Profile %q has no config.json configured.", profile),
	}, nil
}

func ConfigStatus(profile string) (string, error) {
	st, err := GetConfigStatus(profile)
	if err != nil {
		return "", err
	}
	return st.Description, nil
}


func ConfigShare(profile string) ([]string, error) {
	if err := config.ValidateProfileName(profile); err != nil {
		return nil, err
	}
	profileDir := config.GetProfileDir(profile)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profile)
	}

	var messages []string
	_ = os.Remove(filepath.Join(profileDir, config.SentinelIsolatedConfig))

	targetConfig := filepath.Join(profileDir, ".gemini", "config", "config.json")
	if isSymlink(targetConfig) {
		messages = append(messages, fmt.Sprintf("Profile %q is already sharing host config.json.", profile))
		return messages, nil
	}

	if isRegularFile(targetConfig) {
		_ = os.Rename(targetConfig, targetConfig+".bak")
		messages = append(messages, "Backed up existing config.json to config.json.bak")
	}

	LinkUserConfig(profileDir, getHostHome())
	messages = append(messages, fmt.Sprintf("Profile %q is now sharing host config.json.", profile))
	return messages, nil
}

func ConfigIsolate(profile string) ([]string, error) {
	if err := config.ValidateProfileName(profile); err != nil {
		return nil, err
	}
	profileDir := config.GetProfileDir(profile)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profile)
	}

	var messages []string
	_ = touchFile(filepath.Join(profileDir, config.SentinelIsolatedConfig))

	hostHome := getHostHome()
	hostConfig := filepath.Join(hostHome, ".gemini", "config", "config.json")
	targetConfig := filepath.Join(profileDir, ".gemini", "config", "config.json")

	if isSymlink(targetConfig) {
		_ = os.Remove(targetConfig)
		if isRegularFile(hostConfig) {
			_ = CopyFile(hostConfig, targetConfig)
			messages = append(messages, fmt.Sprintf("Copied host config.json to standalone file for %q.", profile))
		}
	}

	_, _ = SeedDefaultPermissions(targetConfig)
	messages = append(messages, fmt.Sprintf("Profile %q is now isolated from host config updates.", profile))
	return messages, nil
}

func ConfigSeed(targetArg string) ([]string, error) {
	var messages []string
	hostHome := getHostHome()
	hostConfig := filepath.Join(hostHome, ".gemini", "config", "config.json")

	if targetArg == "--host" {
		_, _ = SeedDefaultPermissions(hostConfig)
		messages = append(messages, "Default read-only permissions seeded in host config.json")
		return messages, nil
	}

	if targetArg == "--all" || targetArg == "" {
		_, _ = SeedDefaultPermissions(hostConfig)
		messages = append(messages, "Default read-only permissions seeded in host config.json")

		profiles, err := ListProfiles()
		if err == nil {
			for _, p := range profiles {
				pDir := config.GetProfileDir(p)
				pCfg := filepath.Join(pDir, ".gemini", "config", "config.json")
				if isRegularFile(pCfg) && !isSymlink(pCfg) {
					_, _ = SeedDefaultPermissions(pCfg)
					messages = append(messages, fmt.Sprintf("Seeded read-only permissions in standalone config for %q", p))
				}
			}
		}
		return messages, nil
	}

	if err := config.ValidateProfileName(targetArg); err != nil {
		return nil, err
	}

	profileDir := config.GetProfileDir(targetArg)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", targetArg)
	}

	pCfg := filepath.Join(profileDir, ".gemini", "config", "config.json")
	if isSymlink(pCfg) {
		_, _ = SeedDefaultPermissions(hostConfig)
		messages = append(messages, fmt.Sprintf("Profile %q shares host config. Host config.json seeded.", targetArg))
	} else {
		_, _ = SeedDefaultPermissions(pCfg)
		messages = append(messages, fmt.Sprintf("Seeded read-only permissions in config.json for %q.", targetArg))
	}

	return messages, nil
}

// --- GitHub CLI ---

func getGHPaths(profileDir, hostHome string) (hostGH, targetGH string) {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		hostGH = filepath.Join(appData, "GitHub CLI")
		targetGH = filepath.Join(profileDir, "AppData", "Roaming", "GitHub CLI")
	} else {
		hostGH = filepath.Join(hostHome, ".config", "gh")
		targetGH = filepath.Join(profileDir, ".config", "gh")
	}
	return hostGH, targetGH
}

func GetGhStatus(profile string) (*SharingStatus, error) {
	if err := config.ValidateProfileName(profile); err != nil {
		return nil, err
	}
	profileDir := config.GetProfileDir(profile)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profile)
	}

	_, targetGH := getGHPaths(profileDir, getHostHome())

	if hasSentinel(profileDir, config.SentinelIsolatedGH) {
		return &SharingStatus{
			Profile:     profile,
			Resource:    "gh",
			Mode:        "isolated",
			Description: fmt.Sprintf("Profile %q has isolated GitHub CLI credentials (--isolated-gh active).", profile),
		}, nil
	}
	if hasSentinel(profileDir, config.SentinelIsolatedDotfiles) {
		return &SharingStatus{
			Profile:     profile,
			Resource:    "gh",
			Mode:        "isolated",
			Description: fmt.Sprintf("Profile %q has isolated dotfiles (--isolated-dotfiles active).", profile),
		}, nil
	}
	if isSymlink(targetGH) {
		target, _ := os.Readlink(targetGH)
		return &SharingStatus{
			Profile:     profile,
			Resource:    "gh",
			Mode:        "shared",
			Target:      target,
			Description: fmt.Sprintf("Profile %q shares host GitHub CLI credentials -> %s", profile, target),
		}, nil
	}
	if isDir(targetGH) {
		return &SharingStatus{
			Profile:     profile,
			Resource:    "gh",
			Mode:        "standalone",
			Description: fmt.Sprintf("Profile %q has standalone local GitHub CLI credentials.", profile),
		}, nil
	}
	return &SharingStatus{
		Profile:     profile,
		Resource:    "gh",
		Mode:        "none",
		Description: fmt.Sprintf("Profile %q has no GitHub CLI credentials configured.", profile),
	}, nil
}

func GhStatus(profile string) (string, error) {
	st, err := GetGhStatus(profile)
	if err != nil {
		return "", err
	}
	return st.Description, nil
}

// GetAllSharingStatus returns sharing status for all resources (mcp, skills, config, gh, git)
func GetAllSharingStatus(profile string) ([]SharingStatus, error) {
	mcpSt, err := GetMcpStatus(profile)
	if err != nil {
		return nil, err
	}
	skillsSt, err := GetSkillsStatus(profile)
	if err != nil {
		return nil, err
	}
	configSt, err := GetConfigStatus(profile)
	if err != nil {
		return nil, err
	}
	ghSt, err := GetGhStatus(profile)
	if err != nil {
		return nil, err
	}
	gitSt, err := GetGitStatus(profile)
	if err != nil {
		return nil, err
	}

	return []SharingStatus{*mcpSt, *skillsSt, *configSt, *ghSt, *gitSt}, nil
}


func GhShare(profile string) ([]string, error) {
	if err := config.ValidateProfileName(profile); err != nil {
		return nil, err
	}
	profileDir := config.GetProfileDir(profile)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profile)
	}

	var messages []string
	_ = os.Remove(filepath.Join(profileDir, config.SentinelIsolatedGH))

	hostHome := getHostHome()
	_, targetGH := getGHPaths(profileDir, hostHome)

	if isSymlink(targetGH) {
		messages = append(messages, fmt.Sprintf("Profile %q is already sharing host GitHub CLI credentials.", profile))
		return messages, nil
	}

	if isDir(targetGH) && !isSymlink(targetGH) {
		_ = os.Rename(targetGH, targetGH+".bak")
		messages = append(messages, "Backed up existing gh directory to gh.bak")
	}

	LinkGHConfig(profileDir, hostHome)
	messages = append(messages, fmt.Sprintf("Profile %q is now sharing host GitHub CLI credentials.", profile))
	return messages, nil
}

func GhIsolate(profile string) ([]string, error) {
	if err := config.ValidateProfileName(profile); err != nil {
		return nil, err
	}
	profileDir := config.GetProfileDir(profile)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profile)
	}

	var messages []string
	_ = touchFile(filepath.Join(profileDir, config.SentinelIsolatedGH))

	hostHome := getHostHome()
	hostGH, targetGH := getGHPaths(profileDir, hostHome)

	if isSymlink(targetGH) {
		_ = os.Remove(targetGH)
		if isDir(hostGH) {
			_ = CopyDir(hostGH, targetGH)
			messages = append(messages, fmt.Sprintf("Copied host GitHub CLI credentials to standalone directory for %q.", profile))
		}
	}

	messages = append(messages, fmt.Sprintf("Profile %q is now isolated from host GitHub CLI credentials updates.", profile))
	return messages, nil
}

// --- Git / Dev Dotfiles ---

func GetDotfilesStatus(profile string) (*SharingStatus, error) {
	if err := config.ValidateProfileName(profile); err != nil {
		return nil, err
	}
	profileDir := config.GetProfileDir(profile)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profile)
	}

	targetGitconfig := filepath.Join(profileDir, ".gitconfig")
	targetSSH := filepath.Join(profileDir, ".ssh")

	if hasSentinel(profileDir, config.SentinelIsolatedDotfiles) {
		return &SharingStatus{
			Profile:     profile,
			Resource:    "git",
			Mode:        "isolated",
			Description: fmt.Sprintf("Profile %q has isolated dev dotfiles (--isolated-dotfiles active).", profile),
		}, nil
	}

	if isSymlink(targetGitconfig) || isSymlink(targetSSH) {
		target := "(none)"
		if isSymlink(targetGitconfig) {
			target, _ = os.Readlink(targetGitconfig)
		} else if isSymlink(targetSSH) {
			target, _ = os.Readlink(targetSSH)
		}
		return &SharingStatus{
			Profile:     profile,
			Resource:    "git",
			Mode:        "shared",
			Target:      target,
			Description: fmt.Sprintf("Profile %q shares host dev dotfiles (.gitconfig, .ssh).", profile),
		}, nil
	}

	if isRegularFile(targetGitconfig) || isDir(targetSSH) {
		return &SharingStatus{
			Profile:     profile,
			Resource:    "git",
			Mode:        "standalone",
			Description: fmt.Sprintf("Profile %q has standalone local dev dotfiles.", profile),
		}, nil
	}

	return &SharingStatus{
		Profile:     profile,
		Resource:    "git",
		Mode:        "none",
		Description: fmt.Sprintf("Profile %q has no dev dotfiles configured.", profile),
	}, nil
}

func GetGitStatus(profile string) (*SharingStatus, error) {
	return GetDotfilesStatus(profile)
}

func DotfilesStatus(profile string) (string, error) {
	st, err := GetDotfilesStatus(profile)
	if err != nil {
		return "", err
	}
	return st.Description, nil
}

func GitStatus(profile string) (string, error) {
	return DotfilesStatus(profile)
}

func DotfilesShare(profile string) ([]string, error) {
	if err := config.ValidateProfileName(profile); err != nil {
		return nil, err
	}
	profileDir := config.GetProfileDir(profile)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profile)
	}

	var messages []string
	_ = os.Remove(filepath.Join(profileDir, config.SentinelIsolatedDotfiles))

	targetGitconfig := filepath.Join(profileDir, ".gitconfig")
	targetSSH := filepath.Join(profileDir, ".ssh")

	if isSymlink(targetGitconfig) && isSymlink(targetSSH) {
		messages = append(messages, fmt.Sprintf("Profile %q is already sharing host dev dotfiles.", profile))
		return messages, nil
	}

	if isRegularFile(targetGitconfig) && !isSymlink(targetGitconfig) {
		_ = os.Rename(targetGitconfig, targetGitconfig+".bak")
		messages = append(messages, "Backed up existing .gitconfig to .gitconfig.bak")
	}
	if isDir(targetSSH) && !isSymlink(targetSSH) {
		_ = os.Rename(targetSSH, targetSSH+".bak")
		messages = append(messages, "Backed up existing .ssh to .ssh.bak")
	}

	LinkDevDotfiles(profileDir, getHostHome())
	messages = append(messages, fmt.Sprintf("Profile %q is now sharing host dev dotfiles.", profile))
	return messages, nil
}

func GitShare(profile string) ([]string, error) {
	return DotfilesShare(profile)
}

func DotfilesIsolate(profile string) ([]string, error) {
	if err := config.ValidateProfileName(profile); err != nil {
		return nil, err
	}
	profileDir := config.GetProfileDir(profile)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profile)
	}

	var messages []string
	_ = touchFile(filepath.Join(profileDir, config.SentinelIsolatedDotfiles))

	hostHome := getHostHome()
	targetGitconfig := filepath.Join(profileDir, ".gitconfig")
	hostGitconfig := filepath.Join(hostHome, ".gitconfig")
	if isSymlink(targetGitconfig) {
		_ = os.Remove(targetGitconfig)
		if isRegularFile(hostGitconfig) {
			_ = CopyFile(hostGitconfig, targetGitconfig)
			messages = append(messages, fmt.Sprintf("Copied host .gitconfig to standalone file for %q.", profile))
		}
	}

	targetSSH := filepath.Join(profileDir, ".ssh")
	hostSSH := filepath.Join(hostHome, ".ssh")
	if isSymlink(targetSSH) {
		_ = os.Remove(targetSSH)
		if isDir(hostSSH) {
			_ = CopyDir(hostSSH, targetSSH)
			messages = append(messages, fmt.Sprintf("Copied host .ssh to standalone directory for %q.", profile))
		}
	}

	targetCreds := filepath.Join(profileDir, ".git-credentials")
	hostCreds := filepath.Join(hostHome, ".git-credentials")
	if isSymlink(targetCreds) {
		_ = os.Remove(targetCreds)
		if isRegularFile(hostCreds) {
			_ = CopyFile(hostCreds, targetCreds)
		}
	}

	targetGnupg := filepath.Join(profileDir, ".gnupg")
	if isSymlink(targetGnupg) {
		_ = os.Remove(targetGnupg)
	}

	messages = append(messages, fmt.Sprintf("Profile %q is now isolated from host dev dotfiles updates.", profile))
	return messages, nil
}

func GitIsolate(profile string) ([]string, error) {
	return DotfilesIsolate(profile)
}

// SetResourceSharing mutates or toggles the sharing mode of a specific resource.
// Accepted resources: "mcp", "skills", "config", "gh", "github", "git", "dotfiles".
// Accepted actions: "share", "shared", "isolate", "isolated", "toggle", "seed" (config only).
func SetResourceSharing(profName, resource, action string) (*SharingStatus, []string, error) {
	if err := config.ValidateProfileName(profName); err != nil {
		return nil, nil, err
	}
	profileDir := config.GetProfileDir(profName)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("profile %q does not exist", profName)
	}

	normalizedResource := strings.ToLower(strings.TrimSpace(resource))
	switch normalizedResource {
	case "github":
		normalizedResource = "gh"
	case "dotfiles":
		normalizedResource = "git"
	}

	normalizedAction := strings.ToLower(strings.TrimSpace(action))
	switch normalizedAction {
	case "shared", "true", "enable", "on":
		normalizedAction = "share"
	case "isolated", "false", "disable", "off":
		normalizedAction = "isolate"
	}

	if normalizedAction == "toggle" {
		var curStatus *SharingStatus
		var err error
		switch normalizedResource {
		case "mcp":
			curStatus, err = GetMcpStatus(profName)
		case "skills":
			curStatus, err = GetSkillsStatus(profName)
		case "config":
			curStatus, err = GetConfigStatus(profName)
		case "gh":
			curStatus, err = GetGhStatus(profName)
		case "git":
			curStatus, err = GetGitStatus(profName)
		default:
			return nil, nil, fmt.Errorf("invalid resource %q: must be mcp, skills, config, gh, or git", resource)
		}
		if err != nil {
			return nil, nil, err
		}
		if curStatus.Mode == "shared" {
			normalizedAction = "isolate"
		} else {
			normalizedAction = "share"
		}
	}

	var messages []string
	var err error

	switch normalizedResource {
	case "mcp":
		switch normalizedAction {
		case "share":
			messages, err = McpShare(profName)
		case "isolate":
			messages, err = McpIsolate(profName)
		default:
			return nil, nil, fmt.Errorf("invalid action %q for mcp: must be share, isolate, or toggle", action)
		}
	case "skills":
		switch normalizedAction {
		case "share":
			messages, err = SkillsShare(profName)
		case "isolate":
			messages, err = SkillsIsolate(profName)
		default:
			return nil, nil, fmt.Errorf("invalid action %q for skills: must be share, isolate, or toggle", action)
		}
	case "config":
		switch normalizedAction {
		case "share":
			messages, err = ConfigShare(profName)
		case "isolate":
			messages, err = ConfigIsolate(profName)
		case "seed":
			messages, err = ConfigSeed(profName)
		default:
			return nil, nil, fmt.Errorf("invalid action %q for config: must be share, isolate, seed, or toggle", action)
		}
	case "gh":
		switch normalizedAction {
		case "share":
			messages, err = GhShare(profName)
		case "isolate":
			messages, err = GhIsolate(profName)
		default:
			return nil, nil, fmt.Errorf("invalid action %q for gh: must be share, isolate, or toggle", action)
		}
	case "git":
		switch normalizedAction {
		case "share":
			messages, err = GitShare(profName)
		case "isolate":
			messages, err = GitIsolate(profName)
		default:
			return nil, nil, fmt.Errorf("invalid action %q for git: must be share, isolate, or toggle", action)
		}
	default:
		return nil, nil, fmt.Errorf("invalid resource %q: must be mcp, skills, config, gh, or git", resource)
	}

	if err != nil {
		return nil, nil, err
	}

	var newStatus *SharingStatus
	switch normalizedResource {
	case "mcp":
		newStatus, err = GetMcpStatus(profName)
	case "skills":
		newStatus, err = GetSkillsStatus(profName)
	case "config":
		newStatus, err = GetConfigStatus(profName)
	case "gh":
		newStatus, err = GetGhStatus(profName)
	case "git":
		newStatus, err = GetGitStatus(profName)
	}
	if err != nil {
		return nil, nil, err
	}

	return newStatus, messages, nil
}
