package headless

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/app"
	"github.com/ye-dev/multigravity-cli/internal/auth"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

var (
	httpsPortRegex = regexp.MustCompile(`at (\d+) for HTTPS`)

	// Test hooks for hermetic unit testing
	findLanguageServerFn = app.FindLanguageServer
	probeHealthFn        = defaultProbeHealth
	isProcessAliveHook   = isProcessAlive
	terminateProcessHook = terminateProcess
	startProcessHook     func(cmd *exec.Cmd, logFile string, portChan chan int, errChan chan error) (*InstanceInfo, error)

	defaultManager *Manager
	once           sync.Once
)

// GetDefaultManager returns the singleton headless manager.
func GetDefaultManager() *Manager {
	once.Do(func() {
		defaultManager = NewManager()
	})
	return defaultManager
}

// Manager orchestrates background headless language server instances.
type Manager struct {
	mu sync.RWMutex
}

// NewManager creates a new Manager instance.
func NewManager() *Manager {
	return &Manager{}
}

// SetTestHooks allows tests to inject custom execution and health probe logic.
func SetTestHooks(
	findLS func() (string, error),
	probeHealth func(port int, csrf string) error,
	isAlive func(pid int) bool,
	terminate func(pid int) error,
	startProc func(cmd *exec.Cmd, logFile string, portChan chan int, errChan chan error) (*InstanceInfo, error),
) func() {
	origFindLS := findLanguageServerFn
	origProbe := probeHealthFn
	origIsAlive := isProcessAliveHook
	origTerminate := terminateProcessHook
	origStartProc := startProcessHook

	if findLS != nil {
		findLanguageServerFn = findLS
	}
	if probeHealth != nil {
		probeHealthFn = probeHealth
	}
	if isAlive != nil {
		isProcessAliveHook = isAlive
	}
	if terminate != nil {
		terminateProcessHook = terminate
	}
	if startProc != nil {
		startProcessHook = startProc
	}

	return func() {
		findLanguageServerFn = origFindLS
		probeHealthFn = origProbe
		isProcessAliveHook = origIsAlive
		terminateProcessHook = origTerminate
		startProcessHook = origStartProc
	}
}

func defaultProbeHealth(port int, csrf string) error {
	client := quota.NewClient(1500 * time.Millisecond)
	_, err := client.RetrieveUserQuotaSummary(port, csrf)
	return err
}

func getHeadlessStatePath(profileName string) string {
	profileDir := config.GetProfileDir(profileName)
	return filepath.Join(profileDir, ".multigravity", "headless.json")
}

func getHeadlessLogPath(profileName string) string {
	profileDir := config.GetProfileDir(profileName)
	return filepath.Join(profileDir, ".multigravity", "headless.log")
}

func readStateFile(profileName string) (*InstanceInfo, error) {
	statePath := getHeadlessStatePath(profileName)
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, err
	}
	var info InstanceInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func writeStateFile(profileName string, info *InstanceInfo) error {
	statePath := getHeadlessStatePath(profileName)
	dir := filepath.Dir(statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, data, 0644)
}

func removeStateFile(profileName string) {
	statePath := getHeadlessStatePath(profileName)
	_ = os.Remove(statePath)
}

