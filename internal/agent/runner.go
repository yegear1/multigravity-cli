package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/app"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/headless"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

var (
	runnerMu        sync.Mutex
	findAgyFn       = app.FindAgy
	runAgyCmdFn     = defaultRunAgyCmd
	ensureProfileFn = defaultEnsureProfileReady
	runCascadeFn    = defaultRunCascadeRPC
)

// SetRunnerTestHooks allows injecting mocks for hermetic testing of subagent execution.
func SetRunnerTestHooks(
	findAgy func() (string, error),
	runAgy func(ctx context.Context, bin string, args []string, env []string, dir string) ([]byte, int, error),
	ensureProfile func(profileName string) error,
	runCascade func(ctx context.Context, profileName, prompt, model string, timeout time.Duration) (string, int, error),
) func() {
	runnerMu.Lock()
	defer runnerMu.Unlock()

	origFind := findAgyFn
	origRun := runAgyCmdFn
	origEnsure := ensureProfileFn
	origCascade := runCascadeFn

	if findAgy != nil {
		findAgyFn = findAgy
	}
	if runAgy != nil {
		runAgyCmdFn = runAgy
	}
	if ensureProfile != nil {
		ensureProfileFn = ensureProfile
	}
	if runCascade != nil {
		runCascadeFn = runCascade
	}

	return func() {
		runnerMu.Lock()
		defer runnerMu.Unlock()
		findAgyFn = origFind
		runAgyCmdFn = origRun
		ensureProfileFn = origEnsure
		runCascadeFn = origCascade
	}
}

// NormalizeAgyArgs cleans and normalizes argument lists intended for agy CLI.
// It removes unsupported flags like --headless or --headless=true (which cause exit code 2),
// and merges conflicting --model and --effort flags if present.
func NormalizeAgyArgs(args []string) []string {
	// First pass: strip --headless variants
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--headless" || strings.HasPrefix(a, "--headless=") {
			continue
		}
		filtered = append(filtered, a)
	}

	modelIdx := -1
	isModelCombined := false
	modelVal := ""

	effortIdx := -1
	isEffortCombined := false
	effortVal := ""

	for i := 0; i < len(filtered); i++ {
		a := filtered[i]
		if a == "--model" && i+1 < len(filtered) {
			modelIdx = i
			isModelCombined = false
			modelVal = filtered[i+1]
			i++
		} else if strings.HasPrefix(a, "--model=") {
			modelIdx = i
			isModelCombined = true
			modelVal = strings.TrimPrefix(a, "--model=")
		} else if a == "--effort" && i+1 < len(filtered) {
			effortIdx = i
			isEffortCombined = false
			effortVal = filtered[i+1]
			i++
		} else if strings.HasPrefix(a, "--effort=") {
			effortIdx = i
			isEffortCombined = true
			effortVal = strings.TrimPrefix(a, "--effort=")
		}
	}

	if modelIdx == -1 || effortIdx == -1 {
		return filtered
	}

	var suffix string
	switch {
	case strings.HasSuffix(modelVal, "-low"):
		suffix = "-low"
	case strings.HasSuffix(modelVal, "-medium"):
		suffix = "-medium"
	case strings.HasSuffix(modelVal, "-high"):
		suffix = "-high"
	case strings.HasSuffix(modelVal, "-tiered"):
		suffix = "-tiered"
	default:
		return filtered
	}

	effortLower := strings.ToLower(effortVal)
	if effortLower != "low" && effortLower != "medium" && effortLower != "high" {
		return filtered
	}

	newModelVal := strings.TrimSuffix(modelVal, suffix) + "-" + effortLower

	res := make([]string, 0, len(filtered))
	for i := 0; i < len(filtered); i++ {
		if i == effortIdx {
			if !isEffortCombined {
				i++ // skip value
			}
			continue
		}
		if i == modelIdx {
			if isModelCombined {
				res = append(res, "--model="+newModelVal)
			} else {
				res = append(res, "--model", newModelVal)
				i++ // skip old value
			}
			continue
		}
		res = append(res, filtered[i])
	}

	return res
}

