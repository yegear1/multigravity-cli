package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

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

// GetAllSharingStatus returns sharing status for all resources (mcp, skills, config, gh)
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

	return []SharingStatus{*mcpSt, *skillsSt, *configSt, *ghSt}, nil
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
