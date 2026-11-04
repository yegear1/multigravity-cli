package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/agent"
	"github.com/ye-dev/multigravity-cli/internal/alert"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/dispatch"
	"github.com/ye-dev/multigravity-cli/internal/doctor"
	"github.com/ye-dev/multigravity-cli/internal/prime"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
	"github.com/ye-dev/multigravity-cli/internal/workspace"
	"github.com/ye-dev/multigravity-cli/internal/worktree"
)

func toJSONText(v any) (*CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to encode result as JSON: %w", err)
	}
	return NewTextResult(string(b)), nil
}

func getString(args map[string]any, key string) string {
	if val, ok := args[key]; ok && val != nil {
		if s, ok := val.(string); ok {
			return strings.TrimSpace(s)
		}
		return fmt.Sprintf("%v", val)
	}
	return ""
}

func getBool(args map[string]any, key string) bool {
	if val, ok := args[key]; ok && val != nil {
		if b, ok := val.(bool); ok {
			return b
		}
		if s, ok := val.(string); ok {
			b, _ := strconv.ParseBool(s)
			return b
		}
	}
	return false
}

func getInt(args map[string]any, key string, defaultVal int) int {
	if val, ok := args[key]; ok && val != nil {
		switch v := val.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case int64:
			return int(v)
		case string:
			if parsed, err := strconv.Atoi(v); err == nil {
				return parsed
			}
		}
	}
	return defaultVal
}

func getStringMap(args map[string]any, key string) map[string]string {
	res := make(map[string]string)
	if val, ok := args[key]; ok && val != nil {
		if m, ok := val.(map[string]any); ok {
			for k, v := range m {
				res[k] = fmt.Sprintf("%v", v)
			}
		}
	}
	return res
}

func getStringSlice(args map[string]any, key string) []string {
	if val, ok := args[key]; ok && val != nil {
		if slice, ok := val.([]string); ok {
			return slice
		}
		if slice, ok := val.([]any); ok {
			res := make([]string, 0, len(slice))
			for _, item := range slice {
				if item != nil {
					res = append(res, fmt.Sprintf("%v", item))
				}
			}
			return res
		}
	}
	return nil
}