// EnsureProfileReady verifies that the specified profile is active.
// If inactive (neither desktop IDE nor headless language server running),
// it automatically spawns a background headless server to awaken the profile environment.
func EnsureProfileReady(profileName string) error {
	runnerMu.Lock()
	fn := ensureProfileFn
	runnerMu.Unlock()

	if fn != nil {
		return fn(profileName)
	}
	return defaultEnsureProfileReady(profileName)
}

func defaultEnsureProfileReady(profileName string) error {
	profileName = strings.TrimSpace(profileName)
	if profileName == "" || profileName == "host" {
		return nil
	}
	if err := config.ValidateProfileName(profileName); err != nil {
		return err
	}
	if !profile.ProfileExists(profileName) {
		return fmt.Errorf("profile %q does not exist", profileName)
	}

	// Fast-path: Check if already active
	if headless.IsProfileActive(profileName) {
		return nil
	}

	// Awaken inactive profile with on-demand headless server
	_, err := headless.EnsureRunning(profileName, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to awaken inactive profile %q: %w", profileName, err)
	}
	return nil
}

// HeadlessSubagentOptions defines parameters for running a subagent prompt on a profile.
type HeadlessSubagentOptions struct {
	Profile                    string            `json:"profile"`
	Prompt                     string            `json:"prompt"`
	Model                      string            `json:"model,omitempty"`
	Effort                     string            `json:"effort,omitempty"`
	Timeout                    time.Duration     `json:"timeout,omitempty"`
	Cwd                        string            `json:"cwd,omitempty"`
	RepoPath                   string            `json:"repo_path,omitempty"`
	WorktreeID                 string            `json:"worktree_id,omitempty"`
	DangerouslySkipPermissions bool              `json:"dangerously_skip_permissions,omitempty"`
	DisableSlashCommands       bool              `json:"disable_slash_commands,omitempty"`
	TraceID                    string            `json:"trace_id,omitempty"`
	Env                        map[string]string `json:"env,omitempty"`
}

// HeadlessSubagentResult captures the execution output and telemetry of a subagent run.
type HeadlessSubagentResult struct {
	Profile         string  `json:"profile"`
	Prompt          string  `json:"prompt"`
	Response        string  `json:"response"`
	TotalTokens     int     `json:"total_tokens"`
	DurationSeconds float64 `json:"duration_seconds"`
	ExitCode        int     `json:"exit_code"`
	Error           string  `json:"error,omitempty"`
	UsedFallback    bool    `json:"used_fallback,omitempty"`
}

