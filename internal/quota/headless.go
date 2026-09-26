package quota

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
	"time"

	"github.com/ye-dev/multigravity-cli/internal/app"
	"github.com/ye-dev/multigravity-cli/internal/config"
)

type HeadlessInstance struct {
	Cmd       *exec.Cmd
	Port      int
	CSRF      string
	PID       int
	IsManaged bool
}

func (h *HeadlessInstance) Close() {
	if h.IsManaged {
		return
	}
	if h.Cmd != nil && h.Cmd.Process != nil {
		_ = h.Cmd.Process.Kill()
		_ = h.Cmd.Wait()
	}
}

var httpsPortRegex = regexp.MustCompile(`at (\d+) for HTTPS`)

func StartHeadlessServer(profileName string) (*HeadlessInstance, error) {
	// Fast-path: Check if a managed background headless instance is already running
	if profileName != "" {
		statePath := filepath.Join(config.GetProfileDir(profileName), ".multigravity", "headless.json")
		if data, err := os.ReadFile(statePath); err == nil {
			var state struct {
				PID       int    `json:"pid"`
				Port      int    `json:"port"`
				CSRFToken string `json:"csrf_token"`
			}
			if json.Unmarshal(data, &state) == nil && state.Port > 0 && state.CSRFToken != "" {
				client := NewClient(800 * time.Millisecond)
				if _, err := client.RetrieveUserQuotaSummary(state.Port, state.CSRFToken); err == nil {
					return &HeadlessInstance{
						Port:      state.Port,
						CSRF:      state.CSRFToken,
						PID:       state.PID,
						IsManaged: true,
					}, nil
				}
			}
		}
	}

	lsBin, err := app.FindLanguageServer()
	if err != nil || lsBin == "" {
		return nil, fmt.Errorf("language server executable not found")
	}

	pDir := ""
	pGemini := ""
	if profileName != "" {
		pDir = config.GetProfileDir(profileName)
		pGemini = filepath.Join(pDir, ".gemini")
	} else {
		userHome, _ := os.UserHomeDir()
		pGemini = filepath.Join(userHome, ".gemini")
	}

	csrf := GenerateUUID()
	args := []string{
		"--standalone",
		"--headless=true",
		"--gemini_dir", pGemini,
		"--app_data_dir", "antigravity",
		"--csrf_token", csrf,
		"--https_server_port", "0",
		"--http_server_port", "0",
		"--api_server_url", "https://generativelanguage.googleapis.com",
		"--cloud_code_endpoint", "https://daily-cloudcode-pa.googleapis.com",
	}

	cmd := exec.Command(lsBin, args...)

	// Invariant: HOME / USERPROFILE must be set to profile dir so credentials are read from the profile
	env := os.Environ()
	if pDir != "" {
		if runtime.GOOS == "windows" {
			env = append(env, "USERPROFILE="+pDir)
		} else {
			env = append(env, "HOME="+pDir)
		}
	}
	cmd.Env = env
	cmd.Stdout = io.Discard

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start headless language server: %w", err)
	}

	portChan := make(chan int, 1)
	errChan := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			if stringsContainsHTTPS(line) {
				m := httpsPortRegex.FindStringSubmatch(line)
				if len(m) >= 2 {
					if p, err := strconv.Atoi(m[1]); err == nil {
						portChan <- p
						return
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			errChan <- err
		}
	}()

	client := NewClient(1500 * time.Millisecond)
	select {
	case port := <-portChan:
		// Verify port works
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := client.RetrieveUserQuotaSummary(port, csrf); err == nil {
				return &HeadlessInstance{
					Cmd:  cmd,
					Port: port,
					CSRF: csrf,
					PID:  cmd.Process.Pid,
				}, nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("language server started on port %d but failed health check", port)

	case err := <-errChan:
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("error reading language server stderr: %w", err)

	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		return nil, errors.New("timed out waiting for language server to initialize port")
	}
}

func stringsContainsHTTPS(s string) bool {
	return len(s) > 0 && (httpsPortRegex.MatchString(s))
}
