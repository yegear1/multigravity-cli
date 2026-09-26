package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/agent"
	"github.com/ye-dev/multigravity-cli/internal/alert"
	"github.com/ye-dev/multigravity-cli/internal/chat"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/dispatch"
	"github.com/ye-dev/multigravity-cli/internal/doctor"
	"github.com/ye-dev/multigravity-cli/internal/headless"
	"github.com/ye-dev/multigravity-cli/internal/idelog"
	"github.com/ye-dev/multigravity-cli/internal/prime"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
	"github.com/ye-dev/multigravity-cli/internal/server/ui"
	"github.com/ye-dev/multigravity-cli/internal/workspace"
	"github.com/ye-dev/multigravity-cli/internal/worktree"
)

// APIResponse standardizes the JSON response payload
type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// HealthResponse holds the health check status
type HealthResponse struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// StopRequest defines the JSON payload for stopping a profile
type StopRequest struct {
	Force bool `json:"force"`
}

// CreateProfileRequest defines the payload for creating a new profile
type CreateProfileRequest struct {
	Name             string `json:"name"`
	AuthOnly         bool   `json:"auth_only"`
	AuthOnlyKebab    bool   `json:"auth-only,omitempty"`
	Shared           bool   `json:"shared"`
	IsolatedDotfiles bool   `json:"isolated_dotfiles"`
	IsolatedMCP      bool   `json:"isolated_mcp"`
	IsolatedSkills   bool   `json:"isolated_skills"`
	IsolatedConfig   bool   `json:"isolated_config"`
	IsolatedGH       bool   `json:"isolated_gh"`
	Color            string `json:"color"`
	FromTemplate     string `json:"from_template"`
}

// DeleteProfileRequest defines optional parameters for deleting a profile
type DeleteProfileRequest struct {
	Force bool `json:"force"`
}

// LaunchProfileRequest defines optional parameters for launching or restarting a profile
type LaunchProfileRequest struct {
	Args []string `json:"args,omitempty"`
}

// RenameProfileRequest defines the payload for renaming a profile
type RenameProfileRequest struct {
	NewName string `json:"new_name"`
}

// SetSharingRequest defines the payload for setting or toggling sharing for a resource
type SetSharingRequest struct {
	Action   string `json:"action,omitempty"`
	Mode     string `json:"mode,omitempty"`
	Shared   *bool  `json:"shared,omitempty"`
	Resource string `json:"resource,omitempty"`
}

// PrimeRequest defines parameters for triggering prime on a single profile
type PrimeRequest struct {
	Force        bool    `json:"force"`
	Check        bool    `json:"check"`
	Include5h    bool    `json:"include_5h"`
	Include5hAlt bool    `json:"5h,omitempty"`
	Warm5h       bool    `json:"warm_5h"`
	Warm5hAlt    bool    `json:"warm-5h,omitempty"`
	NoJitter     bool    `json:"no_jitter"`
	MaxJitter    float64 `json:"max_jitter"`
}

// BatchPrimeRequest defines parameters for triggering prime on multiple profiles
type BatchPrimeRequest struct {
	Profiles     []string `json:"profiles,omitempty"`
	Force        bool     `json:"force"`
	Check        bool     `json:"check"`
	Include5h    bool     `json:"include_5h"`
	Include5hAlt bool     `json:"5h,omitempty"`
	Warm5h       bool     `json:"warm_5h"`
	Warm5hAlt    bool     `json:"warm-5h,omitempty"`
	NoJitter     bool     `json:"no_jitter"`
	MaxJitter    float64  `json:"max_jitter"`
}