// reapDeadState removes the headless state file when its PID is no longer alive.
// A nil result means the file is absent or the process is still running.
func reapDeadState(profileName string) (*InstanceInfo, error) {
	info, err := readStateFile(profileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if isProcessAliveHook(info.PID) {
		return nil, nil
	}
	removeStateFile(profileName)
	info.Status = StateStopped
	info.HealthError = ""
	return info, nil
}

// ReapStale removes a headless state file whose process has already exited.
// Alive instances are left untouched. Callers use the returned record as the
// orphan-process alert; GetStatus uses the same removal.
func (m *Manager) ReapStale(profileName string) (*InstanceInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := config.ValidateProfileName(profileName); err != nil {
		return nil, err
	}
	return reapDeadState(profileName)
}

// GetStatus checks and reports the status of a headless server for the given profile.
func (m *Manager) GetStatus(profileName string) (*InstanceInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := config.ValidateProfileName(profileName); err != nil {
		return nil, err
	}

	reaped, err := reapDeadState(profileName)
	if err != nil {
		return nil, err
	}
	if reaped != nil {
		return &InstanceInfo{
			Profile: profileName,
			Status:  StateStopped,
		}, nil
	}

	info, err := readStateFile(profileName)
	if err != nil {
		if os.IsNotExist(err) {
			return &InstanceInfo{
				Profile: profileName,
				Status:  StateStopped,
			}, nil
		}
		return nil, err
	}

	// Probe health over HTTPS
	if err := probeHealthFn(info.Port, info.CSRFToken); err != nil {
		info.Status = StateUnhealthy
		info.HealthError = err.Error()
	} else {
		info.Status = StateRunning
		info.HealthError = ""
	}

	return info, nil
}

// Start launches a new background headless server for the specified profile.
func (m *Manager) Start(profileName string, opts StartOptions) (*InstanceInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := config.ValidateProfileName(profileName); err != nil {
		return nil, err
	}

	profileDir := config.GetProfileDir(profileName)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profileName)
	}

	// Check existing instance
	existing, _ := readStateFile(profileName)
	if existing != nil && isProcessAliveHook(existing.PID) {
		if !opts.ForceRestart {
			// Already running, probe health
			if err := probeHealthFn(existing.Port, existing.CSRFToken); err == nil {
				existing.Status = StateRunning
				return existing, nil
			}
		}
		// Force restart or unhealthy: terminate previous
		_ = terminateProcessHook(existing.PID)
		removeStateFile(profileName)
	}

	lsBin, err := findLanguageServerFn()
	if err != nil || lsBin == "" {
		return nil, fmt.Errorf("language server executable not found")
	}

	pGemini := filepath.Join(profileDir, ".gemini")
	csrf := quota.GenerateUUID()
	targetPort := opts.Port
	if targetPort < 0 {
		targetPort = 0
	}

	args := []string{
		"--standalone",
		"--headless=true",
		"--gemini_dir", pGemini,
		"--app_data_dir", "antigravity",
		"--csrf_token", csrf,
		"--https_server_port", strconv.Itoa(targetPort),
		"--http_server_port", "0",
		"--api_server_url", "https://generativelanguage.googleapis.com",
		"--cloud_code_endpoint", "https://daily-cloudcode-pa.googleapis.com",
	}

	cmd := exec.Command(lsBin, args...)

	// Invariant #8: Strictly set HOME / USERPROFILE to profileDir and enrich PATH
	env, err := buildHeadlessEnv(profileDir)
	if err != nil {
		return nil, fmt.Errorf("failed to build environment: %w", err)
	}
	cmd.Env = env
	cmd.Dir = profileDir

	logFilePath := getHeadlessLogPath(profileName)
	if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
		return nil, err
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	if startProcessHook != nil {
		portChan := make(chan int, 1)
		errChan := make(chan error, 1)
		inst, err := startProcessHook(cmd, logFilePath, portChan, errChan)
		if err != nil {
			return nil, err
		}
		inst.Profile = profileName
		inst.ExecutablePath = lsBin
		inst.LogFile = logFilePath
		_ = writeStateFile(profileName, inst)
		return inst, nil
	}

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %q: %w", logFilePath, err)
	}

	// Detached process group
	setDetachedProcessGroup(cmd)

	cmd.Stdout = logFile

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = logFile.Close()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, fmt.Errorf("failed to start headless server: %w", err)
	}

	portChan := make(chan int, 1)
	errChan := make(chan error, 1)

	// Stream stderr to logFile and parse HTTPS port
	go func() {
		defer logFile.Close()
		multiWriter := io.MultiWriter(logFile)
		scanner := bufio.NewScanner(stderrPipe)
		captured := false
		for scanner.Scan() {
			line := scanner.Text()
			_, _ = fmt.Fprintln(multiWriter, line)
			if !captured && strings.Contains(line, "for HTTPS") {
				m := httpsPortRegex.FindStringSubmatch(line)
				if len(m) >= 2 {
					if p, err := strconv.Atoi(m[1]); err == nil {
						portChan <- p
						captured = true
					}
				}
			}
		}
		if err := scanner.Err(); err != nil && !captured {
			errChan <- err
		}
	}()

	select {
	case detectedPort := <-portChan:
		// Verify responsiveness
		healthDeadline := time.Now().Add(5 * time.Second)
		var lastHealthErr error
		for time.Now().Before(healthDeadline) {
			if err := probeHealthFn(detectedPort, csrf); err == nil {
				info := &InstanceInfo{
					Profile:        profileName,
					PID:            cmd.Process.Pid,
					Port:           detectedPort,
					CSRFToken:      csrf,
					Status:         StateRunning,
					StartedAt:      time.Now(),
					LogFile:        logFilePath,
					ExecutablePath: lsBin,
				}
				if err := writeStateFile(profileName, info); err != nil {
					return nil, err
				}
				return info, nil
			} else {
				lastHealthErr = err
			}
			time.Sleep(150 * time.Millisecond)
		}
		_ = terminateProcessHook(cmd.Process.Pid)
		return nil, fmt.Errorf("headless server started on port %d but failed health check: %v", detectedPort, lastHealthErr)

	case err := <-errChan:
		_ = terminateProcessHook(cmd.Process.Pid)
		return nil, fmt.Errorf("headless server failed: %w", err)

	case <-time.After(timeout):
		_ = terminateProcessHook(cmd.Process.Pid)
		return nil, errors.New("timeout waiting for headless server port binding")
	}
}

