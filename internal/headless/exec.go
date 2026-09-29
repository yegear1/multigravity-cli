package headless

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

// Exec fans the same prompt out across isolated profiles by calling RunAgentPrompt.
// Each selected profile keeps its own HOME and credential vault. There is no shared
// keyring swap and no second agent engine beside dispatch or the gateway.
func (m *Manager) Exec(opts ExecOptions) (*ExecReport, error) {
	start := time.Now()
	names, skipped, err := resolveExecProfiles(opts)
	if err != nil {
		return nil, err
	}

	workers := opts.Workers
	if workers <= 0 || workers > len(names) {
		workers = len(names)
	}
	if workers == 0 {
		workers = 1
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	if len(names) == 0 {
		report := &ExecReport{
			Prompt:          opts.Prompt,
			Workers:         workers,
			Results:         []AgentRunResult{},
			Skipped:         skipped,
			DurationSeconds: time.Since(start).Seconds(),
		}
		return report, nil
	}

	type runItem struct {
		idx    int
		result AgentRunResult
	}

	var (
		nextIdx   int
		idxMu     sync.Mutex
		resultsMu sync.Mutex
		finished  []runItem
		failed    atomic.Bool
		wg        sync.WaitGroup
	)

	getNext := func() (int, string, bool) {
		idxMu.Lock()
		defer idxMu.Unlock()
		if failed.Load() || nextIdx >= len(names) {
			return -1, "", false
		}
		idx := nextIdx
		name := names[idx]
		nextIdx++
		return idx, name, true
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				idx, name, ok := getNext()
				if !ok {
					return
				}
				res, runErr := m.RunAgentPrompt(AgentRunOptions{
					Profile:                    name,
					Prompt:                     opts.Prompt,
					Model:                      opts.Model,
					Timeout:                    timeout,
					DangerouslySkipPermissions: opts.DangerouslySkipPermissions,
				})

				var r AgentRunResult
				if runErr != nil {
					r = AgentRunResult{
						Profile:  name,
						Prompt:   opts.Prompt,
						ExitCode: 1,
						Error:    runErr.Error(),
					}
				} else if res == nil {
					r = AgentRunResult{
						Profile:  name,
						Prompt:   opts.Prompt,
						ExitCode: 1,
						Error:    "headless run returned no result",
					}
				} else {
					r = *res
				}

				if execResultFailed(r) {
					failed.Store(true)
				}

				resultsMu.Lock()
				finished = append(finished, runItem{idx: idx, result: r})
				resultsMu.Unlock()
			}
		}()
	}

	wg.Wait()

	sort.Slice(finished, func(i, j int) bool {
		return finished[i].idx < finished[j].idx
	})

	results := make([]AgentRunResult, len(finished))
	for i := range finished {
		results[i] = finished[i].result
	}

	report := &ExecReport{
		Prompt:          opts.Prompt,
		Workers:         workers,
		Results:         results,
		Skipped:         skipped,
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

func resolveExecProfiles(opts ExecOptions) ([]string, []string, error) {
	if strings.TrimSpace(opts.Prompt) == "" {
		return nil, nil, fmt.Errorf("prompt is required")
	}

	var names []string
	switch {
	case opts.All:
		listed, err := profile.ListProfiles()
		if err != nil {
			return nil, nil, err
		}
		names = listed
	case len(opts.Profiles) > 0:
		names = opts.Profiles
	case opts.Profile != "":
		names = []string{opts.Profile}
	default:
		return nil, nil, fmt.Errorf("usage: multigravity exec [profile|--all] \"<prompt>\"")
	}

	if len(names) == 0 {
		return nil, nil, fmt.Errorf("no profiles to execute")
	}

	seen := make(map[string]struct{}, len(names))
	out := make([]string, 0, len(names))
	var skipped []string
	for _, name := range names {
		if err := config.ValidateProfileName(name); err != nil {
			return nil, nil, err
		}
		if !profile.ProfileExists(name) {
			return nil, nil, fmt.Errorf("profile %q does not exist", name)
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}

		if opts.All {
			info, err := profile.GetProfile(name)
			if err == nil && info != nil && info.Type == "auth-only" && !info.IsRunning {
				skipped = append(skipped, name)
				continue
			}
		}

		out = append(out, name)
	}

	if len(out) == 0 && len(skipped) == 0 {
		return nil, nil, fmt.Errorf("no profiles to execute")
	}

	return out, skipped, nil
}