func defaultRunAgyCmd(ctx context.Context, bin string, args []string, env []string, dir string) ([]byte, int, error) {
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

func defaultRunCascadeRPC(ctx context.Context, profileName, prompt, model string, timeout time.Duration) (string, int, error) {
	mgr := headless.GetDefaultManager()
	inst, err := mgr.EnsureRunning(profileName, 10*time.Second)
	if err != nil {
		return "", 1, fmt.Errorf("failed to ensure headless server for cascade: %w", err)
	}

	client := quota.NewClient(timeout)
	cascadeID, err := client.StartCascade(inst.Port, inst.CSRFToken)
	if err != nil {
		return "", 1, fmt.Errorf("failed to start cascade session: %w", err)
	}

	cascadeModel := "MODEL_PLACEHOLDER_M73"
	if strings.TrimSpace(model) != "" {
		cascadeModel = strings.TrimSpace(model)
	}
	err = client.SendUserCascadeMessage(inst.Port, inst.CSRFToken, cascadeID, prompt, cascadeModel)
	if err != nil {
		return "", 1, err
	}
	return "Message dispatched successfully to cascade session " + cascadeID, 0, nil
}

// RunHeadlessSubagent executes an autonomous subagent task against the target profile.
// If the profile is inactive, it automatically awakens it via EnsureProfileReady.
// It prioritizes the agy CLI (with normalized arguments), falling back cleanly to the
// headless Language Server Cascade RPC if agy is not installed.
func RunHeadlessSubagent(ctx context.Context, opts HeadlessSubagentOptions) (*HeadlessSubagentResult, error) {
	if err := config.ValidateProfileName(opts.Profile); err != nil {
		return nil, err
	}

	// 1. Ensure target profile environment is awakened and ready
	if err := EnsureProfileReady(opts.Profile); err != nil {
		return nil, err
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	subCtx := ctx
	var cancel context.CancelFunc
	if subCtx == nil {
		subCtx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
	}

	start := time.Now()

	profileDir := config.GetProfileDir(opts.Profile)
	execDir := opts.Cwd
	if execDir == "" {
		execDir = opts.RepoPath
	}
	if execDir == "" {
		execDir = profileDir
	}

	runnerMu.Lock()
	findAgy := findAgyFn
	runAgy := runAgyCmdFn
	runCascade := runCascadeFn
	runnerMu.Unlock()

	// 2. Try agy CLI first
	agyBin, err := findAgy()
	if err == nil && agyBin != "" {
		args := []string{
			"-p", opts.Prompt,
			"--print-timeout", fmt.Sprintf("%ds", int(timeout.Seconds())),
			"--output-format", "json",
		}
		if opts.DangerouslySkipPermissions {
			args = append(args, "--dangerously-skip-permissions")
		}
		if opts.DisableSlashCommands {
			args = append(args, "--disable-slash-commands")
		}
		if model := strings.TrimSpace(opts.Model); model != "" {
			args = append(args, "--model", model)
		}
		if effort := strings.TrimSpace(opts.Effort); effort != "" {
			args = append(args, "--effort", effort)
		}

		args = NormalizeAgyArgs(args)

		sessOpts := CreateSessionOptions{
			Profile:    opts.Profile,
			AgentType:  "agy",
			Command:    agyBin,
			Args:       args,
			Cwd:        execDir,
			RepoPath:   opts.RepoPath,
			WorktreeID: opts.WorktreeID,
			Env:        opts.Env,
			TraceID:    opts.TraceID,
		}
		env, err := BuildAgentEnv(sessOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to build subagent environment: %w", err)
		}

		out, exitCode, cmdErr := runAgy(subCtx, agyBin, args, env, execDir)
		duration := time.Since(start).Seconds()

		res := &HeadlessSubagentResult{
			Profile:         opts.Profile,
			Prompt:          opts.Prompt,
			DurationSeconds: duration,
			ExitCode:        exitCode,
		}

		type agyJSON struct {
			Response string `json:"response"`
			Usage    struct {
				TotalTokens int `json:"total_tokens"`
			} `json:"usage"`
			DurationSeconds float64 `json:"duration_seconds"`
		}
		var parsed agyJSON
		if jErr := json.Unmarshal(out, &parsed); jErr == nil && parsed.Response != "" {
			res.Response = parsed.Response
			res.TotalTokens = parsed.Usage.TotalTokens
			if parsed.DurationSeconds > 0 {
				res.DurationSeconds = parsed.DurationSeconds
			}
		} else {
			res.Response = strings.TrimSpace(string(out))
		}

		if cmdErr != nil {
			if subCtx.Err() == context.DeadlineExceeded {
				res.Error = fmt.Sprintf("execution timed out after %s (context deadline exceeded)", timeout)
			} else {
				res.Error = cmdErr.Error()
			}
		}

		noteSubagentTokens(res)
		return res, nil
	}

	// 3. Fallback: Language Server Cascade RPC
	cascadeResp, exitCode, cErr := runCascade(subCtx, opts.Profile, opts.Prompt, opts.Model, timeout)
	duration := time.Since(start).Seconds()

	res := &HeadlessSubagentResult{
		Profile:         opts.Profile,
		Prompt:          opts.Prompt,
		Response:        cascadeResp,
		DurationSeconds: duration,
		ExitCode:        exitCode,
		UsedFallback:    true,
	}

	if cErr != nil {
		res.ExitCode = 1
		res.Error = cErr.Error()
	}

	noteSubagentTokens(res)
	return res, nil
}

func noteSubagentTokens(result *HeadlessSubagentResult) {
	if result == nil || result.ExitCode != 0 || result.TotalTokens <= 0 {
		return
	}
	quota.CalibrateTokens("", result.Prompt, 0, result.Response, result.TotalTokens)
	_ = quota.RecordTokens(result.Profile, quota.SourceHeadless, "", 0, 0, result.TotalTokens, false)
}