// Stop terminates the headless server for the profile.
func (m *Manager) Stop(profileName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := config.ValidateProfileName(profileName); err != nil {
		return err
	}

	info, err := readStateFile(profileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if isProcessAliveHook(info.PID) {
		if err := terminateProcessHook(info.PID); err != nil {
			return fmt.Errorf("failed to terminate process %d: %w", info.PID, err)
		}
	}

	removeStateFile(profileName)
	return nil
}

// Restart stops and restarts the headless server for the profile.
func (m *Manager) Restart(profileName string, opts StartOptions) (*InstanceInfo, error) {
	_ = m.Stop(profileName)
	return m.Start(profileName, opts)
}

// List inspects all profiles and returns their headless server instances.
func (m *Manager) List() ([]*InstanceInfo, error) {
	profiles, err := profile.ListProfiles()
	if err != nil {
		return nil, err
	}

	var results []*InstanceInfo
	for _, name := range profiles {
		st, err := m.GetStatus(name)
		if err != nil {
			continue
		}
		if st.Status != StateStopped {
			results = append(results, st)
		}
	}
	return results, nil
}

// GetLogs reads the latest log lines from the profile's headless log file.
func (m *Manager) GetLogs(profileName string, tailLines int) (string, error) {
	logPath := getHeadlessLogPath(profileName)
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return "", fmt.Errorf("log file not found for profile %q", profileName)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", err
	}

	if tailLines <= 0 {
		return string(data), nil
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
	}
	return strings.Join(lines, "\n"), nil
}

func buildHeadlessEnv(profileDir string) ([]string, error) {
	userHome, _ := os.UserHomeDir()
	realHome := os.Getenv("REAL_HOME")
	if realHome == "" {
		realHome = userHome
	}

	envMap := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// 1. Identity & Filesystem Isolation
	enrichedPATH := buildEnrichedPath(realHome)
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
	return finalEnv, nil
}

func buildEnrichedPath(hostHome string) string {
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