// RegisterDefaultTools registers the standard Multigravity toolset on the server
func RegisterDefaultTools(s *Server) {
	// ==========================================
	// 1. Dispatch Tools
	// ==========================================

	type dispatchTaskParams struct {
		Command           string            `json:"command" description:"Command or agent instruction to execute (e.g., 'claude -p \"fix bug\"' or shell script)." required:"true"`
		Profile           string            `json:"profile" description:"Antigravity profile name for identity and credentials isolation (e.g. 'yegear', 'yegear2'), or 'auto' to dynamically route to the profile with the healthiest quota." required:"true"`
		AgentType         string            `json:"agent_type" description:"Agent type for command resolution (e.g., 'claude', 'aider', 'opencode', 'agy', 'codex')."`
		Args              []string          `json:"args" description:"Command arguments array. If provided, overrides automatic argument resolution."`
		AutoSpawnHeadless bool              `json:"auto_spawn_headless" description:"Automatically awaken inactive profiles with on-demand headless server if needed."`
		Background        bool              `json:"background" description:"Whether to dispatch detached in background and return immediately with the task ID (default false)."`
		Branch            string            `json:"branch" description:"Branch name to create or use inside an isolated Git worktree."`
		Env               map[string]string `json:"env" description:"Custom environment variables key-value map to pass into the task session."`
		Governance        bool              `json:"governance" description:"Inject project governance directives (AGENTS.md, TASK.md) into agent prompt and environment (defaults to false for MCP dispatch)."`
		Headless          bool              `json:"headless" description:"Execute task in headless agent mode."`
		Model             string            `json:"model" description:"Target model for quota-aware routing when profile is 'auto' (e.g. 'gemini-3.8-flash', 'claude-3-7-sonnet')."`
		Probe             bool              `json:"probe" description:"Whether to probe and awaken inactive secondary profiles when profile is 'auto' (default false)."`
		Prompt            string            `json:"prompt" description:"Prompt instruction for the agent command (e.g. passed as -p to agy or claude if args is not provided)."`
		ReadOnly          bool              `json:"read_only" description:"Enforce strict read-only execution guard preventing file modifications."`
		Repo              string            `json:"repo" description:"Path to target Git repository. Defaults to current directory or detected active workspace."`
		Retry             *int              `json:"retry" description:"Maximum number of retries if verification fails (default 0)."`
		Role              string            `json:"role" description:"Agent execution role persona: 'developer', 'researcher', 'verifier', 'reviewer', or 'auditor'."`
		Timeout           *int              `json:"timeout" description:"Maximum seconds to wait when wait is true (default 120)."`
		TraceID           string            `json:"trace_id" description:"Distributed trace identifier (or MAESTRI_TRACE_ID) for VictoriaLogs token delta correlation."`
		Verify            string            `json:"verify" description:"Verification command to execute inside worktree upon agent completion (e.g. 'go test ./...'). If non-zero exit code, marks task audit_failed."`
		Wait              bool              `json:"wait" description:"Wait synchronously for task execution to reach terminal status before returning result (default false)."`
		Weight            string            `json:"weight" description:"Task weight for quota routing thresholds when profile is 'auto': 'light', 'medium', or 'heavy' (default 'medium')."`
		Workspace         string            `json:"workspace" description:"Workspace isolation mode: 'worktree' (isolated Git worktree), 'mirror' (lightweight mirror), or 'inherit' (zero-I/O in-place)."`
		Worktree          string            `json:"worktree" description:"Custom worktree path or reuse identifier. If empty and branch is given, an ephemeral worktree is created."`
	}
	RegisterTypedTool(s, "dispatch_task", "Dispatch an agent command or script within an isolated Git worktree or specified repository using a dedicated Antigravity profile.", func(ctx context.Context, params dispatchTaskParams) (*CallToolResult, error) {
		cmdStr := params.Command
		if cmdStr == "" {
			return nil, fmt.Errorf("command is required")
		}

		prof := params.Profile
		if prof == "" {
			return nil, fmt.Errorf("profile is required")
		}

		if strings.EqualFold(prof, "auto") {
			weight := quota.NormalizeWeight(quota.RouteWeight(params.Weight))
			model := params.Model
			if model == "" {
				model = "gemini-3.8-flash"
			}
			if params.Probe {
				_, _ = quota.ProbeInactiveProfiles(nil)
			}
			resolvedProf := ""
			eligible, err := quota.SelectEligibleProfilesForModel(0, model, weight)
			if err == nil && len(eligible) > 0 {
				for _, p := range eligible {
					if profile.ProfileExists(p) && !quota.IsProfileOnCooldown(p) {
						resolvedProf = p
						break
					}
				}
			}
			if resolvedProf == "" {
				candidates, cErr := quota.PreflightProfilesForModel(0, model, weight, quota.DefaultSafetyQuotaThreshold)
				if cErr == nil && len(candidates) > 0 {
					for _, c := range candidates {
						if profile.ProfileExists(c.Profile) && !quota.IsProfileOnCooldown(c.Profile) {
							resolvedProf = c.Profile
							break
						}
					}
				}
			}
			if resolvedProf == "" {
				if profs, pErr := profile.ListProfiles(); pErr == nil && len(profs) > 0 {
					for _, p := range profs {
						if profile.ProfileExists(p) && !quota.IsProfileOnCooldown(p) {
							resolvedProf = p
							break
						}
					}
					if resolvedProf == "" {
						resolvedProf = profs[0]
					}
				}
			}
			if resolvedProf == "" {
				return nil, fmt.Errorf("auto-routing failed: no eligible profiles available")
			}
			prof = resolvedProf
		} else {
			if err := config.ValidateProfileName(prof); err != nil {
				return nil, err
			}
			if !profile.ProfileExists(prof) {
				return nil, fmt.Errorf("profile %q does not exist", prof)
			}
		}

		repo := params.Repo
		if repo == "" {
			var err error
			repo, err = os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("failed to determine repository path: %w", err)
			}
		}

		branch := params.Branch
		wtID := params.Worktree
		newWorktree := (wtID == "" && branch != "")

		opts := dispatch.DispatchOptions{
			Command:           cmdStr,
			Args:              params.Args,
			Prompt:            params.Prompt,
			Profile:           prof,
			RepoPath:          repo,
			Branch:            branch,
			WorktreeID:        wtID,
			NewWorktree:       newWorktree,
			AgentType:         params.AgentType,
			Detached:          params.Background,
			Env:               params.Env,
			Verify:            params.Verify,
			Retry:             intOr(params.Retry, 0),
			TraceID:           params.TraceID,
			NoGovernance:      !params.Governance,
			Role:              params.Role,
			ReadOnly:          params.ReadOnly,
			Workspace:         params.Workspace,
			Headless:          params.Headless,
			AutoSpawnHeadless: params.AutoSpawnHeadless,
		}

		mgr := dispatch.GetDefaultTaskManager()
		task, err := mgr.Dispatch(opts)
		if err != nil {
			return nil, fmt.Errorf("failed to dispatch task: %w", err)
		}

		if params.Wait {
			timeoutSec := intOr(params.Timeout, 120)
			waitCtx, waitCancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
			defer waitCancel()
			completedTask, waitErr := mgr.WaitForTask(waitCtx, repo, task.ID)
			if waitErr == nil && completedTask != nil {
				task = completedTask
			}
		}

		return toJSONText(task)
	})

	type dispatchSubagentParams struct {
		Prompt                     string            `json:"prompt" description:"Subagent task prompt or instruction to execute autonomously." required:"true"`
		AutoFailover               *bool             `json:"auto_failover" description:"Automatically fail over to secondary candidate profiles if the primary hits rate limits or quota exhaustion (default true)."`
		Cwd                        string            `json:"cwd" description:"Working directory for subagent execution."`
		DangerouslySkipPermissions *bool             `json:"dangerously_skip_permissions" description:"Bypass interactive confirmation prompts in headless mode (default true)."`
		DisableSlashCommands       *bool             `json:"disable_slash_commands" description:"Disable interactive slash commands in headless execution (default true)."`
		Effort                     string            `json:"effort" description:"Reasoning effort level for the model: 'low', 'medium', or 'high'."`
		Env                        map[string]string `json:"env" description:"Custom environment variables to pass to the subagent session."`
		MaxFailovers               *int              `json:"max_failovers" description:"Maximum number of secondary profile failovers to attempt (default 3)."`
		Model                      string            `json:"model" description:"Target model for execution and quota routing (e.g. 'gemini-3.8-flash', 'gemini-3.8-pro', 'claude-3-7-sonnet')."`
		Profile                    string            `json:"profile" description:"Antigravity profile name for execution, or 'auto' for dynamic quota-aware routing (default 'auto')."`
		Repo                       string            `json:"repo" description:"Repository path for subagent execution."`
		Timeout                    *int              `json:"timeout" description:"Execution timeout in seconds (default 120)."`
		TraceID                    string            `json:"trace_id" description:"Distributed trace identifier for log and token correlation."`
		Weight                     string            `json:"weight" description:"Task weight for quota threshold evaluation: 'light', 'medium', or 'heavy' (default 'medium')."`
	}
	RegisterTypedTool(s, "dispatch_subagent", "Delegate an autonomous subagent prompt directly to an Antigravity worker using headless CLI invocation (agy -p / cascade) with quota-aware multi-profile routing and automatic failover.", func(ctx context.Context, params dispatchSubagentParams) (*CallToolResult, error) {
		prompt := params.Prompt
		if prompt == "" {
			return nil, fmt.Errorf("prompt is required")
		}

		prof := params.Profile
		if prof == "" {
			prof = "auto"
		}

		model := params.Model
		if model == "" {
			model = "gemini-3.8-flash"
		}

		timeoutSec := intOr(params.Timeout, 120)
		autoFailover := true
		if params.AutoFailover != nil {
			autoFailover = *params.AutoFailover
		}

		dangerouslySkipPermissions := true
		if params.DangerouslySkipPermissions != nil {
			dangerouslySkipPermissions = *params.DangerouslySkipPermissions
		}

		disableSlashCommands := true
		if params.DisableSlashCommands != nil {
			disableSlashCommands = *params.DisableSlashCommands
		}

		subOpts := agent.HeadlessSubagentOptions{
			Profile:                    prof,
			Prompt:                     prompt,
			Model:                      model,
			Effort:                     params.Effort,
			Weight:                     params.Weight,
			Timeout:                    time.Duration(timeoutSec) * time.Second,
			Cwd:                        params.Cwd,
			RepoPath:                   params.Repo,
			DangerouslySkipPermissions: dangerouslySkipPermissions,
			DisableSlashCommands:       disableSlashCommands,
			TraceID:                    params.TraceID,
			Env:                        params.Env,
			AutoFailover:               autoFailover,
			MaxFailovers:               intOr(params.MaxFailovers, 3),
		}

		res, err := agent.RunHeadlessSubagent(ctx, subOpts)
		if err != nil {
			return nil, fmt.Errorf("subagent execution failed: %w", err)
		}
		return toJSONText(res)
	})

	s.RegisterTool(Tool{
		Name:        "dispatch_plan",
		Description: "Execute a multi-agent execution plan with concurrent subtasks across isolated Git worktrees, aggregating diffs and validating disjoint scopes.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"plan_id": {
					Type:        "string",
					Description: "Optional custom plan identifier. Auto-generated if omitted.",
				},
				"repo": {
					Type:        "string",
					Description: "Path to target Git repository. Defaults to current working directory.",
				},
				"base_branch": {
					Type:        "string",
					Description: "Base branch to fork ephemeral worktrees from.",
				},
				"base_commit": {
					Type:        "string",
					Description: "Base commit hash to fork ephemeral worktrees from.",
				},
				"timeout": {
					Type:        "string",
					Description: "Global plan execution timeout (e.g., '10m', '30m').",
				},
				"workers": {
					Type:        "integer",
					Description: "Maximum number of concurrent worker tasks executing simultaneously (default 4).",
				},
				"subtasks": {
					Type:        "array",
					Description: "List of subtask specifications to execute in parallel worktrees.",
					Items: &PropertySchema{
						Type: "object",
						Properties: map[string]PropertySchema{
							"id": {
								Type:        "string",
								Description: "Unique subtask identifier.",
							},
							"profile": {
								Type:        "string",
								Description: "Target Antigravity profile for identity, quota, and credential isolation.",
							},
							"prompt": {
								Type:        "string",
								Description: "Prompt instruction for the worker agent.",
							},
							"agent_type": {
								Type:        "string",
								Description: "Agent runner type (e.g., 'claude', 'aider', 'opencode', 'agy', 'codex').",
							},
							"command": {
								Type:        "string",
								Description: "Custom command or script to execute inside the worktree.",
							},
							"args": {
								Type:        "array",
								Description: "Command arguments array.",
								Items:       &PropertySchema{Type: "string"},
							},
							"branch": {
								Type:        "string",
								Description: "Custom branch name for this subtask's worktree.",
							},
							"env": {
								Type:        "object",
								Description: "Environment variables map for this subtask.",
							},
						},
						Required: []string{"profile"},
					},
				},
			},
			Required: []string{"subtasks"},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		repo := getString(args, "repo")
		if repo == "" {
			repo = getString(args, "repo_path")
		}
		if repo == "" {
			var err error
			repo, err = os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("failed to determine repository path: %w", err)
			}
		}

		var req dispatch.PlanRequest
		b, err := json.Marshal(args)
		if err == nil {
			_ = json.Unmarshal(b, &req)
		}
		req.RepoPath = repo
		if req.Workers <= 0 {
			req.Workers = getInt(args, "workers", 4)
		}
		if req.PlanID == "" {
			req.PlanID = getString(args, "plan_id")
		}
		if req.BaseBranch == "" {
			req.BaseBranch = getString(args, "base_branch")
		}
		if req.BaseCommit == "" {
			req.BaseCommit = getString(args, "base_commit")
		}
		if req.Timeout == "" {
			req.Timeout = getString(args, "timeout")
		}

		if len(req.Subtasks) == 0 && len(req.Tasks) == 0 {
			return nil, fmt.Errorf("subtasks list cannot be empty")
		}

		mgr := dispatch.GetDefaultTaskManager()
		result, err := mgr.ExecutePlan(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to execute plan: %w", err)
		}

		return toJSONText(result)
	})

	type dispatchListParams struct {
		Status  string `json:"status" description:"Filter tasks by status ('pending', 'running', 'completed', 'failed', 'cancelled')." enum:"pending,running,completed,failed,cancelled"`
		Profile string `json:"profile" description:"Filter tasks by profile name."`
		Repo    string `json:"repo" description:"Filter tasks by repository path."`
		Limit   *int   `json:"limit" description:"Maximum number of tasks to return (default 20)."`
	}
	RegisterTypedTool(s, "dispatch_list", "List dispatched tasks with optional filters by status, profile, or repository.", func(ctx context.Context, params dispatchListParams) (*CallToolResult, error) {
		repo := params.Repo
		if repo == "" {
			repo, _ = os.Getwd()
		}

		filter := dispatch.TaskFilter{
			Status:  dispatch.TaskStatus(params.Status),
			Profile: params.Profile,
		}

		limit := 20
		if params.Limit != nil {
			limit = *params.Limit
		}

		mgr := dispatch.GetDefaultTaskManager()
		tasks, err := mgr.ListTasks(repo, filter)
		if err != nil {
			return nil, fmt.Errorf("failed to list tasks: %w", err)
		}
		if tasks == nil {
			tasks = []dispatch.Task{}
		}
		if limit > 0 && len(tasks) > limit {
			tasks = tasks[:limit]
		}

		return toJSONText(tasks)
	})

	type dispatchStatusParams struct {
		TaskID string `json:"task_id" description:"The unique ID of the task." required:"true"`
		Repo   string `json:"repo" description:"Optional path to the Git repository containing the task."`
	}
	RegisterTypedTool(s, "dispatch_status", "Get detailed execution status, runtime duration, exit code, and worktree info of a dispatched task.", func(ctx context.Context, params dispatchStatusParams) (*CallToolResult, error) {
		taskID := params.TaskID
		if taskID == "" {
			return nil, fmt.Errorf("task_id is required")
		}

		mgr := dispatch.GetDefaultTaskManager()
		task, err := mgr.GetTask(params.Repo, taskID)
		if err != nil {
			return nil, err
		}
		return toJSONText(task)
	})

	type dispatchCancelParams struct {
		TaskID string `json:"task_id" description:"The unique ID of the task to cancel." required:"true"`
		Repo   string `json:"repo" description:"Optional repository path."`
		Force  bool   `json:"force" description:"Force immediate termination (default false)."`
	}
	RegisterTypedTool(s, "dispatch_cancel", "Cancel a currently running dispatched task.", func(ctx context.Context, params dispatchCancelParams) (*CallToolResult, error) {
		taskID := params.TaskID
		if taskID == "" {
			return nil, fmt.Errorf("task_id is required")
		}

		mgr := dispatch.GetDefaultTaskManager()
		force := params.Force
		if err := mgr.CancelTask(params.Repo, taskID, force); err != nil {
			return nil, err
		}
		return NewTextResult(fmt.Sprintf("Task %s successfully cancelled", taskID)), nil
	})

	type dispatchLogsParams struct {
		TaskID string `json:"task_id" description:"The unique ID of the task." required:"true"`
		Repo   string `json:"repo" description:"Optional repository path."`
		Tail   *int   `json:"tail" description:"Number of tail lines or bytes to retrieve (default 10000, 0 for all)."`
	}
	RegisterTypedTool(s, "dispatch_logs", "Retrieve terminal logs and output from a dispatched task.", func(ctx context.Context, params dispatchLogsParams) (*CallToolResult, error) {
		taskID := params.TaskID
		if taskID == "" {
			return nil, fmt.Errorf("task_id is required")
		}

		mgr := dispatch.GetDefaultTaskManager()
		tail := 10000
		if params.Tail != nil {
			tail = *params.Tail
		}
		logs, err := mgr.GetTaskLogs(params.Repo, taskID, tail)
		if err != nil {
			return nil, err
		}
		return NewTextResult(string(logs)), nil
	})

	// ==========================================
	// 2. Status & Inspection Tools
	// ==========================================

	s.RegisterTool(Tool{
		Name:        "profile_list",
		Description: "List all Antigravity profiles with their execution state, running PID, ports, disk usage, and authentication mode.",
		InputSchema: ToolInputSchema{
			Type:       "object",
			Properties: map[string]PropertySchema{},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		profiles, err := profile.GetProfiles()
		if err != nil {
			return nil, fmt.Errorf("failed to list profiles: %w", err)
		}
		if profiles == nil {
			profiles = []profile.ProfileInfo{}
		}
		return toJSONText(profiles)
	})

	type profileStatusParams struct {
		Profile string `json:"profile" description:"Name of the target profile." required:"true"`
	}
	RegisterTypedTool(s, "profile_status", "Retrieve detailed status, storage metrics, and sharing configurations for a specific Antigravity profile.", func(ctx context.Context, params profileStatusParams) (*CallToolResult, error) {
		name := params.Profile
		if name == "" {
			return nil, fmt.Errorf("profile name is required")
		}

		info, err := profile.GetProfile(name)
		if err != nil {
			return nil, err
		}

		stat, _ := profile.GetSingleProfileStat(name)
		sharing, _ := profile.GetAllSharingStatus(name)

		res := map[string]any{
			"profile": info,
			"stats":   stat,
			"sharing": sharing,
		}
		return toJSONText(res)
	})

	s.RegisterTool(Tool{
		Name:        "doctor_diagnose",
		Description: "Run Multigravity Doctor diagnostics to check IDE installations, language servers, toolchains, and environment health.",
		InputSchema: ToolInputSchema{
			Type:       "object",
			Properties: map[string]PropertySchema{},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		rep, err := doctor.Diagnose()
		if err != nil {
			return nil, fmt.Errorf("doctor diagnosis failed: %w", err)
		}
		return toJSONText(rep)
	})

	type workspaceListParams struct {
		ActiveOnly bool   `json:"active_only" description:"When true, returns only currently active workspaces in running profiles."`
		Profile    string `json:"profile" description:"Filter workspaces by profile name."`
	}
	RegisterTypedTool(s, "workspace_list", "List detected project workspaces and active Git repositories across Antigravity profiles.", func(ctx context.Context, params workspaceListParams) (*CallToolResult, error) {
		prof := params.Profile
		activeOnly := params.ActiveOnly

		var workspaces []workspace.Workspace
		var err error

		if activeOnly {
			workspaces, err = workspace.GetActiveWorkspaces()
		} else if prof != "" {
			workspaces, err = workspace.GetProfileWorkspaces(prof)
		} else {
			workspaces, err = workspace.GetAllWorkspaces()
		}

		if err != nil {
			return nil, fmt.Errorf("failed to query workspaces: %w", err)
		}
		if workspaces == nil {
			workspaces = []workspace.Workspace{}
		}

		return toJSONText(workspaces)
	})

	type alertsListParams struct {
		Profile string `json:"profile" description:"Optional profile name to inspect (or omit to inspect all profiles)."`
	}
	RegisterTypedTool(s, "alerts_list", "Evaluate and report active system alerts including critical quota thresholds (<=5%), sudden quota drops, and dead headless processes.", func(ctx context.Context, params alertsListParams) (*CallToolResult, error) {
		prof := params.Profile
		report, err := alert.Evaluate(prof)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate alerts: %w", err)
		}
		return toJSONText(report)
	})

	// ==========================================
	// 3. Quota & Priming Tools
	// ==========================================

	type quotaSummaryParams struct {
		Profile string `json:"profile" description:"Optional target profile name. If omitted, queries all active profiles."`
	}
	RegisterTypedTool(s, "quota_summary", "Query live AI quota limits (5-hour sliding window and weekly limits, percentage remaining, reset countdown) from running Language Servers.", func(ctx context.Context, params quotaSummaryParams) (*CallToolResult, error) {
		prof := params.Profile
		if prof != "" {
			if err := config.ValidateProfileName(prof); err != nil {
				return nil, err
			}
			if !profile.ProfileExists(prof) {
				return nil, fmt.Errorf("profile %q does not exist", prof)
			}
		}

		servers, err := quota.FindActiveServers(prof)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve quota: %w", err)
		}
		quota.RecordLiveSnapshots(servers)

		summary, err := quota.BuildQuotaSummary(prof, servers)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve quota: %w", err)
		}
		return toJSONText(summary)
	})

	type quotaHistoryParams struct {
		Limit   *int   `json:"limit" description:"Maximum number of history points to return (default 50)."`
		Profile string `json:"profile" description:"Profile name (if omitted, aggregates history across all profiles)."`
		Since   string `json:"since" description:"RFC3339 timestamp start filter."`
		Source  string `json:"source" description:"Filter by source ('quota', 'gateway', 'headless')." enum:"quota,gateway,headless"`
		Until   string `json:"until" description:"RFC3339 timestamp end filter."`
	}
	RegisterTypedTool(s, "quota_history", "Retrieve historical time-series of quota percentages and token consumption for one or all profiles.", func(ctx context.Context, params quotaHistoryParams) (*CallToolResult, error) {
		prof := params.Profile
		since := params.Since
		until := params.Until
		source := params.Source
		limit := intOr(params.Limit, 50)

		query, err := quota.ParseHistoryQuery(since, until, source, strconv.Itoa(limit))
		if err != nil {
			return nil, err
		}

		if prof != "" {
			if err := config.ValidateProfileName(prof); err != nil {
				return nil, err
			}
			series, err := quota.LoadSeries(prof, query)
			if err != nil {
				return nil, err
			}
			return toJSONText(series)
		}

		series, err := quota.LoadFleetSeries(query)
		if err != nil {
			return nil, err
		}
		return toJSONText(series)
	})

	type primeStatusParams struct {
		Profile string `json:"profile" description:"Optional target profile name."`
	}
	RegisterTypedTool(s, "prime_status", "Check AI quota priming status, scheduled watchdog timers, and last primed timestamps for profiles.", func(ctx context.Context, params primeStatusParams) (*CallToolResult, error) {
		prof := params.Profile
		if prof != "" {
			st, err := prime.GetProfilePrimeStatus(prof)
			if err != nil {
				return nil, err
			}
			return toJSONText(st)
		}

		st, err := prime.GetAllProfilesPrimeStatus()
		if err != nil {
			return nil, err
		}
		return toJSONText(st)
	})

	type primeTriggerParams struct {
		Check   bool   `json:"check" description:"Dry-run check: verify if profiles can be primed without sending actual prompts."`
		Force   bool   `json:"force" description:"Force priming regardless of quota percentage or reset timing."`
		Profile string `json:"profile" description:"Profile name to prime (omit to prime all eligible profiles)."`
		Warm5h  bool   `json:"warm_5h" description:"Proactively send a minimal 1-token prompt to start the rolling 5-hour window countdown."`
	}
	RegisterTypedTool(s, "prime_trigger", "Trigger proactive quota priming or 5-hour window warm-up for one or all profiles.", func(ctx context.Context, params primeTriggerParams) (*CallToolResult, error) {
		opts := prime.PrimeOptions{
			Profile:  params.Profile,
			Warm5h:   params.Warm5h,
			Force:    params.Force,
			Check:    params.Check,
			NoJitter: true,
		}

		res, err := prime.ExecutePrime(opts, nil)
		if err != nil {
			return nil, fmt.Errorf("priming failed: %w", err)
		}
		return toJSONText(res)
	})

	// ==========================================
	// 4. Diff Inspection Tools
	// ==========================================

	type taskDiffParams struct {
		TaskID     string `json:"task_id" description:"The unique ID of the dispatched task." required:"true"`
		Repo       string `json:"repo" description:"Optional repository path."`
		Structured *bool  `json:"structured" description:"Return structured JSON containing changed files, hunks, line additions and deletions (default true)."`
	}
	RegisterTypedTool(s, "task_diff", "Inspect the Git diff generated by a dispatched task within its isolated worktree. Returns summary, changed files, hunks, or unified diff.", func(ctx context.Context, params taskDiffParams) (*CallToolResult, error) {
		taskID := params.TaskID
		if taskID == "" {
			return nil, fmt.Errorf("task_id is required")
		}

		repo := params.Repo
		mgr := dispatch.GetDefaultTaskManager()

		structured := true
		if params.Structured != nil {
			structured = *params.Structured
		}

		if structured {
			sd, err := mgr.GetTaskStructuredDiff(repo, taskID)
			if err != nil {
				return nil, err
			}
			return toJSONText(sd)
		}

		diff, err := mgr.GetTaskDiff(repo, taskID, false)
		if err != nil {
			return nil, err
		}
		return NewTextResult(diff), nil
	})

	type worktreeDiffParams struct {
		WorktreeID string `json:"worktree_id" description:"The ID of the worktree to inspect." required:"true"`
		Repo       string `json:"repo" description:"Path to the base Git repository."`
	}
	RegisterTypedTool(s, "worktree_diff", "Inspect the Git diff of a specific ephemeral worktree compared against its base branch or commit.", func(ctx context.Context, params worktreeDiffParams) (*CallToolResult, error) {
		wtID := params.WorktreeID
		if wtID == "" {
			return nil, fmt.Errorf("worktree_id is required")
		}

		repo := params.Repo
		if repo == "" {
			var err error
			repo, err = os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("failed to get current directory: %w", err)
			}
		}

		diff, err := worktree.GetWorktreeDiff(repo, wtID, worktree.DiffOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to inspect worktree diff: %w", err)
		}

		return NewTextResult(diff), nil
	})
}

// NewDefaultServer creates an MCP Server with all native tools registered
func NewDefaultServer() *Server {
	srv := NewServer()
	RegisterDefaultTools(srv)
	return srv
}
