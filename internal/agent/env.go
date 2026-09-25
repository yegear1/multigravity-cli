package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

// DetectAgentType infers the agent type from the command name if not explicitly set.
func DetectAgentType(command string) string {
	base := strings.ToLower(filepath.Base(command))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	fields := strings.Fields(base)
	target := base
	if len(fields) > 0 {
		target = fields[0]
	}

	switch {
	case strings.Contains(target, "claude"):
		return "claude"
	case strings.Contains(target, "aider"):
		return "aider"
	case strings.Contains(target, "opencode") || strings.Contains(target, "open-code"):
		return "opencode"
	case strings.Contains(target, "agy") || strings.Contains(target, "antigravity"):
		return "agy"
	default:
		return "custom"
	}
}

// BuildAgentEnv constructs the environment variables array for an isolated agent process.
func BuildAgentEnv(opts CreateSessionOptions) ([]string, error) {
	userHome, _ := os.UserHomeDir()
	realHome := os.Getenv("REAL_HOME")
	if realHome == "" {
		realHome = userHome
	}

	profileDir := ""
	if opts.Profile != "" {
		if err := config.ValidateProfileName(opts.Profile); err != nil {
			return nil, err
		}
		profileDir = config.GetProfileDir(opts.Profile)
		if _, err := os.Stat(profileDir); os.IsNotExist(err) {
			return nil, fmt.Errorf("profile %q does not exist", opts.Profile)
		}
	} else {
		profileDir = realHome
	}

	// Copy base environment
	envMap := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// 1. Identity & Filesystem Isolation
	enrichedPATH := buildAgentPath(realHome)
	envMap["PATH"] = enrichedPATH
	envMap["REAL_HOME"] = realHome
	envMap["HOME"] = profileDir

	if runtime.GOOS == "windows" {
		envMap["REAL_USERPROFILE"] = realHome
		envMap["USERPROFILE"] = profileDir
		envMap["APPDATA"] = filepath.Join(profileDir, "AppData", "Roaming")
		envMap["LOCALAPPDATA"] = filepath.Join(profileDir, "AppData", "Local")
	} else if runtime.GOOS != "darwin" {
		envMap["XDG_CONFIG_HOME"] = filepath.Join(profileDir, ".config")
		envMap["XDG_CACHE_HOME"] = filepath.Join(profileDir, ".cache")
		envMap["XDG_DATA_HOME"] = filepath.Join(profileDir, ".local", "share")
		envMap["XDG_STATE_HOME"] = filepath.Join(profileDir, ".local", "state")
	}

	// 2. Gateway Environment Injection
	agentType := opts.AgentType
	if agentType == "" {
		agentType = DetectAgentType(opts.Command)
	}

	gatewayURL := strings.TrimRight(opts.GatewayURL, "/")
	gatewayKey := opts.GatewayKey
	if gatewayKey == "" {
		gatewayKey = "multigravity-key"
	}

	if gatewayURL != "" {
		// Ensure v1 endpoint format
		apiBase := gatewayURL
		if !strings.HasSuffix(apiBase, "/v1") {
			apiBase = apiBase + "/v1"
		}

		switch agentType {
		case "claude":
			envMap["ANTHROPIC_BASE_URL"] = apiBase
			envMap["ANTHROPIC_API_KEY"] = gatewayKey
		case "aider":
			envMap["OPENAI_BASE_URL"] = apiBase
			envMap["OPENAI_API_BASE"] = apiBase
			envMap["OPENAI_API_KEY"] = gatewayKey
		case "opencode":
			envMap["OPENAI_BASE_URL"] = apiBase
			envMap["OPENAI_API_KEY"] = gatewayKey
		default:
			// For generic/custom agents, inject both Anthropic and OpenAI compatible URLs
			envMap["ANTHROPIC_BASE_URL"] = apiBase
			envMap["ANTHROPIC_API_KEY"] = gatewayKey
			envMap["OPENAI_BASE_URL"] = apiBase
			envMap["OPENAI_API_BASE"] = apiBase
			envMap["OPENAI_API_KEY"] = gatewayKey
		}
	}

	// 3. Multigravity specific metadata
	envMap["MULTIGRAVITY_PROFILE"] = opts.Profile
	envMap["MULTIGRAVITY_AGENT_TYPE"] = agentType
	if opts.WorktreeID != "" {
		envMap["MULTIGRAVITY_WORKTREE"] = opts.WorktreeID
	}

	// 4. Custom user-supplied overrides
	for k, v := range opts.Env {
		envMap[k] = v
	}

	var finalEnv []string
	for k, v := range envMap {
		finalEnv = append(finalEnv, fmt.Sprintf("%s=%s", k, v))
	}

	return finalEnv, nil
}

func buildAgentPath(hostHome string) string {
	currPath := os.Getenv("PATH")
	var hostBinDirs []string
	if runtime.GOOS == "windows" {
		hostBinDirs = []string{
			filepath.Join(hostHome, ".cargo", "bin"),
			filepath.Join(hostHome, ".local", "bin"),
			filepath.Join(hostHome, "AppData", "Local", "Programs", "Python"),
			filepath.Join(hostHome, "AppData", "Local", "Microsoft", "WinGet", "Links"),
		}
	} else {
		hostBinDirs = []string{
			filepath.Join(hostHome, ".local", "bin"),
			filepath.Join(hostHome, ".cargo", "bin"),
			filepath.Join(hostHome, ".bun", "bin"),
			filepath.Join(hostHome, "go", "bin"),
		}
	}

	pathSep := string(os.PathListSeparator)
	existingParts := strings.Split(currPath, pathSep)
	existingMap := make(map[string]bool)
	for _, p := range existingParts {
		existingMap[p] = true
	}

	var newParts []string
	for _, d := range hostBinDirs {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() && !existingMap[d] {
			newParts = append(newParts, d)
		}
	}

	if len(newParts) == 0 {
		return currPath
	}
	return strings.Join(newParts, pathSep) + pathSep + currPath
}
