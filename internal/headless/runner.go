package headless

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/app"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

var (
	// Test hook for hermetic testing of agent prompt runner
	runAgyCommandFn = defaultRunAgyCommand
	findAgyFn       = app.FindAgy
)

// SetRunnerTestHooks allows injecting custom agy execution mocks.
func SetRunnerTestHooks(
	findAgy func() (string, error),
	runAgy func(ctx context.Context, bin string, args []string, env []string, dir string) ([]byte, int, error),
) func() {
	origFind := findAgyFn
	origRun := runAgyCommandFn

	if findAgy != nil {
		findAgyFn = findAgy
	}
	if runAgy != nil {
		runAgyCommandFn = runAgy
	}

	return func() {
		findAgyFn = origFind
		runAgyCommandFn = origRun
	}
}

func defaultRunAgyCommand(ctx context.Context, bin string, args []string, env []string, dir string) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return out, exitCode, err
}

type agyJSONOutput struct {
	Response string `json:"response"`
	Usage    struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	DurationSeconds float64 `json:"duration_seconds"`
}

// RunAgentPrompt executes a prompt in a headless agent environment with identity isolation.
func (m *Manager) RunAgentPrompt(opts AgentRunOptions) (*AgentRunResult, error) {
	if err := config.ValidateProfileName(opts.Profile); err != nil {
		return nil, err
	}

	profileDir := config.GetProfileDir(opts.Profile)
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()

	// 1. Try agy CLI first
	agyBin, err := findAgyFn()
	if err == nil && agyBin != "" {
		args := []string{
			"-p", opts.Prompt,
			"--print-timeout", fmt.Sprintf("%ds", int(timeout.Seconds())),
			"--output-format", "json",
		}
		if opts.DangerouslySkipPermissions {
			args = append(args, "--dangerously-skip-permissions")
		}

		env, err := buildHeadlessEnv(profileDir)
		if err != nil {
			return nil, err
		}

		out, exitCode, cmdErr := runAgyCommandFn(ctx, agyBin, args, env, profileDir)
		duration := time.Since(start).Seconds()

		result := &AgentRunResult{
			Profile:         opts.Profile,
			Prompt:          opts.Prompt,
			DurationSeconds: duration,
			ExitCode:        exitCode,
		}

		// Try parsing JSON output
		var parsed agyJSONOutput
		if jsonErr := json.Unmarshal(out, &parsed); jsonErr == nil && parsed.Response != "" {
			result.Response = parsed.Response
			result.TotalTokens = parsed.Usage.TotalTokens
			if parsed.DurationSeconds > 0 {
				result.DurationSeconds = parsed.DurationSeconds
			}
		} else {
			result.Response = strings.TrimSpace(string(out))
		}

		if cmdErr != nil {
			result.Error = cmdErr.Error()
		}

		return result, nil
	}

	// 2. Fallback: Language Server Cascade RPC
	inst, err := m.GetStatus(opts.Profile)
	if err != nil || inst.Status != StateRunning {
		// Attempt to start headless server
		inst, err = m.Start(opts.Profile, StartOptions{
			Profile: opts.Profile,
			Timeout: 10 * time.Second,
		})
		if err != nil {
			return nil, fmt.Errorf("neither 'agy' CLI nor language server is available: %w", err)
		}
	}

	client := quota.NewClient(timeout)
	cascadeID, err := client.StartCascade(inst.Port, inst.CSRFToken)
	if err != nil {
		return nil, fmt.Errorf("failed to start cascade session: %w", err)
	}

	err = client.SendUserCascadeMessage(inst.Port, inst.CSRFToken, cascadeID, opts.Prompt, "MODEL_PLACEHOLDER_M73")
	duration := time.Since(start).Seconds()

	result := &AgentRunResult{
		Profile:         opts.Profile,
		Prompt:          opts.Prompt,
		Response:        "Message dispatched successfully to cascade session " + cascadeID,
		DurationSeconds: duration,
		ExitCode:        0,
	}

	if err != nil {
		result.ExitCode = 1
		result.Error = err.Error()
	}

	return result, nil
}
