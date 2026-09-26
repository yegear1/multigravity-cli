package headless

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

// Exec fans the same prompt out across isolated profiles by calling RunAgentPrompt.
// Each selected profile keeps its own HOME and credential vault. There is no shared
// keyring swap and no second agent engine beside dispatch or the gateway.
func (m *Manager) Exec(opts ExecOptions) (*ExecReport, error) {
	names, err := resolveExecProfiles(opts)
	if err != nil {
		return nil, err
	}

	workers := opts.Workers
	if workers <= 0 || workers > len(names) {
		workers = len(names)
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	results := make([]AgentRunResult, len(names))
	jobs := make(chan int)
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				name := names[idx]
				res, runErr := m.RunAgentPrompt(AgentRunOptions{
					Profile:                    name,
					Prompt:                     opts.Prompt,
					Timeout:                    timeout,
					DangerouslySkipPermissions: opts.DangerouslySkipPermissions,
				})
				if runErr != nil {
					results[idx] = AgentRunResult{
						Profile:  name,
						Prompt:   opts.Prompt,
						ExitCode: 1,
						Error:    runErr.Error(),
					}
					continue
				}
				if res == nil {
					results[idx] = AgentRunResult{
						Profile:  name,
						Prompt:   opts.Prompt,
						ExitCode: 1,
						Error:    "headless run returned no result",
					}
					continue
				}
				results[idx] = *res
			}
		}()
	}

	for idx := range names {
		jobs <- idx
	}
	close(jobs)
	wg.Wait()

	report := &ExecReport{
		Prompt:          opts.Prompt,
		Workers:         workers,
		Results:         results,
		DurationSeconds: time.Since(start).Seconds(),
	}
	for i := range report.Results {
		report.TotalTokens += report.Results[i].TotalTokens
		if execResultFailed(report.Results[i]) {
			report.Failed++
		} else {
			report.Succeeded++
		}
	}
	return report, nil
}

func execResultFailed(r AgentRunResult) bool {
	return r.ExitCode != 0 || r.Error != ""
}

func resolveExecProfiles(opts ExecOptions) ([]string, error) {
	if strings.TrimSpace(opts.Prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	var names []string
	switch {
	case opts.All:
		listed, err := profile.ListProfiles()
		if err != nil {
			return nil, err
		}
		names = listed
	case len(opts.Profiles) > 0:
		names = opts.Profiles
	case opts.Profile != "":
		names = []string{opts.Profile}
	default:
		return nil, fmt.Errorf("usage: multigravity exec [profile|--all] \"<prompt>\"")
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("no profiles to execute")
	}

	seen := make(map[string]struct{}, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		if err := config.ValidateProfileName(name); err != nil {
			return nil, err
		}
		if !profile.ProfileExists(name) {
			return nil, fmt.Errorf("profile %q does not exist", name)
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out, nil
}