// CreateWorktreeRequest defines parameters for creating a new worktree via API
type CreateWorktreeRequest struct {
	ID         string            `json:"id"`
	RepoPath   string            `json:"repo_path,omitempty"`
	Branch     string            `json:"branch,omitempty"`
	BaseCommit string            `json:"base_commit,omitempty"`
	TargetDir  string            `json:"target_dir,omitempty"`
	Profile    string            `json:"profile,omitempty"`
	TaskID     string            `json:"task_id,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CopyFiles  []string          `json:"copy_files,omitempty"`
	LinkPaths  []string          `json:"link_paths,omitempty"`
}

// DeleteWorktreeRequest defines optional body parameters for deleting a worktree
type DeleteWorktreeRequest struct {
	Repo         string `json:"repo,omitempty"`
	Force        bool   `json:"force"`
	DeleteBranch bool   `json:"delete_branch"`
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    data,
	})
}

func (s *Server) writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error:   msg,
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, X-Requested-With, X-Profile, X-Routing-Strategy, X-Failover, x-api-key, anthropic-version, anthropic-beta")
		w.Header().Set("Access-Control-Expose-Headers", "X-Profile-Used, X-Failover-Count, X-Remaining-Profiles, X-Routing-Strategy")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) setupRoutes() {
	// Health check
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	// Doctor
	s.mux.HandleFunc("GET /api/v1/doctor", s.handleDoctor)
	s.mux.HandleFunc("GET /api/doctor", s.handleDoctor)

	// Profiles (Query & Mutation)
	s.mux.HandleFunc("GET /api/v1/profiles", s.handleListProfiles)
	s.mux.HandleFunc("GET /api/profiles", s.handleListProfiles)
	s.mux.HandleFunc("POST /api/v1/profiles", s.handleCreateProfile)
	s.mux.HandleFunc("POST /api/profiles", s.handleCreateProfile)

	s.mux.HandleFunc("GET /api/v1/profiles/{name}", s.handleGetProfile)
	s.mux.HandleFunc("GET /api/profiles/{name}", s.handleGetProfile)
	s.mux.HandleFunc("DELETE /api/v1/profiles/{name}", s.handleDeleteProfile)
	s.mux.HandleFunc("DELETE /api/profiles/{name}", s.handleDeleteProfile)

	s.mux.HandleFunc("GET /api/v1/profiles/{name}/stats", s.handleGetProfileStats)
	s.mux.HandleFunc("GET /api/profiles/{name}/stats", s.handleGetProfileStats)
	s.mux.HandleFunc("GET /api/v1/profiles/{name}/ide/logs", s.handleGetIDELogs)
	s.mux.HandleFunc("GET /api/profiles/{name}/ide/logs", s.handleGetIDELogs)
	s.mux.HandleFunc("GET /api/v1/stats", s.handleStats)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)

	// Profile Actions (Launch, Stop, Restart, Clean, Rename)
	s.mux.HandleFunc("POST /api/v1/profiles/{name}/launch", s.handleLaunchProfile)
	s.mux.HandleFunc("POST /api/profiles/{name}/launch", s.handleLaunchProfile)
	s.mux.HandleFunc("POST /api/v1/profiles/{name}/stop", s.handleStopProfile)
	s.mux.HandleFunc("POST /api/profiles/{name}/stop", s.handleStopProfile)
	s.mux.HandleFunc("POST /api/v1/profiles/{name}/restart", s.handleRestartProfile)
	s.mux.HandleFunc("POST /api/profiles/{name}/restart", s.handleRestartProfile)
	s.mux.HandleFunc("POST /api/v1/profiles/{name}/clean", s.handleCleanProfile)
	s.mux.HandleFunc("POST /api/profiles/{name}/clean", s.handleCleanProfile)
	s.mux.HandleFunc("POST /api/v1/profiles/{name}/rename", s.handleRenameProfile)
	s.mux.HandleFunc("POST /api/profiles/{name}/rename", s.handleRenameProfile)

	s.mux.HandleFunc("GET /api/v1/profiles/{name}/snapshots", s.handleListSnapshots)
	s.mux.HandleFunc("GET /api/profiles/{name}/snapshots", s.handleListSnapshots)
	s.mux.HandleFunc("POST /api/v1/profiles/{name}/snapshots", s.handleCreateSnapshot)
	s.mux.HandleFunc("POST /api/profiles/{name}/snapshots", s.handleCreateSnapshot)
	s.mux.HandleFunc("POST /api/v1/profiles/{name}/snapshots/{id}/rollback", s.handleRollbackSnapshot)
	s.mux.HandleFunc("POST /api/profiles/{name}/snapshots/{id}/rollback", s.handleRollbackSnapshot)
	s.mux.HandleFunc("DELETE /api/v1/profiles/{name}/snapshots/{id}", s.handleDeleteSnapshot)
	s.mux.HandleFunc("DELETE /api/profiles/{name}/snapshots/{id}", s.handleDeleteSnapshot)

	// Sharing (Query & Mutation)
	s.mux.HandleFunc("GET /api/v1/profiles/{name}/sharing", s.handleGetAllSharing)
	s.mux.HandleFunc("GET /api/profiles/{name}/sharing", s.handleGetAllSharing)
	s.mux.HandleFunc("GET /api/v1/profiles/{name}/sharing/{resource}", s.handleGetResourceSharing)
	s.mux.HandleFunc("GET /api/profiles/{name}/sharing/{resource}", s.handleGetResourceSharing)

	s.mux.HandleFunc("POST /api/v1/profiles/{name}/sharing/{resource}", s.handleSetResourceSharing)
	s.mux.HandleFunc("POST /api/profiles/{name}/sharing/{resource}", s.handleSetResourceSharing)
	s.mux.HandleFunc("PUT /api/v1/profiles/{name}/sharing/{resource}", s.handleSetResourceSharing)
	s.mux.HandleFunc("PUT /api/profiles/{name}/sharing/{resource}", s.handleSetResourceSharing)

	s.mux.HandleFunc("POST /api/v1/profiles/{name}/sharing", s.handleBatchSharing)
	s.mux.HandleFunc("POST /api/profiles/{name}/sharing", s.handleBatchSharing)
	s.mux.HandleFunc("PUT /api/v1/profiles/{name}/sharing", s.handleBatchSharing)
	s.mux.HandleFunc("PUT /api/profiles/{name}/sharing", s.handleBatchSharing)

	s.mux.HandleFunc("POST /api/v1/profiles/{name}/sharing/config/seed", s.handleSeedConfig)
	s.mux.HandleFunc("POST /api/profiles/{name}/sharing/config/seed", s.handleSeedConfig)

	// AI Conversations
	s.mux.HandleFunc("GET /api/v1/profiles/{name}/conversations", s.handleGetConversations)
	s.mux.HandleFunc("GET /api/profiles/{name}/conversations", s.handleGetConversations)

	// Quota & Priming
	s.mux.HandleFunc("GET /api/v1/quota", s.handleQuota)
	s.mux.HandleFunc("GET /api/quota", s.handleQuota)
	s.mux.HandleFunc("GET /api/v1/quota/history", s.handleQuotaHistory)
	s.mux.HandleFunc("GET /api/quota/history", s.handleQuotaHistory)
	s.mux.HandleFunc("GET /api/v1/quota/{profile}/history", s.handleQuotaProfileHistory)
	s.mux.HandleFunc("GET /api/quota/{profile}/history", s.handleQuotaProfileHistory)
	s.mux.HandleFunc("GET /api/v1/quota/{profile}", s.handleQuotaProfile)
	s.mux.HandleFunc("GET /api/quota/{profile}", s.handleQuotaProfile)

	s.mux.HandleFunc("GET /api/v1/alerts", s.handleAlerts)
	s.mux.HandleFunc("GET /api/alerts", s.handleAlerts)
	s.mux.HandleFunc("GET /api/v1/alerts/{profile}", s.handleAlertsProfile)
	s.mux.HandleFunc("GET /api/alerts/{profile}", s.handleAlertsProfile)

	s.mux.HandleFunc("GET /api/v1/profiles/{name}/prime", s.handleGetProfilePrime)
	s.mux.HandleFunc("GET /api/profiles/{name}/prime", s.handleGetProfilePrime)
	s.mux.HandleFunc("GET /api/v1/prime", s.handleGetAllPrime)
	s.mux.HandleFunc("GET /api/prime", s.handleGetAllPrime)

	s.mux.HandleFunc("POST /api/v1/profiles/{name}/prime", s.handlePostProfilePrime)
	s.mux.HandleFunc("POST /api/profiles/{name}/prime", s.handlePostProfilePrime)
	s.mux.HandleFunc("POST /api/v1/prime", s.handlePostAllPrime)
	s.mux.HandleFunc("POST /api/prime", s.handlePostAllPrime)

	// Real-time Streaming (SSE)
	s.mux.HandleFunc("GET /events", s.handleEvents)
	s.mux.HandleFunc("GET /api/v1/events", s.handleEvents)

	// OpenAI-Compatible Completions Gateway
	s.mux.HandleFunc("POST /v1/chat/completions", s.handleGatewayChatCompletions)
	s.mux.HandleFunc("POST /api/v1/chat/completions", s.handleGatewayChatCompletions)
	s.mux.HandleFunc("GET /v1/models", s.handleGatewayModels)
	s.mux.HandleFunc("GET /api/v1/models", s.handleGatewayModels)

	// Anthropic-Compatible Messages Gateway
	s.mux.HandleFunc("POST /v1/messages", s.handleGatewayMessages)
	s.mux.HandleFunc("POST /api/v1/messages", s.handleGatewayMessages)

	// Multi-Account Router Management
	s.mux.HandleFunc("GET /v1/router/status", s.handleGatewayRouterStatus)
	s.mux.HandleFunc("GET /api/v1/router/status", s.handleGatewayRouterStatus)
	s.mux.HandleFunc("POST /v1/router/reset", s.handleGatewayRouterReset)
	s.mux.HandleFunc("POST /api/v1/router/reset", s.handleGatewayRouterReset)
	s.mux.HandleFunc("POST /v1/router/strategy", s.handleGatewayRouterStrategy)
	s.mux.HandleFunc("POST /api/v1/router/strategy", s.handleGatewayRouterStrategy)

	// Git Worktrees (ADE Orchestrator)
	s.mux.HandleFunc("GET /api/v1/worktrees", s.handleListWorktrees)
	s.mux.HandleFunc("GET /api/worktrees", s.handleListWorktrees)
	s.mux.HandleFunc("POST /api/v1/worktrees", s.handleCreateWorktree)
	s.mux.HandleFunc("POST /api/worktrees", s.handleCreateWorktree)
	s.mux.HandleFunc("POST /api/v1/worktrees/prune", s.handlePruneWorktrees)
	s.mux.HandleFunc("POST /api/worktrees/prune", s.handlePruneWorktrees)

	s.mux.HandleFunc("GET /api/v1/worktrees/{id}", s.handleGetWorktree)
	s.mux.HandleFunc("GET /api/worktrees/{id}", s.handleGetWorktree)
	s.mux.HandleFunc("DELETE /api/v1/worktrees/{id}", s.handleDeleteWorktree)
	s.mux.HandleFunc("DELETE /api/worktrees/{id}", s.handleDeleteWorktree)
	s.mux.HandleFunc("GET /api/v1/worktrees/{id}/status", s.handleGetWorktreeStatus)
	s.mux.HandleFunc("GET /api/worktrees/{id}/status", s.handleGetWorktreeStatus)
	s.mux.HandleFunc("GET /api/v1/worktrees/{id}/diff", s.handleGetWorktreeDiff)
	s.mux.HandleFunc("GET /api/worktrees/{id}/diff", s.handleGetWorktreeDiff)

	// Agent PTY & Headless Runner (ADE Orchestrator)
	s.mux.HandleFunc("GET /api/v1/agent/sessions", s.handleListAgentSessions)
	s.mux.HandleFunc("GET /api/agent/sessions", s.handleListAgentSessions)
	s.mux.HandleFunc("POST /api/v1/agent/sessions", s.handleCreateAgentSession)
	s.mux.HandleFunc("POST /api/agent/sessions", s.handleCreateAgentSession)
	s.mux.HandleFunc("POST /api/v1/agent/sessions/prune", s.handlePruneAgentSessions)
	s.mux.HandleFunc("POST /api/agent/sessions/prune", s.handlePruneAgentSessions)

	s.mux.HandleFunc("GET /api/v1/agent/sessions/{id}", s.handleGetAgentSession)
	s.mux.HandleFunc("GET /api/agent/sessions/{id}", s.handleGetAgentSession)
	s.mux.HandleFunc("DELETE /api/v1/agent/sessions/{id}", s.handleDeleteAgentSession)
	s.mux.HandleFunc("DELETE /api/agent/sessions/{id}", s.handleDeleteAgentSession)
	s.mux.HandleFunc("GET /api/v1/agent/sessions/{id}/output", s.handleGetAgentSessionOutput)
	s.mux.HandleFunc("GET /api/agent/sessions/{id}/output", s.handleGetAgentSessionOutput)
	s.mux.HandleFunc("GET /api/v1/agent/sessions/{id}/stream", s.handleStreamAgentSessionOutput)
	s.mux.HandleFunc("GET /api/agent/sessions/{id}/stream", s.handleStreamAgentSessionOutput)
	s.mux.HandleFunc("POST /api/v1/agent/sessions/{id}/input", s.handleSendAgentSessionInput)
	s.mux.HandleFunc("POST /api/agent/sessions/{id}/input", s.handleSendAgentSessionInput)
	s.mux.HandleFunc("POST /api/v1/agent/sessions/{id}/resize", s.handleResizeAgentSession)
	s.mux.HandleFunc("POST /api/agent/sessions/{id}/resize", s.handleResizeAgentSession)

	// Task Dispatcher (Orchestrator)
	s.mux.HandleFunc("GET /api/v1/dispatch/tasks", s.handleListDispatchedTasks)
	s.mux.HandleFunc("GET /api/dispatch/tasks", s.handleListDispatchedTasks)
	s.mux.HandleFunc("POST /api/v1/dispatch/tasks", s.handleCreateDispatchedTask)
	s.mux.HandleFunc("POST /api/dispatch/tasks", s.handleCreateDispatchedTask)
	s.mux.HandleFunc("POST /api/v1/dispatch/tasks/prune", s.handlePruneDispatchedTasks)
	s.mux.HandleFunc("POST /api/dispatch/tasks/prune", s.handlePruneDispatchedTasks)

	s.mux.HandleFunc("GET /api/v1/dispatch/tasks/{id}", s.handleGetDispatchedTask)
	s.mux.HandleFunc("GET /api/dispatch/tasks/{id}", s.handleGetDispatchedTask)
	s.mux.HandleFunc("DELETE /api/v1/dispatch/tasks/{id}", s.handleDeleteDispatchedTask)
	s.mux.HandleFunc("DELETE /api/dispatch/tasks/{id}", s.handleDeleteDispatchedTask)
	s.mux.HandleFunc("POST /api/v1/dispatch/tasks/{id}/cancel", s.handleCancelDispatchedTask)
	s.mux.HandleFunc("POST /api/dispatch/tasks/{id}/cancel", s.handleCancelDispatchedTask)
	s.mux.HandleFunc("GET /api/v1/dispatch/tasks/{id}/logs", s.handleGetDispatchedTaskLogs)
	s.mux.HandleFunc("GET /api/dispatch/tasks/{id}/logs", s.handleGetDispatchedTaskLogs)
	s.mux.HandleFunc("GET /api/v1/dispatch/tasks/{id}/stream", s.handleStreamDispatchedTaskLogs)
	s.mux.HandleFunc("GET /api/dispatch/tasks/{id}/stream", s.handleStreamDispatchedTaskLogs)
	s.mux.HandleFunc("GET /api/v1/dispatch/tasks/{id}/diff", s.handleGetDispatchedTaskDiff)
	s.mux.HandleFunc("GET /api/dispatch/tasks/{id}/diff", s.handleGetDispatchedTaskDiff)
	s.mux.HandleFunc("GET /api/v1/dispatch/tasks/{id}/files", s.handleGetDispatchedTaskFiles)
	s.mux.HandleFunc("GET /api/dispatch/tasks/{id}/files", s.handleGetDispatchedTaskFiles)
	s.mux.HandleFunc("GET /api/v1/dispatch/dashboard", s.handleGetDispatchedDashboard)
	s.mux.HandleFunc("GET /api/dispatch/dashboard", s.handleGetDispatchedDashboard)

	// Workspaces & Active Repositories
	s.mux.HandleFunc("GET /api/v1/workspaces", s.handleListWorkspaces)
	s.mux.HandleFunc("GET /api/workspaces", s.handleListWorkspaces)
	s.mux.HandleFunc("GET /api/v1/workspaces/active", s.handleActiveWorkspaces)
	s.mux.HandleFunc("GET /api/workspaces/active", s.handleActiveWorkspaces)
	s.mux.HandleFunc("GET /api/v1/profiles/{name}/workspaces", s.handleProfileWorkspaces)
	s.mux.HandleFunc("GET /api/profiles/{name}/workspaces", s.handleProfileWorkspaces)
	s.mux.HandleFunc("GET /api/v1/profiles/{name}/workspaces/active", s.handleProfileActiveWorkspace)
	s.mux.HandleFunc("GET /api/profiles/{name}/workspaces/active", s.handleProfileActiveWorkspace)

	// Headless Language Server & Agent Manager
	s.mux.HandleFunc("GET /api/v1/headless", s.handleListHeadlessInstances)
	s.mux.HandleFunc("GET /api/headless", s.handleListHeadlessInstances)
	s.mux.HandleFunc("GET /api/v1/headless/{profile}", s.handleGetHeadlessStatus)
	s.mux.HandleFunc("GET /api/headless/{profile}", s.handleGetHeadlessStatus)
	s.mux.HandleFunc("POST /api/v1/headless/{profile}/start", s.handleStartHeadless)
	s.mux.HandleFunc("POST /api/headless/{profile}/start", s.handleStartHeadless)
	s.mux.HandleFunc("POST /api/v1/headless/{profile}/stop", s.handleStopHeadless)
	s.mux.HandleFunc("POST /api/headless/{profile}/stop", s.handleStopHeadless)
	s.mux.HandleFunc("POST /api/v1/headless/{profile}/restart", s.handleRestartHeadless)
	s.mux.HandleFunc("POST /api/headless/{profile}/restart", s.handleRestartHeadless)
	s.mux.HandleFunc("POST /api/v1/headless/{profile}/run", s.handleRunHeadlessPrompt)
	s.mux.HandleFunc("POST /api/headless/{profile}/run", s.handleRunHeadlessPrompt)
	s.mux.HandleFunc("GET /api/v1/headless/{profile}/logs", s.handleGetHeadlessLogs)
	s.mux.HandleFunc("GET /api/headless/{profile}/logs", s.handleGetHeadlessLogs)
	s.mux.HandleFunc("POST /api/v1/exec", s.handleExecPrompts)
	s.mux.HandleFunc("POST /api/exec", s.handleExecPrompts)

	// Web UI Visualizer (HTML for Tauri/Wails/Browser)
	s.mux.HandleFunc("GET /ui/tasks", s.handleUITasksDashboard)
	s.mux.HandleFunc("GET /ui/dispatch/tasks", s.handleUITasksDashboard)
	s.mux.HandleFunc("GET /ui/tasks/{id}/diff", s.handleUITaskDiff)
	s.mux.HandleFunc("GET /ui/dispatch/tasks/{id}/diff", s.handleUITaskDiff)
}

func (s *Server) handleGatewayChatCompletions(w http.ResponseWriter, r *http.Request) {
	if s.gateway != nil {
		s.gateway.HandleChatCompletions(w, r)
		return
	}
	s.writeError(w, http.StatusServiceUnavailable, "gateway is not initialized")
}

func (s *Server) handleGatewayMessages(w http.ResponseWriter, r *http.Request) {
	if s.gateway != nil {
		s.gateway.HandleMessages(w, r)
		return
	}
	s.writeError(w, http.StatusServiceUnavailable, "gateway is not initialized")
}

func (s *Server) handleGatewayModels(w http.ResponseWriter, r *http.Request) {
	if s.gateway != nil {
		s.gateway.HandleModels(w, r)
		return
	}
	s.writeError(w, http.StatusServiceUnavailable, "gateway is not initialized")
}

func (s *Server) handleGatewayRouterStatus(w http.ResponseWriter, r *http.Request) {
	if s.gateway != nil {
		s.gateway.HandleRouterStatus(w, r)
		return
	}
	s.writeError(w, http.StatusServiceUnavailable, "gateway is not initialized")
}

func (s *Server) handleGatewayRouterReset(w http.ResponseWriter, r *http.Request) {
	if s.gateway != nil {
		s.gateway.HandleRouterReset(w, r)
		return
	}
	s.writeError(w, http.StatusServiceUnavailable, "gateway is not initialized")
}

func (s *Server) handleGatewayRouterStrategy(w http.ResponseWriter, r *http.Request) {
	if s.gateway != nil {
		s.gateway.HandleRouterStrategy(w, r)
		return
	}
	s.writeError(w, http.StatusServiceUnavailable, "gateway is not initialized")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := int64(time.Since(s.cfg.StartTime).Seconds())
	version := s.cfg.Version
	if version == "" {
		version = "dev"
	}
	s.writeJSON(w, http.StatusOK, HealthResponse{
		Status:        "ok",
		Version:       version,
		UptimeSeconds: uptime,
	})
}

func (s *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	rep, err := doctor.Diagnose()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, rep)
}

func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := profile.GetProfiles()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if profiles == nil {
		profiles = []profile.ProfileInfo{}
	}
	s.writeJSON(w, http.StatusOK, profiles)
}

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	info, err := profile.GetProfile(name)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleGetProfileStats(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	st, err := profile.GetSingleProfileStat(name)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, totalSize, err := profile.GetProfileStats()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if stats == nil {
		stats = []profile.ProfileStat{}
	}
	s.writeJSON(w, http.StatusOK, profile.ProfileStatsReport{
		Profiles:  stats,
		TotalSize: totalSize,
	})
}

func (s *Server) handleGetAllSharing(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	statuses, err := profile.GetAllSharingStatus(name)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, statuses)
}

func (s *Server) handleGetResourceSharing(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	resource := r.PathValue("resource")

	var status *profile.SharingStatus
	var err error

	switch strings.ToLower(resource) {
	case "mcp":
		status, err = profile.GetMcpStatus(name)
	case "skills":
		status, err = profile.GetSkillsStatus(name)
	case "config":
		status, err = profile.GetConfigStatus(name)
	case "gh", "github":
		status, err = profile.GetGhStatus(name)
	case "git", "dotfiles":
		status, err = profile.GetGitStatus(name)
	default:
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid resource %q: must be mcp, skills, config, gh, or git", resource))
		return
	}

	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleSetResourceSharing(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	resource := r.PathValue("resource")

	if !profile.ProfileExists(name) {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("profile %q does not exist", name))
		return
	}

	var req SetSharingRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON payload: %v", err))
			return
		}
	}

	action := req.Action
	if action == "" && req.Mode != "" {
		action = req.Mode
	}
	if action == "" && req.Shared != nil {
		if *req.Shared {
			action = "share"
		} else {
			action = "isolate"
		}
	}
	if action == "" {
		s.writeError(w, http.StatusBadRequest, "action, mode, or shared boolean is required (e.g. {\"action\": \"share\"|\"isolate\"|\"toggle\"})")
		return
	}

	newStatus, messages, err := profile.SetResourceSharing(name, resource, action)
	if err != nil {
		if strings.Contains(err.Error(), "invalid resource") || strings.Contains(err.Error(), "invalid action") {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if strings.Contains(err.Error(), "does not exist") {
			s.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":   "sharing",
				"profile":  name,
				"resource": newStatus.Resource,
				"mode":     newStatus.Mode,
				"status":   "updated",
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"profile":  name,
		"resource": newStatus.Resource,
		"mode":     newStatus.Mode,
		"status":   newStatus,
		"messages": messages,
	})
}

func (s *Server) handleBatchSharing(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !profile.ProfileExists(name) {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("profile %q does not exist", name))
		return
	}

	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON payload: %v", err))
		return
	}

	// Try decoding as array of SetSharingRequest
	var items []SetSharingRequest
	if err := json.Unmarshal(raw, &items); err == nil && len(items) > 0 {
		var results []map[string]any
		for _, item := range items {
			if item.Resource == "" {
				s.writeError(w, http.StatusBadRequest, "each item in list must specify 'resource'")
				return
			}
			act := item.Action
			if act == "" && item.Mode != "" {
				act = item.Mode
			}
			if act == "" && item.Shared != nil {
				if *item.Shared {
					act = "share"
				} else {
					act = "isolate"
				}
			}
			if act == "" {
				s.writeError(w, http.StatusBadRequest, fmt.Sprintf("resource %q missing action/mode/shared", item.Resource))
				return
			}
			st, msgs, err := profile.SetResourceSharing(name, item.Resource, act)
			if err != nil {
				s.writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			results = append(results, map[string]any{
				"resource": st.Resource,
				"mode":     st.Mode,
				"status":   st,
				"messages": msgs,
			})
		}
		if s.broker != nil {
			s.broker.Broadcast(SSEEvent{
				Event: "action",
				Data: map[string]any{
					"action":  "sharing",
					"profile": name,
					"status":  "updated",
					"batch":   true,
				},
				Time: time.Now().UTC().Format(time.RFC3339),
			})
		}
		s.writeJSON(w, http.StatusOK, results)
		return
	}

	// Try decoding as map[string]any
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		targetMap := obj
		if resMap, ok := obj["resources"].(map[string]any); ok {
			targetMap = resMap
		}
		var results []map[string]any
		for k, v := range targetMap {
			if k == "resources" {
				continue
			}
			var act string
			switch val := v.(type) {
			case string:
				act = val
			case bool:
				if val {
					act = "share"
				} else {
					act = "isolate"
				}
			case map[string]any:
				if a, ok := val["action"].(string); ok {
					act = a
				} else if m, ok := val["mode"].(string); ok {
					act = m
				} else if sh, ok := val["shared"].(bool); ok {
					if sh {
						act = "share"
					} else {
						act = "isolate"
					}
				}
			default:
				s.writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported value type for resource %q", k))
				return
			}

			st, msgs, err := profile.SetResourceSharing(name, k, act)
			if err != nil {
				s.writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			results = append(results, map[string]any{
				"resource": st.Resource,
				"mode":     st.Mode,
				"status":   st,
				"messages": msgs,
			})
		}
		if s.broker != nil {
			s.broker.Broadcast(SSEEvent{
				Event: "action",
				Data: map[string]any{
					"action":  "sharing",
					"profile": name,
					"status":  "updated",
					"batch":   true,
				},
				Time: time.Now().UTC().Format(time.RFC3339),
			})
		}
		s.writeJSON(w, http.StatusOK, results)
		return
	}

	s.writeError(w, http.StatusBadRequest, "invalid batch sharing payload: must be object or array")
}

func (s *Server) handleSeedConfig(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !profile.ProfileExists(name) {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("profile %q does not exist", name))
		return
	}

	msgs, err := profile.ConfigSeed(name)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	status, _ := profile.GetConfigStatus(name)

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":    "sharing",
				"profile":   name,
				"resource":  "config",
				"subaction": "seed",
				"status":    "seeded",
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"profile":  name,
		"resource": "config",
		"status":   status,
		"messages": msgs,
	})
}

func (s *Server) handleGetConversations(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	rawFilter := r.URL.Query().Get("filter")
	filter := chat.FilterAll
	switch rawFilter {
	case "active":
		filter = chat.FilterActive
	case "archived":
		filter = chat.FilterArchived
	case "in_use", "open":
		filter = chat.FilterInUse
	}
	convs, err := chat.GetFilteredConversations(name, filter)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if convs == nil {
		convs = []chat.ConversationInfo{}
	}
	s.writeJSON(w, http.StatusOK, convs)
}

func (s *Server) handleQuota(w http.ResponseWriter, r *http.Request) {
	servers, err := quota.FindActiveServers("")
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if servers == nil {
		servers = []quota.ActiveServer{}
	}
	quota.RecordLiveSnapshots(servers)
	s.writeJSON(w, http.StatusOK, servers)
}

func (s *Server) handleQuotaProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("profile")
	servers, err := quota.FindActiveServers(name)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if servers == nil {
		servers = []quota.ActiveServer{}
	}
	quota.RecordLiveSnapshots(servers)
	s.writeJSON(w, http.StatusOK, servers)
}

func (s *Server) handleQuotaHistory(w http.ResponseWriter, r *http.Request) {
	q, err := quota.ParseHistoryQuery(
		r.URL.Query().Get("since"),
		r.URL.Query().Get("until"),
		r.URL.Query().Get("source"),
		r.URL.Query().Get("limit"),
	)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	fleet, err := quota.LoadFleetSeries(q)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, fleet)
}

func (s *Server) handleQuotaProfileHistory(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("profile")
	if err := config.ValidateProfileName(name); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !profile.ProfileExists(name) {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("profile %q does not exist", name))
		return
	}
	q, err := quota.ParseHistoryQuery(
		r.URL.Query().Get("since"),
		r.URL.Query().Get("until"),
		r.URL.Query().Get("source"),
		r.URL.Query().Get("limit"),
	)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	series, err := quota.LoadSeries(name, q)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, series)
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	report, err := alert.Evaluate("")
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleAlertsProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("profile")
	report, err := alert.Evaluate(name)
	if err != nil {
		if !profile.ProfileExists(name) {
			s.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleGetProfilePrime(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !profile.ProfileExists(name) {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("profile %q does not exist", name))
		return
	}

	st, err := prime.GetProfilePrimeStatus(name)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleGetAllPrime(w http.ResponseWriter, r *http.Request) {
	statuses, err := prime.GetAllProfilesPrimeStatus()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if statuses == nil {
		statuses = []prime.ProfilePrimeStatus{}
	}
	s.writeJSON(w, http.StatusOK, statuses)
}

func (s *Server) handlePostProfilePrime(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !profile.ProfileExists(name) {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("profile %q does not exist", name))
		return
	}

	var req PrimeRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	q := r.URL.Query()
	if q.Get("force") == "true" {
		req.Force = true
	}
	if q.Get("check") == "true" {
		req.Check = true
	}
	if q.Get("5h") == "true" || q.Get("include_5h") == "true" {
		req.Include5h = true
	}
	if q.Get("warm_5h") == "true" || q.Get("warm-5h") == "true" {
		req.Warm5h = true
	}
	if q.Get("no_jitter") == "true" {
		req.NoJitter = true
	}

	opts := prime.PrimeOptions{
		Profile:   name,
		Force:     req.Force,
		Check:     req.Check,
		Include5h: req.Include5h || req.Include5hAlt,
		Warm5h:    req.Warm5h || req.Warm5hAlt,
		NoJitter:  req.NoJitter,
		MaxJitter: req.MaxJitter,
		Quiet:     true,
	}

	progressCb := func(ev prime.PrimeProgressEvent) {
		if s.broker != nil {
			s.broker.Broadcast(SSEEvent{
				Event: "prime",
				Data:  ev,
				Time:  time.Now().UTC().Format(time.RFC3339),
			})
		}
	}

	res, err := prime.ExecutePrime(opts, progressCb)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":  "prime",
				"profile": name,
				"status":  "completed",
				"results": res.Buckets,
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
	}

	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handlePostAllPrime(w http.ResponseWriter, r *http.Request) {
	var req BatchPrimeRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	q := r.URL.Query()
	if q.Get("force") == "true" {
		req.Force = true
	}
	if q.Get("check") == "true" {
		req.Check = true
	}
	if q.Get("5h") == "true" || q.Get("include_5h") == "true" {
		req.Include5h = true
	}
	if q.Get("warm_5h") == "true" || q.Get("warm-5h") == "true" {
		req.Warm5h = true
	}
	if q.Get("no_jitter") == "true" {
		req.NoJitter = true
	}

	targetProfiles := req.Profiles
	if len(targetProfiles) == 0 {
		var err error
		targetProfiles, err = profile.ListProfiles()
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	if len(targetProfiles) == 0 {
		targetProfiles = []string{"default"}
	}

	results := make([]prime.ProfilePrimeResult, 0, len(targetProfiles))

	progressCb := func(ev prime.PrimeProgressEvent) {
		if s.broker != nil {
			s.broker.Broadcast(SSEEvent{
				Event: "prime",
				Data:  ev,
				Time:  time.Now().UTC().Format(time.RFC3339),
			})
		}
	}

	for _, p := range targetProfiles {
		opts := prime.PrimeOptions{
			Profile:   p,
			Force:     req.Force,
			Check:     req.Check,
			Include5h: req.Include5h || req.Include5hAlt,
			Warm5h:    req.Warm5h || req.Warm5hAlt,
			NoJitter:  req.NoJitter,
			MaxJitter: req.MaxJitter,
			Quiet:     true,
		}

		res, err := prime.ExecutePrime(opts, progressCb)
		if err != nil {
			results = append(results, prime.ProfilePrimeResult{
				Profile: p,
				Error:   err.Error(),
			})
			continue
		}
		results = append(results, *res)
	}

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action": "prime",
				"status": "completed",
				"batch":  true,
				"count":  len(results),
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
	}

	s.writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleStopProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req StopRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if !profile.ProfileExists(name) {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("profile %q does not exist", name))
		return
	}

	if !profile.IsProfileRunning(name) {
		s.writeJSON(w, http.StatusOK, map[string]any{
			"profile": name,
			"status":  "not_running",
			"message": fmt.Sprintf("profile %q is not running", name),
		})
		return
	}

	if err := profile.StopProfile(name, req.Force); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":  "stop",
				"profile": name,
				"status":  "stopped",
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
		s.broker.CheckProfilesChange()
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"profile": name,
		"status":  "stopped",
		"message": fmt.Sprintf("profile %q stopped successfully", name),
	})
}

func (s *Server) handleCleanProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !profile.ProfileExists(name) {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("profile %q does not exist", name))
		return
	}

	before, after, err := profile.CleanSingleProfile(name)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":      "clean",
				"profile":     name,
				"status":      "cleaned",
				"size_before": before,
				"size_after":  after,
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
		s.broker.CheckProfilesChange()
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"profile":     name,
		"status":      "cleaned",
		"size_before": before,
		"size_after":  after,
	})
}

func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	var req CreateProfileRequest
	if r.Body == nil {
		s.writeError(w, http.StatusBadRequest, "request body is required")
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON payload: %v", err))
		return
	}

	if req.Name == "" {
		s.writeError(w, http.StatusBadRequest, "profile name is required")
		return
	}

	if err := config.ValidateProfileName(req.Name); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if profile.ProfileExists(req.Name) {
		s.writeError(w, http.StatusConflict, fmt.Sprintf("profile %q already exists", req.Name))
		return
	}

	isAuthOnly := req.AuthOnly || req.AuthOnlyKebab || req.Shared

	opts := profile.CreateOptions{
		Name:             req.Name,
		AuthOnly:         isAuthOnly,
		Shared:           isAuthOnly,
		IsolatedDotfiles: req.IsolatedDotfiles,
		IsolatedMCP:      req.IsolatedMCP,
		IsolatedSkills:   req.IsolatedSkills,
		IsolatedConfig:   req.IsolatedConfig,
		IsolatedGH:       req.IsolatedGH,
		Color:            req.Color,
		FromTemplate:     req.FromTemplate,
	}

	if err := profile.CreateProfile(opts); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			s.writeError(w, http.StatusConflict, err.Error())
			return
		}
		if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "template") {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":    "create",
				"profile":   req.Name,
				"status":    "created",
				"auth_only": isAuthOnly,
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
		s.broker.CheckProfilesChange()
	}

	info, err := profile.GetProfile(req.Name)
	if err != nil {
		s.writeJSON(w, http.StatusCreated, map[string]any{
			"name":      req.Name,
			"status":    "created",
			"auth_only": isAuthOnly,
		})
		return
	}

	s.writeJSON(w, http.StatusCreated, info)
}

func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !profile.ProfileExists(name) {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("profile %q does not exist", name))
		return
	}

	force := r.URL.Query().Get("force") == "true" || r.URL.Query().Get("force") == "1"
	if r.Body != nil && r.ContentLength > 0 {
		var req DeleteProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Force {
			force = true
		}
	}

	if profile.IsProfileRunning(name) && !force {
		s.writeError(w, http.StatusConflict, fmt.Sprintf("cannot delete profile %q because it is currently running (use force=true to stop and delete)", name))
		return
	}

	if err := profile.DeleteProfile(name, force); err != nil {
		if strings.Contains(err.Error(), "currently running") {
			s.writeError(w, http.StatusConflict, err.Error())
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":  "delete",
				"profile": name,
				"status":  "deleted",
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
		s.broker.CheckProfilesChange()
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"profile": name,
		"status":  "deleted",
		"message": fmt.Sprintf("profile %q deleted successfully", name),
	})
}

func (s *Server) handleLaunchProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !profile.ProfileExists(name) {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("profile %q does not exist", name))
		return
	}

	var req LaunchProfileRequest
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if err := profile.LaunchProfile(name, req.Args); err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to launch profile %q: %v", name, err))
		return
	}

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":  "launch",
				"profile": name,
				"status":  "launched",
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
		s.broker.CheckProfilesChange()
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"profile": name,
		"status":  "launched",
		"message": fmt.Sprintf("profile %q launched successfully", name),
	})
}

func (s *Server) handleRestartProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !profile.ProfileExists(name) {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("profile %q does not exist", name))
		return
	}

	var req LaunchProfileRequest
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if err := profile.RestartProfile(name, req.Args); err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to restart profile %q: %v", name, err))
		return
	}

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":  "restart",
				"profile": name,
				"status":  "restarted",
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
		s.broker.CheckProfilesChange()
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"profile": name,
		"status":  "restarted",
		"message": fmt.Sprintf("profile %q restarted successfully", name),
	})
}

func (s *Server) handleRenameProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !profile.ProfileExists(name) {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("profile %q does not exist", name))
		return
	}

	var req RenameProfileRequest
	if r.Body == nil {
		s.writeError(w, http.StatusBadRequest, "request body is required")
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON payload: %v", err))
		return
	}

	if req.NewName == "" {
		s.writeError(w, http.StatusBadRequest, "new profile name is required")
		return
	}

	if err := config.ValidateProfileName(req.NewName); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if profile.ProfileExists(req.NewName) {
		s.writeError(w, http.StatusConflict, fmt.Sprintf("profile %q already exists", req.NewName))
		return
	}

	if profile.IsProfileRunning(name) {
		s.writeError(w, http.StatusConflict, fmt.Sprintf("cannot rename profile %q because it is currently running", name))
		return
	}

	if err := profile.RenameProfile(name, req.NewName); err != nil {
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "currently running") {
			s.writeError(w, http.StatusConflict, err.Error())
			return
		}
		if strings.Contains(err.Error(), "invalid") {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":   "rename",
				"old_name": name,
				"new_name": req.NewName,
				"status":   "renamed",
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
		s.broker.CheckProfilesChange()
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"old_name": name,
		"new_name": req.NewName,
		"status":   "renamed",
		"message":  fmt.Sprintf("profile %q renamed to %q successfully", name, req.NewName),
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	profiles, _ := profile.GetProfiles()
	if profiles == nil {
		profiles = []profile.ProfileInfo{}
	}

	initData := map[string]any{
		"profiles":  profiles,
		"version":   s.cfg.Version,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	initBytes, err := json.Marshal(initData)
	if err == nil {
		fmt.Fprintf(w, "event: init\ndata: %s\n\n", string(initBytes))
		flusher.Flush()
	}

	clientChan := make(chan SSEEvent, 32)
	s.broker.Register(clientChan)
	defer s.broker.Unregister(clientChan)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-clientChan:
			if !ok {
				return
			}
			payload, err := json.Marshal(ev.Data)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Event, string(payload))
			flusher.Flush()
		}
	}
}

// Worktree Handlers

func (s *Server) handleListWorktrees(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	list, err := worktree.ListWorktrees(repo)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if list == nil {
		list = []worktree.Worktree{}
	}
	s.writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateWorktree(w http.ResponseWriter, r *http.Request) {
	var req CreateWorktreeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json body: %v", err))
		return
	}

	opts := worktree.CreateOptions{
		ID:         req.ID,
		RepoPath:   req.RepoPath,
		Branch:     req.Branch,
		BaseCommit: req.BaseCommit,
		TargetDir:  req.TargetDir,
		Profile:    req.Profile,
		TaskID:     req.TaskID,
		Metadata:   req.Metadata,
		CopyFiles:  req.CopyFiles,
		LinkPaths:  req.LinkPaths,
	}

	wt, err := worktree.CreateWorktree(opts)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.broker.Broadcast(SSEEvent{
		Event: "action",
		Data: map[string]any{
			"action":    "worktree",
			"type":      "create",
			"id":        wt.ID,
			"path":      wt.Path,
			"branch":    wt.Branch,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	})

	s.writeJSON(w, http.StatusCreated, wt)
}

func (s *Server) handleGetWorktree(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repo := r.URL.Query().Get("repo")

	wt, err := worktree.GetWorktree(repo, id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, wt)
}

func (s *Server) handleDeleteWorktree(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repo := r.URL.Query().Get("repo")
	force := r.URL.Query().Get("force") == "true"
	deleteBranch := r.URL.Query().Get("delete_branch") == "true" || r.URL.Query().Get("delete-branch") == "true"

	if r.Header.Get("Content-Type") == "application/json" && r.ContentLength > 0 {
		var req DeleteWorktreeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if req.Repo != "" {
				repo = req.Repo
			}
			if req.Force {
				force = true
			}
			if req.DeleteBranch {
				deleteBranch = true
			}
		}
	}

	opts := worktree.RemoveOptions{
		Force:        force,
		DeleteBranch: deleteBranch,
	}

	if err := worktree.RemoveWorktree(repo, id, opts); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.broker.Broadcast(SSEEvent{
		Event: "action",
		Data: map[string]any{
			"action":    "worktree",
			"type":      "delete",
			"id":        id,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	})

	s.writeJSON(w, http.StatusOK, map[string]any{
		"id":      id,
		"removed": true,
	})
}

func (s *Server) handleGetWorktreeStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repo := r.URL.Query().Get("repo")

	status, err := worktree.GetWorktreeStatus(repo, id)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleGetWorktreeDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repo := r.URL.Query().Get("repo")
	stat := r.URL.Query().Get("stat") == "true"
	cached := r.URL.Query().Get("cached") == "true"
	base := r.URL.Query().Get("base")

	opts := worktree.DiffOptions{
		Base:     base,
		StatOnly: stat,
		Cached:   cached,
	}

	diff, err := worktree.GetWorktreeDiff(repo, id, opts)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{
		"diff": diff,
	})
}

func (s *Server) handlePruneWorktrees(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	if err := worktree.PruneWorktrees(repo); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.broker.Broadcast(SSEEvent{
		Event: "action",
		Data: map[string]any{
			"action":    "worktree",
			"type":      "prune",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	})

	s.writeJSON(w, http.StatusOK, map[string]any{
		"pruned": true,
	})
}

// --- Agent PTY & Headless Runner Handlers ---

func (s *Server) handleListAgentSessions(w http.ResponseWriter, r *http.Request) {
	filter := agent.SessionFilter{
		Profile:   r.URL.Query().Get("profile"),
		Status:    agent.SessionStatus(r.URL.Query().Get("status")),
		AgentType: r.URL.Query().Get("agent_type"),
	}
	if filter.AgentType == "" {
		filter.AgentType = r.URL.Query().Get("type")
	}

	sessions := agent.GetDefaultManager().ListSessions(filter)
	if sessions == nil {
		sessions = []agent.Session{}
	}
	s.writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) handleCreateAgentSession(w http.ResponseWriter, r *http.Request) {
	var opts agent.CreateSessionOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request payload: %v", err))
		return
	}

	if opts.GatewayURL == "" {
		opts.GatewayURL = fmt.Sprintf("http://%s/v1", r.Host)
	}

	inst, err := agent.GetDefaultManager().StartSession(opts)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	info := inst.GetInfo()

	s.broker.Broadcast(SSEEvent{
		Event: "action",
		Data: map[string]any{
			"action":     "agent_session",
			"type":       "create",
			"id":         info.ID,
			"profile":    info.Profile,
			"agent_type": info.AgentType,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		},
	})

	s.writeJSON(w, http.StatusCreated, info)
}

func (s *Server) handlePruneAgentSessions(w http.ResponseWriter, r *http.Request) {
	maxAge := 1 * time.Hour
	if q := r.URL.Query().Get("max_age"); q != "" {
		if d, err := time.ParseDuration(q); err == nil {
			maxAge = d
		}
	}

	pruned := agent.GetDefaultManager().PruneSessions(maxAge)

	s.broker.Broadcast(SSEEvent{
		Event: "action",
		Data: map[string]any{
			"action":    "agent_session",
			"type":      "prune",
			"pruned":    pruned,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	})

	s.writeJSON(w, http.StatusOK, map[string]any{
		"pruned": pruned,
	})
}

func (s *Server) handleGetAgentSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	inst, err := agent.GetDefaultManager().GetSession(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, inst.GetInfo())
}

func (s *Server) handleDeleteAgentSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	force := r.URL.Query().Get("force") == "true"

	var err error
	if force {
		err = agent.GetDefaultManager().KillSession(id)
	} else {
		err = agent.GetDefaultManager().StopSession(id)
	}

	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.broker.Broadcast(SSEEvent{
		Event: "action",
		Data: map[string]any{
			"action":    "agent_session",
			"type":      "delete",
			"id":        id,
			"force":     force,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	})

	s.writeJSON(w, http.StatusOK, map[string]any{
		"id":      id,
		"stopped": true,
	})
}

func (s *Server) handleGetAgentSessionOutput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tail := 0
	if q := r.URL.Query().Get("tail"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			tail = n * 128
		}
	}

	out, err := agent.GetDefaultManager().GetSessionOutput(id, tail)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"id":     id,
		"output": string(out),
	})
}

func (s *Server) handleStreamAgentSessionOutput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	ch, unsub, err := agent.GetDefaultManager().SubscribeSession(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	defer unsub()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-ch:
			if !ok {
				fmt.Fprintf(w, "event: done\ndata: {}\n\n")
				flusher.Flush()
				return
			}
			chunkBytes, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "event: output\ndata: %s\n\n", chunkBytes)
			flusher.Flush()
		}
	}
}

func (s *Server) handleSendAgentSessionInput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req agent.SendInputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid input payload: %v", err))
		return
	}

	if err := agent.GetDefaultManager().WriteSessionInput(id, []byte(req.Data)); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"id":            id,
		"bytes_written": len(req.Data),
	})
}

func (s *Server) handleResizeAgentSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req agent.ResizeOptions
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid resize payload: %v", err))
		return
	}

	if err := agent.GetDefaultManager().ResizeSession(id, req.Rows, req.Cols); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"id":   id,
		"rows": req.Rows,
		"cols": req.Cols,
	})
}

// --- Task Dispatcher Handlers ---

func (s *Server) handleListDispatchedTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	repoPath := q.Get("repo")
	filter := dispatch.TaskFilter{
		Profile:    q.Get("profile"),
		Status:     dispatch.TaskStatus(q.Get("status")),
		AgentType:  q.Get("agent"),
		WorktreeID: q.Get("worktree"),
	}

	tasks, err := dispatch.GetDefaultTaskManager().ListTasks(repoPath, filter)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleCreateDispatchedTask(w http.ResponseWriter, r *http.Request) {
	var opts dispatch.DispatchOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid task payload: %v", err))
		return
	}

	if opts.Profile == "" {
		s.writeError(w, http.StatusBadRequest, "profile is required")
		return
	}

	task, err := dispatch.GetDefaultTaskManager().Dispatch(opts)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.broker.Broadcast(SSEEvent{
		Event: "action",
		Time:  time.Now().UTC().Format(time.RFC3339),
		Data: map[string]any{
			"action":  "dispatch",
			"status":  "dispatched",
			"task_id": task.ID,
			"profile": task.Profile,
		},
	})

	s.writeJSON(w, http.StatusCreated, task)
}

func (s *Server) handleGetDispatchedTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repoPath := r.URL.Query().Get("repo")

	task, err := dispatch.GetDefaultTaskManager().GetTask(repoPath, id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleDeleteDispatchedTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	q := r.URL.Query()
	repoPath := q.Get("repo")
	removeWT := q.Get("worktree") == "true"

	if err := dispatch.GetDefaultTaskManager().DeleteTask(repoPath, id, removeWT); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.broker.Broadcast(SSEEvent{
		Event: "action",
		Time:  time.Now().UTC().Format(time.RFC3339),
		Data: map[string]any{
			"action":  "dispatch",
			"status":  "deleted",
			"task_id": id,
		},
	})

	s.writeJSON(w, http.StatusOK, map[string]any{
		"id":      id,
		"deleted": true,
	})
}

func (s *Server) handleCancelDispatchedTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	q := r.URL.Query()
	repoPath := q.Get("repo")
	force := q.Get("force") == "true"

	if err := dispatch.GetDefaultTaskManager().CancelTask(repoPath, id, force); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.broker.Broadcast(SSEEvent{
		Event: "action",
		Time:  time.Now().UTC().Format(time.RFC3339),
		Data: map[string]any{
			"action":  "dispatch",
			"status":  "cancelled",
			"task_id": id,
		},
	})

	s.writeJSON(w, http.StatusOK, map[string]any{
		"id":     id,
		"status": "cancelled",
	})
}

func (s *Server) handleGetDispatchedTaskLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	q := r.URL.Query()
	repoPath := q.Get("repo")
	tailBytes := 0
	if tailStr := q.Get("tail"); tailStr != "" {
		if n, err := strconv.Atoi(tailStr); err == nil && n > 0 {
			tailBytes = n
		}
	}

	data, err := dispatch.GetDefaultTaskManager().GetTaskLogs(repoPath, id, tailBytes)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"id":   id,
		"logs": string(data),
	})
}

func (s *Server) handleStreamDispatchedTaskLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repoPath := r.URL.Query().Get("repo")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	task, err := dispatch.GetDefaultTaskManager().GetTask(repoPath, id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()

	// If session is active, subscribe to live chunks
	if task.Status == dispatch.StatusRunning && task.SessionID != "" {
		ch, unsub, err := agent.GetDefaultManager().SubscribeSession(task.SessionID)
		if err == nil {
			defer unsub()
			for {
				select {
				case <-ctx.Done():
					return
				case chunk, ok := <-ch:
					if !ok {
						fmt.Fprintf(w, "event: done\ndata: {}\n\n")
						flusher.Flush()
						return
					}
					chunkBytes, _ := json.Marshal(chunk)
					fmt.Fprintf(w, "event: output\ndata: %s\n\n", chunkBytes)
					flusher.Flush()
				}
			}
		}
	}

	// Completed or archived task: stream existing log buffer
	if logData, err := dispatch.GetDefaultTaskManager().GetTaskLogs(repoPath, id, 0); err == nil && len(logData) > 0 {
		chunk := agent.OutputChunk{
			SessionID: task.SessionID,
			Data:      string(logData),
			Offset:    0,
			Timestamp: time.Now(),
		}
		chunkBytes, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "event: output\ndata: %s\n\n", chunkBytes)
		flusher.Flush()
	}

	fmt.Fprintf(w, "event: done\ndata: {}\n\n")
	flusher.Flush()
}

func (s *Server) handleGetDispatchedTaskDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	q := r.URL.Query()
	repoPath := q.Get("repo")
	statOnly := q.Get("stat") == "true"
	isStructured := q.Get("format") == "structured" || q.Get("structured") == "true"

	task, err := dispatch.GetDefaultTaskManager().GetTask(repoPath, id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	diff, err := dispatch.GetDefaultTaskManager().GetTaskDiff(repoPath, id, statOnly)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp := map[string]any{
		"id":          id,
		"worktree_id": task.WorktreeID,
		"branch":      task.Branch,
		"base_commit": task.BaseCommit,
		"diff":        diff,
	}

	if isStructured {
		resp["structured"] = dispatch.ParseUnifiedDiff(diff)
	}

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetDispatchedTaskFiles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repoPath := r.URL.Query().Get("repo")

	files, err := dispatch.GetDefaultTaskManager().GetTaskFiles(repoPath, id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"id":    id,
		"files": files,
		"total": len(files),
	})
}

func (s *Server) handleGetDispatchedDashboard(w http.ResponseWriter, r *http.Request) {
	repoPath := r.URL.Query().Get("repo")
	summary, err := dispatch.GetDefaultTaskManager().GetDashboardSummary(repoPath)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleUITasksDashboard(w http.ResponseWriter, r *http.Request) {
	repoPath := r.URL.Query().Get("repo")
	summary, err := dispatch.GetDefaultTaskManager().GetDashboardSummary(repoPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load dashboard: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := ui.DashboardPageData{
		Summary:     summary,
		GeneratedAt: time.Now().UTC(),
	}
	if err := ui.RenderDashboard(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleUITaskDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repoPath := r.URL.Query().Get("repo")

	task, err := dispatch.GetDefaultTaskManager().GetTask(repoPath, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Task not found: %v", err), http.StatusNotFound)
		return
	}

	sd, err := dispatch.GetDefaultTaskManager().GetTaskStructuredDiff(repoPath, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to compute diff: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := ui.DiffPageData{
		Task:        *task,
		Structured:  sd,
		GeneratedAt: time.Now().UTC(),
	}
	if err := ui.RenderDiff(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handlePruneDispatchedTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	repoPath := q.Get("repo")
	maxAge := 24 * time.Hour
	if maxAgeStr := q.Get("max_age"); maxAgeStr != "" {
		if dur, err := time.ParseDuration(maxAgeStr); err == nil {
			maxAge = dur
		}
	}

	pruned, err := dispatch.GetDefaultTaskManager().PruneTasks(repoPath, maxAge)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"pruned": pruned,
	})
}

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	targetProfile := q.Get("profile")
	targetPath := q.Get("path")
	activeOnly := q.Get("active") == "true" || q.Get("active") == "1"

	var list []workspace.Workspace
	var err error

	if targetProfile != "" {
		list, err = workspace.GetProfileWorkspaces(targetProfile)
	} else if targetPath != "" {
		list, err = workspace.GetWorkspaceByPath(targetPath)
	} else {
		list, err = workspace.GetAllWorkspaces()
	}

	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if activeOnly {
		var filtered []workspace.Workspace
		for _, ws := range list {
			if ws.IsActive {
				filtered = append(filtered, ws)
			}
		}
		list = filtered
	}

	if list == nil {
		list = []workspace.Workspace{}
	}

	s.writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleActiveWorkspaces(w http.ResponseWriter, r *http.Request) {
	list, err := workspace.GetActiveWorkspaces()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []workspace.Workspace{}
	}
	s.writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleProfileWorkspaces(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "profile name is required")
		return
	}

	summary, err := workspace.GetProfileSummary(name)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleProfileActiveWorkspace(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "profile name is required")
		return
	}

	summary, err := workspace.GetProfileSummary(name)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if summary.ActiveWorkspace == nil {
		s.writeJSON(w, http.StatusOK, map[string]any{
			"profile":   name,
			"is_active": false,
			"workspace": nil,
		})
		return
	}

	s.writeJSON(w, http.StatusOK, summary.ActiveWorkspace)
}

// --- Headless Language Server & Agent Handlers ---

type startHeadlessRequest struct {
	Port         int    `json:"port"`
	Timeout      string `json:"timeout"`
	ForceRestart bool   `json:"force_restart"`
	Force        bool   `json:"force"`
}

type runHeadlessRequest struct {
	Prompt                     string `json:"prompt"`
	Timeout                    string `json:"timeout"`
	DangerouslySkipPermissions bool   `json:"dangerously_skip_permissions"`
}

func (s *Server) handleListHeadlessInstances(w http.ResponseWriter, r *http.Request) {
	mgr := headless.GetDefaultManager()
	list, err := mgr.List()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []*headless.InstanceInfo{}
	}
	s.writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetHeadlessStatus(w http.ResponseWriter, r *http.Request) {
	profileName := r.PathValue("profile")
	if profileName == "" {
		s.writeError(w, http.StatusBadRequest, "profile name is required")
		return
	}

	mgr := headless.GetDefaultManager()
	st, err := mgr.GetStatus(profileName)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleStartHeadless(w http.ResponseWriter, r *http.Request) {
	profileName := r.PathValue("profile")
	if profileName == "" {
		s.writeError(w, http.StatusBadRequest, "profile name is required")
		return
	}

	var req startHeadlessRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON payload: %v", err))
			return
		}
	}

	timeout := 10 * time.Second
	if req.Timeout != "" {
		if d, err := time.ParseDuration(req.Timeout); err == nil {
			timeout = d
		}
	}

	mgr := headless.GetDefaultManager()
	inst, err := mgr.Start(profileName, headless.StartOptions{
		Profile:      profileName,
		Port:         req.Port,
		Timeout:      timeout,
		ForceRestart: req.Force || req.ForceRestart,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":  "headless_start",
				"profile": profileName,
				"status":  string(inst.Status),
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
	}

	s.writeJSON(w, http.StatusOK, inst)
}

func (s *Server) handleStopHeadless(w http.ResponseWriter, r *http.Request) {
	profileName := r.PathValue("profile")
	if profileName == "" {
		s.writeError(w, http.StatusBadRequest, "profile name is required")
		return
	}

	mgr := headless.GetDefaultManager()
	if err := mgr.Stop(profileName); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":  "headless_stop",
				"profile": profileName,
				"status":  "stopped",
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"profile": profileName,
		"status":  "stopped",
	})
}

func (s *Server) handleRestartHeadless(w http.ResponseWriter, r *http.Request) {
	profileName := r.PathValue("profile")
	if profileName == "" {
		s.writeError(w, http.StatusBadRequest, "profile name is required")
		return
	}

	var req startHeadlessRequest
	if r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	timeout := 10 * time.Second
	if req.Timeout != "" {
		if d, err := time.ParseDuration(req.Timeout); err == nil {
			timeout = d
		}
	}

	mgr := headless.GetDefaultManager()
	inst, err := mgr.Restart(profileName, headless.StartOptions{
		Profile: profileName,
		Port:    req.Port,
		Timeout: timeout,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":  "headless_restart",
				"profile": profileName,
				"status":  string(inst.Status),
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
	}

	s.writeJSON(w, http.StatusOK, inst)
}

func (s *Server) handleRunHeadlessPrompt(w http.ResponseWriter, r *http.Request) {
	profileName := r.PathValue("profile")
	if profileName == "" {
		s.writeError(w, http.StatusBadRequest, "profile name is required")
		return
	}

	var req runHeadlessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON payload: %v", err))
		return
	}
	if req.Prompt == "" {
		s.writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	timeout := 120 * time.Second
	if req.Timeout != "" {
		if d, err := time.ParseDuration(req.Timeout); err == nil {
			timeout = d
		}
	}

	mgr := headless.GetDefaultManager()
	result, err := mgr.RunAgentPrompt(headless.AgentRunOptions{
		Profile:                    profileName,
		Prompt:                     req.Prompt,
		Timeout:                    timeout,
		DangerouslySkipPermissions: req.DangerouslySkipPermissions,
	})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, result)
}

type execPromptsRequest struct {
	Profile                    string   `json:"profile,omitempty"`
	Profiles                   []string `json:"profiles,omitempty"`
	All                        bool     `json:"all,omitempty"`
	Prompt                     string   `json:"prompt"`
	Timeout                    string   `json:"timeout,omitempty"`
	Workers                    int      `json:"workers,omitempty"`
	DangerouslySkipPermissions *bool    `json:"dangerously_skip_permissions,omitempty"`
}

func (s *Server) handleExecPrompts(w http.ResponseWriter, r *http.Request) {
	var req execPromptsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON payload: %v", err))
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		s.writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	timeout := 120 * time.Second
	if req.Timeout != "" {
		d, err := time.ParseDuration(req.Timeout)
		if err != nil || d <= 0 {
			s.writeError(w, http.StatusBadRequest, "timeout must be a positive duration")
			return
		}
		timeout = d
	}

	skip := true
	if req.DangerouslySkipPermissions != nil {
		skip = *req.DangerouslySkipPermissions
	}

	mgr := headless.GetDefaultManager()
	report, err := mgr.Exec(headless.ExecOptions{
		Profile:                    req.Profile,
		Profiles:                   req.Profiles,
		All:                        req.All,
		Prompt:                     req.Prompt,
		Timeout:                    timeout,
		Workers:                    req.Workers,
		DangerouslySkipPermissions: skip,
	})
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":    "exec",
				"succeeded": report.Succeeded,
				"failed":    report.Failed,
				"workers":   report.Workers,
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
	}

	s.writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleGetIDELogs(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tail := 100
	if tStr := r.URL.Query().Get("tail"); tStr != "" {
		if t, err := strconv.Atoi(tStr); err == nil && t >= 0 {
			tail = t
		}
	}
	follow := r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("follow") == "1"
	if !follow {
		snap, err := idelog.Read(name, tail)
		if err != nil {
			s.writeIDELogError(w, err)
			return
		}
		s.writeJSON(w, http.StatusOK, snap)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	ch, err := idelog.Follow(r.Context(), name, tail)
	if err != nil {
		s.writeIDELogError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	for chunk := range ch {
		payload, err := json.Marshal(map[string]string{"chunk": chunk})
		if err != nil {
			return
		}
		fmt.Fprintf(w, "event: log\ndata: %s\n\n", payload)
		flusher.Flush()
	}
}

func (s *Server) writeIDELogError(w http.ResponseWriter, err error) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "invalid"):
		s.writeError(w, http.StatusBadRequest, msg)
	case strings.Contains(msg, "does not exist"), strings.Contains(msg, "not found"):
		s.writeError(w, http.StatusNotFound, msg)
	default:
		s.writeError(w, http.StatusInternalServerError, msg)
	}
}

func (s *Server) handleGetHeadlessLogs(w http.ResponseWriter, r *http.Request) {
	profileName := r.PathValue("profile")
	if profileName == "" {
		s.writeError(w, http.StatusBadRequest, "profile name is required")
		return
	}

	tail := 50
	if tStr := r.URL.Query().Get("tail"); tStr != "" {
		if t, err := strconv.Atoi(tStr); err == nil && t > 0 {
			tail = t
		}
	}

	mgr := headless.GetDefaultManager()
	logs, err := mgr.GetLogs(profileName, tail)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"profile": profileName,
		"logs":    logs,
	})
}

func (s *Server) writeSnapshotError(w http.ResponseWriter, err error) {
	var busy *profile.ProfileBusyError
	var missing *profile.SnapshotNotFoundError
	switch {
	case errors.As(err, &busy):
		s.writeError(w, http.StatusConflict, err.Error())
	case errors.As(err, &missing):
		s.writeError(w, http.StatusNotFound, err.Error())
	case strings.Contains(err.Error(), "does not exist"):
		s.writeError(w, http.StatusNotFound, err.Error())
	case strings.Contains(err.Error(), "invalid"):
		s.writeError(w, http.StatusBadRequest, err.Error())
	default:
		s.writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	snaps, err := profile.ListSnapshots(name)
	if err != nil {
		s.writeSnapshotError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, snaps)
}

func (s *Server) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Note string `json:"note"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}
	snap, err := profile.CreateSnapshot(name, req.Note)
	if err != nil {
		s.writeSnapshotError(w, err)
		return
	}
	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":  "snapshot",
				"profile": name,
				"id":      snap.ID,
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
	}
	s.writeJSON(w, http.StatusCreated, snap)
}

func (s *Server) handleRollbackSnapshot(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id := r.PathValue("id")
	snap, err := profile.RollbackSnapshot(name, id)
	if err != nil {
		s.writeSnapshotError(w, err)
		return
	}
	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Data: map[string]any{
				"action":  "rollback",
				"profile": name,
				"id":      snap.ID,
			},
			Time: time.Now().UTC().Format(time.RFC3339),
		})
	}
	s.writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id := r.PathValue("id")
	if err := profile.DeleteSnapshot(name, id); err != nil {
		s.writeSnapshotError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{
		"profile": name,
		"id":      id,
		"status":  "deleted",
	})
}
