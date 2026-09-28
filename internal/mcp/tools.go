package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

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

	s.RegisterTool(Tool{
		Name:        "dispatch_task",
		Description: "Dispatch an agent command or script within an isolated Git worktree or specified repository using a dedicated Antigravity profile.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"command": {
					Type:        "string",
					Description: "Command or agent instruction to execute (e.g., 'claude -p \"fix bug\"' or shell script).",
				},
				"args": {
					Type:        "array",
					Description: "Command arguments array. If provided, overrides automatic argument resolution.",
					Items:       &PropertySchema{Type: "string"},
				},
				"prompt": {
					Type:        "string",
					Description: "Prompt instruction for the agent command (e.g. passed as -p to agy or claude if args is not provided).",
				},
				"profile": {
					Type:        "string",
					Description: "Antigravity profile name for identity and credentials isolation (e.g. 'yegear', 'yegear2').",
				},
				"repo": {
					Type:        "string",
					Description: "Path to target Git repository. Defaults to current directory or detected active workspace.",
				},
				"branch": {
					Type:        "string",
					Description: "Branch name to create or use inside an isolated Git worktree.",
				},
				"worktree": {
					Type:        "string",
					Description: "Custom worktree path or reuse identifier. If empty and branch is given, an ephemeral worktree is created.",
				},
				"agent_type": {
					Type:        "string",
					Description: "Agent type for command resolution (e.g., 'claude', 'aider', 'opencode', 'agy').",
				},
				"background": {
					Type:        "boolean",
					Description: "Whether to dispatch detached in background and return immediately with the task ID (default false).",
				},
				"env": {
					Type:        "object",
					Description: "Custom environment variables key-value map to pass into the task session.",
				},
			},
			Required: []string{"command", "profile"},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		cmdStr := getString(args, "command")
		if cmdStr == "" {
			return nil, fmt.Errorf("command is required")
		}

		prof := getString(args, "profile")
		if prof == "" {
			return nil, fmt.Errorf("profile is required")
		}
		if err := config.ValidateProfileName(prof); err != nil {
			return nil, err
		}
		if !profile.ProfileExists(prof) {
			return nil, fmt.Errorf("profile %q does not exist", prof)
		}

		repo := getString(args, "repo")
		if repo == "" {
			var err error
			repo, err = os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("failed to determine repository path: %w", err)
			}
		}

		branch := getString(args, "branch")
		wtID := getString(args, "worktree")
		newWorktree := (wtID == "" && branch != "")

		opts := dispatch.DispatchOptions{
			Command:     cmdStr,
			Args:        getStringSlice(args, "args"),
			Prompt:      getString(args, "prompt"),
			Profile:     prof,
			RepoPath:    repo,
			Branch:      branch,
			WorktreeID:  wtID,
			NewWorktree: newWorktree,
			AgentType:   getString(args, "agent_type"),
			Detached:    getBool(args, "background"),
			Env:         getStringMap(args, "env"),
		}

		mgr := dispatch.GetDefaultTaskManager()
		task, err := mgr.Dispatch(opts)
		if err != nil {
			return nil, fmt.Errorf("failed to dispatch task: %w", err)
		}

		return toJSONText(task)
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
								Description: "Agent runner type (e.g., 'claude', 'aider', 'opencode', 'agy').",
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

	s.RegisterTool(Tool{
		Name:        "dispatch_list",
		Description: "List dispatched tasks with optional filters by status, profile, or repository.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"status": {
					Type:        "string",
					Description: "Filter tasks by status ('pending', 'running', 'completed', 'failed', 'cancelled').",
					Enum:        []string{"pending", "running", "completed", "failed", "cancelled"},
				},
				"profile": {
					Type:        "string",
					Description: "Filter tasks by profile name.",
				},
				"repo": {
					Type:        "string",
					Description: "Filter tasks by repository path.",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of tasks to return (default 20).",
				},
			},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		repo := getString(args, "repo")
		if repo == "" {
			repo, _ = os.Getwd()
		}

		filter := dispatch.TaskFilter{
			Status:  dispatch.TaskStatus(getString(args, "status")),
			Profile: getString(args, "profile"),
		}

		limit := getInt(args, "limit", 20)

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

	s.RegisterTool(Tool{
		Name:        "dispatch_status",
		Description: "Get detailed execution status, runtime duration, exit code, and worktree info of a dispatched task.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"task_id": {
					Type:        "string",
					Description: "The unique ID of the task.",
				},
				"repo": {
					Type:        "string",
					Description: "Optional path to the Git repository containing the task.",
				},
			},
			Required: []string{"task_id"},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		taskID := getString(args, "task_id")
		if taskID == "" {
			return nil, fmt.Errorf("task_id is required")
		}

		mgr := dispatch.GetDefaultTaskManager()
		task, err := mgr.GetTask(getString(args, "repo"), taskID)
		if err != nil {
			return nil, err
		}
		return toJSONText(task)
	})

	s.RegisterTool(Tool{
		Name:        "dispatch_cancel",
		Description: "Cancel a currently running dispatched task.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"task_id": {
					Type:        "string",
					Description: "The unique ID of the task to cancel.",
				},
				"repo": {
					Type:        "string",
					Description: "Optional repository path.",
				},
				"force": {
					Type:        "boolean",
					Description: "Force immediate termination (default false).",
				},
			},
			Required: []string{"task_id"},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		taskID := getString(args, "task_id")
		if taskID == "" {
			return nil, fmt.Errorf("task_id is required")
		}

		mgr := dispatch.GetDefaultTaskManager()
		force := getBool(args, "force")
		if err := mgr.CancelTask(getString(args, "repo"), taskID, force); err != nil {
			return nil, err
		}
		return NewTextResult(fmt.Sprintf("Task %s successfully cancelled", taskID)), nil
	})

	s.RegisterTool(Tool{
		Name:        "dispatch_logs",
		Description: "Retrieve terminal logs and output from a dispatched task.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"task_id": {
					Type:        "string",
					Description: "The unique ID of the task.",
				},
				"repo": {
					Type:        "string",
					Description: "Optional repository path.",
				},
				"tail": {
					Type:        "integer",
					Description: "Number of tail lines or bytes to retrieve (default 10000, 0 for all).",
				},
			},
			Required: []string{"task_id"},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		taskID := getString(args, "task_id")
		if taskID == "" {
			return nil, fmt.Errorf("task_id is required")
		}

		mgr := dispatch.GetDefaultTaskManager()
		tail := getInt(args, "tail", 10000)
		logs, err := mgr.GetTaskLogs(getString(args, "repo"), taskID, tail)
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

	s.RegisterTool(Tool{
		Name:        "profile_status",
		Description: "Retrieve detailed status, storage metrics, and sharing configurations for a specific Antigravity profile.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"profile": {
					Type:        "string",
					Description: "Name of the target profile.",
				},
			},
			Required: []string{"profile"},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		name := getString(args, "profile")
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

	s.RegisterTool(Tool{
		Name:        "workspace_list",
		Description: "List detected project workspaces and active Git repositories across Antigravity profiles.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"profile": {
					Type:        "string",
					Description: "Filter workspaces by profile name.",
				},
				"active_only": {
					Type:        "boolean",
					Description: "When true, returns only currently active workspaces in running profiles.",
				},
			},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		prof := getString(args, "profile")
		activeOnly := getBool(args, "active_only")

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

	s.RegisterTool(Tool{
		Name:        "alerts_list",
		Description: "Evaluate and report active system alerts including critical quota thresholds (<=5%), sudden quota drops, and dead headless processes.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"profile": {
					Type:        "string",
					Description: "Optional profile name to inspect (or omit to inspect all profiles).",
				},
			},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		prof := getString(args, "profile")
		report, err := alert.Evaluate(prof)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate alerts: %w", err)
		}
		return toJSONText(report)
	})

	// ==========================================
	// 3. Quota & Priming Tools
	// ==========================================

	s.RegisterTool(Tool{
		Name:        "quota_summary",
		Description: "Query live AI quota limits (5-hour sliding window and weekly limits, percentage remaining, reset countdown) from running Language Servers.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"profile": {
					Type:        "string",
					Description: "Optional target profile name. If omitted, queries all active profiles.",
				},
			},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		prof := getString(args, "profile")
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

		if servers == nil {
			servers = []quota.ActiveServer{}
		}
		return toJSONText(servers)
	})

	s.RegisterTool(Tool{
		Name:        "quota_history",
		Description: "Retrieve historical time-series of quota percentages and token consumption for one or all profiles.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"profile": {
					Type:        "string",
					Description: "Profile name (if omitted, aggregates history across all profiles).",
				},
				"source": {
					Type:        "string",
					Description: "Filter by source ('quota', 'gateway', 'headless').",
					Enum:        []string{"quota", "gateway", "headless"},
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of history points to return (default 50).",
				},
				"since": {
					Type:        "string",
					Description: "RFC3339 timestamp start filter.",
				},
				"until": {
					Type:        "string",
					Description: "RFC3339 timestamp end filter.",
				},
			},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		prof := getString(args, "profile")
		since := getString(args, "since")
		until := getString(args, "until")
		source := getString(args, "source")
		limit := getInt(args, "limit", 50)

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

	s.RegisterTool(Tool{
		Name:        "prime_status",
		Description: "Check AI quota priming status, scheduled watchdog timers, and last primed timestamps for profiles.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"profile": {
					Type:        "string",
					Description: "Optional target profile name.",
				},
			},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		prof := getString(args, "profile")
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

	s.RegisterTool(Tool{
		Name:        "prime_trigger",
		Description: "Trigger proactive quota priming or 5-hour window warm-up for one or all profiles.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"profile": {
					Type:        "string",
					Description: "Profile name to prime (omit to prime all eligible profiles).",
				},
				"warm_5h": {
					Type:        "boolean",
					Description: "Proactively send a minimal 1-token prompt to start the rolling 5-hour window countdown.",
				},
				"force": {
					Type:        "boolean",
					Description: "Force priming regardless of quota percentage or reset timing.",
				},
				"check": {
					Type:        "boolean",
					Description: "Dry-run check: verify if profiles can be primed without sending actual prompts.",
				},
			},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		opts := prime.PrimeOptions{
			Profile:  getString(args, "profile"),
			Warm5h:   getBool(args, "warm_5h"),
			Force:    getBool(args, "force"),
			Check:    getBool(args, "check"),
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

	s.RegisterTool(Tool{
		Name:        "task_diff",
		Description: "Inspect the Git diff generated by a dispatched task within its isolated worktree. Returns summary, changed files, hunks, or unified diff.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"task_id": {
					Type:        "string",
					Description: "The unique ID of the dispatched task.",
				},
				"repo": {
					Type:        "string",
					Description: "Optional repository path.",
				},
				"structured": {
					Type:        "boolean",
					Description: "Return structured JSON containing changed files, hunks, line additions and deletions (default true).",
				},
			},
			Required: []string{"task_id"},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		taskID := getString(args, "task_id")
		if taskID == "" {
			return nil, fmt.Errorf("task_id is required")
		}

		repo := getString(args, "repo")
		mgr := dispatch.GetDefaultTaskManager()

		structured := true
		if _, ok := args["structured"]; ok {
			structured = getBool(args, "structured")
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

	s.RegisterTool(Tool{
		Name:        "worktree_diff",
		Description: "Inspect the Git diff of a specific ephemeral worktree compared against its base branch or commit.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				"worktree_id": {
					Type:        "string",
					Description: "The ID of the worktree to inspect.",
				},
				"repo": {
					Type:        "string",
					Description: "Path to the base Git repository.",
				},
			},
			Required: []string{"worktree_id"},
		},
	}, func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
		wtID := getString(args, "worktree_id")
		if wtID == "" {
			return nil, fmt.Errorf("worktree_id is required")
		}

		repo := getString(args, "repo")
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
