package profile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/app"
	"github.com/ye-dev/multigravity-cli/internal/auth"
	"github.com/ye-dev/multigravity-cli/internal/config"
)

var (
	appRequireFn = app.RequireApp
	runCmdFn     = defaultRunCmd
)

func defaultRunCmd(c *exec.Cmd, wait bool) error {
	if wait {
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	}
	return c.Start()
}

// BuildLaunchCommand prepares the exec.Cmd with isolated environment and platform-specific arguments
func BuildLaunchCommand(name string, forwardArgs []string) (*exec.Cmd, error) {
	if err := config.ValidateProfileName(name); err != nil {
		return nil, err
	}

	profileDir := config.GetProfileDir(name)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist. Run: multigravity new %s", name, name)
	}

	if err := EnsureProfileLayout(profileDir); err != nil {
		return nil, fmt.Errorf("failed to prepare profile layout: %w", err)
	}

	appPath, err := appRequireFn()
	if err != nil {
		return nil, err
	}

	dataDir := config.GetUserDataDir(profileDir)
	extDir := config.GetExtensionsDir(profileDir)

	userHome, _ := os.UserHomeDir()
	realHome := os.Getenv("REAL_HOME")
	if realHome == "" {
		realHome = userHome
	}

	enrichedPATH := buildEnrichedPATH(realHome)

	var cmdName string
	var cmdArgs []string

	switch runtime.GOOS {
	case "darwin":
		cmdName = "open"
		cmdArgs = []string{"-n", appPath, "--args", "--user-data-dir", dataDir, "--extensions-dir", extDir}
		cmdArgs = append(cmdArgs, forwardArgs...)

	case "windows":
		cmdName = appPath
		cmdArgs = forwardArgs

	default: // linux, etc.
		cmdName = appPath
		cmdArgs = []string{"--user-data-dir", dataDir, "--extensions-dir", extDir}
		cmdArgs = append(cmdArgs, forwardArgs...)
	}

	cmd := exec.Command(cmdName, cmdArgs...)

	// Construct isolated environment
	envMap := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	envMap["PATH"] = enrichedPATH
	envMap["REAL_HOME"] = realHome
	envMap["HOME"] = profileDir
	if auth.HasVault(profileDir) {
		envMap["GEMINI_FORCE_FILE_STORAGE"] = "true"
	}

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

	var finalEnv []string
	for k, v := range envMap {
		finalEnv = append(finalEnv, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = finalEnv

	return cmd, nil
}

func defaultLaunchProfileReal(name string, forwardArgs []string) error {
	cmd, err := BuildLaunchCommand(name, forwardArgs)
	if err != nil {
		return err
	}

	fmt.Printf("Launching Antigravity profile %q\n", name)

	// Check if caller wants to wait (e.g. editor mode / git commit)
	wait := false
	for _, arg := range forwardArgs {
		if arg == "--wait" || arg == "-w" {
			wait = true
			break
		}
	}

	return runCmdFn(cmd, wait)
}

func buildEnrichedPATH(hostHome string) string {
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

func init() {
	launchProfileFn = defaultLaunchProfileReal
}
