package server

import (
	"encoding/json"
	"net/http"
)

// APIResponse standardizes the JSON response payload
type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
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
		w.Header().Set("Access-Control-Expose-Headers", "X-Profile-Used, X-Failover-Count, X-Remaining-Profiles, X-Routing-Strategy, X-Throttled, X-Throttle-Delay")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// register records one method+path on the mux. Patterns stay on the server so
// tests can assert the /api twin without scraping the mux tree.
func (s *Server) register(method, path string, handler http.HandlerFunc) {
	pattern := method + " " + path
	s.mux.HandleFunc(pattern, handler)
	s.patterns = append(s.patterns, pattern)
}

// registerPair records the same handler on /api/v1{suffix} and /api{suffix}.
// suffix starts with "/". Paths that are not that alias stay on register.
func (s *Server) registerPair(method, suffix string, handler http.HandlerFunc) {
	s.register(method, "/api/v1"+suffix, handler)
	s.register(method, "/api"+suffix, handler)
}

func (s *Server) setupRoutes() {
	s.register("GET", "/health", s.handleHealth)
	s.registerPair("GET", "/health", s.handleHealth)

	s.registerPair("GET", "/doctor", s.handleDoctor)
	s.registerPair("GET", "/catalog", s.handleCatalog)
	s.registerPair("GET", "/profiles/{name}/catalog", s.handleCatalog)

	s.registerPair("GET", "/profiles", s.handleListProfiles)
	s.registerPair("POST", "/profiles", s.handleCreateProfile)
	s.registerPair("GET", "/profiles/{name}", s.handleGetProfile)
	s.registerPair("DELETE", "/profiles/{name}", s.handleDeleteProfile)
	s.registerPair("GET", "/profiles/{name}/stats", s.handleGetProfileStats)
	s.registerPair("GET", "/profiles/{name}/ide/logs", s.handleGetIDELogs)
	s.registerPair("GET", "/stats", s.handleStats)

	s.registerPair("POST", "/profiles/{name}/launch", s.handleLaunchProfile)
	s.registerPair("POST", "/profiles/{name}/stop", s.handleStopProfile)
	s.registerPair("POST", "/profiles/{name}/restart", s.handleRestartProfile)
	s.registerPair("POST", "/profiles/{name}/clean", s.handleCleanProfile)
	s.registerPair("POST", "/profiles/{name}/rename", s.handleRenameProfile)

	s.registerPair("GET", "/profiles/{name}/snapshots", s.handleListSnapshots)
	s.registerPair("POST", "/profiles/{name}/snapshots", s.handleCreateSnapshot)
	s.registerPair("POST", "/profiles/{name}/snapshots/{id}/rollback", s.handleRollbackSnapshot)
	s.registerPair("DELETE", "/profiles/{name}/snapshots/{id}", s.handleDeleteSnapshot)

	s.registerPair("GET", "/profiles/{name}/sharing", s.handleGetAllSharing)
	s.registerPair("GET", "/profiles/{name}/sharing/{resource}", s.handleGetResourceSharing)
	s.registerPair("POST", "/profiles/{name}/sharing/{resource}", s.handleSetResourceSharing)
	s.registerPair("PUT", "/profiles/{name}/sharing/{resource}", s.handleSetResourceSharing)
	s.registerPair("POST", "/profiles/{name}/sharing", s.handleBatchSharing)
	s.registerPair("PUT", "/profiles/{name}/sharing", s.handleBatchSharing)
	s.registerPair("POST", "/profiles/{name}/sharing/config/seed", s.handleSeedConfig)

	s.registerPair("GET", "/profiles/{name}/conversations", s.handleGetConversations)

	s.registerPair("GET", "/quota", s.handleQuota)
	s.registerPair("GET", "/quota/history", s.handleQuotaHistory)
	s.registerPair("GET", "/quota/{profile}/history", s.handleQuotaProfileHistory)
	s.registerPair("GET", "/quota/{profile}", s.handleQuotaProfile)

	s.registerPair("GET", "/alerts", s.handleAlerts)
	s.registerPair("GET", "/alerts/{profile}", s.handleAlertsProfile)

	s.registerPair("GET", "/profiles/{name}/prime", s.handleGetProfilePrime)
	s.registerPair("GET", "/prime", s.handleGetAllPrime)
	s.registerPair("POST", "/profiles/{name}/prime", s.handlePostProfilePrime)
	s.registerPair("POST", "/prime", s.handlePostAllPrime)

	// SSE is /events and /api/v1/events. There is no /api/events alias.
	s.register("GET", "/events", s.handleEvents)
	s.register("GET", "/api/v1/events", s.handleEvents)

	// Gateway keeps the OpenAI/Anthropic /v1 prefix and the /api/v1 mirror.
	s.register("POST", "/v1/chat/completions", s.handleGatewayChatCompletions)
	s.register("POST", "/api/v1/chat/completions", s.handleGatewayChatCompletions)
	s.register("GET", "/v1/models", s.handleGatewayModels)
	s.register("GET", "/api/v1/models", s.handleGatewayModels)
	s.register("POST", "/v1/messages", s.handleGatewayMessages)
	s.register("POST", "/api/v1/messages", s.handleGatewayMessages)
	s.register("GET", "/v1/router/status", s.handleGatewayRouterStatus)
	s.register("GET", "/api/v1/router/status", s.handleGatewayRouterStatus)
	s.register("POST", "/v1/router/reset", s.handleGatewayRouterReset)
	s.register("POST", "/api/v1/router/reset", s.handleGatewayRouterReset)
	s.register("POST", "/v1/router/strategy", s.handleGatewayRouterStrategy)
	s.register("POST", "/api/v1/router/strategy", s.handleGatewayRouterStrategy)

	s.registerPair("GET", "/worktrees", s.handleListWorktrees)
	s.registerPair("POST", "/worktrees", s.handleCreateWorktree)
	s.registerPair("POST", "/worktrees/prune", s.handlePruneWorktrees)
	s.registerPair("GET", "/worktrees/{id}", s.handleGetWorktree)
	s.registerPair("DELETE", "/worktrees/{id}", s.handleDeleteWorktree)
	s.registerPair("GET", "/worktrees/{id}/status", s.handleGetWorktreeStatus)
	s.registerPair("GET", "/worktrees/{id}/diff", s.handleGetWorktreeDiff)

	s.registerPair("GET", "/agent/sessions", s.handleListAgentSessions)
	s.registerPair("POST", "/agent/sessions", s.handleCreateAgentSession)
	s.registerPair("POST", "/agent/sessions/prune", s.handlePruneAgentSessions)
	s.registerPair("GET", "/agent/sessions/{id}", s.handleGetAgentSession)
	s.registerPair("DELETE", "/agent/sessions/{id}", s.handleDeleteAgentSession)
	s.registerPair("GET", "/agent/sessions/{id}/output", s.handleGetAgentSessionOutput)
	s.registerPair("GET", "/agent/sessions/{id}/stream", s.handleStreamAgentSessionOutput)
	s.registerPair("POST", "/agent/sessions/{id}/input", s.handleSendAgentSessionInput)
	s.registerPair("POST", "/agent/sessions/{id}/resize", s.handleResizeAgentSession)

	s.registerPair("GET", "/dispatch/tasks", s.handleListDispatchedTasks)
	s.registerPair("POST", "/dispatch/tasks", s.handleCreateDispatchedTask)
	s.registerPair("POST", "/dispatch/tasks/prune", s.handlePruneDispatchedTasks)
	s.registerPair("GET", "/dispatch/tasks/{id}", s.handleGetDispatchedTask)
	s.registerPair("DELETE", "/dispatch/tasks/{id}", s.handleDeleteDispatchedTask)
	s.registerPair("POST", "/dispatch/tasks/{id}/cancel", s.handleCancelDispatchedTask)
	s.registerPair("GET", "/dispatch/tasks/{id}/logs", s.handleGetDispatchedTaskLogs)
	s.registerPair("GET", "/dispatch/tasks/{id}/stream", s.handleStreamDispatchedTaskLogs)
	s.registerPair("GET", "/dispatch/tasks/{id}/diff", s.handleGetDispatchedTaskDiff)
	s.registerPair("GET", "/dispatch/tasks/{id}/files", s.handleGetDispatchedTaskFiles)
	s.registerPair("GET", "/dispatch/dashboard", s.handleGetDispatchedDashboard)
	s.registerPair("POST", "/dispatch/plans", s.handleDispatchPlan)
	s.registerPair("GET", "/dispatch/plans", s.handleListDispatchPlans)
	s.registerPair("GET", "/dispatch/plans/{id}", s.handleGetDispatchPlan)

	s.registerPair("GET", "/workspaces", s.handleListWorkspaces)
	s.registerPair("GET", "/workspaces/active", s.handleActiveWorkspaces)
	s.registerPair("GET", "/profiles/{name}/workspaces", s.handleProfileWorkspaces)
	s.registerPair("GET", "/profiles/{name}/workspaces/active", s.handleProfileActiveWorkspace)

	s.registerPair("GET", "/headless", s.handleListHeadlessInstances)
	s.registerPair("GET", "/headless/{profile}", s.handleGetHeadlessStatus)
	s.registerPair("POST", "/headless/{profile}/start", s.handleStartHeadless)
	s.registerPair("POST", "/headless/{profile}/stop", s.handleStopHeadless)
	s.registerPair("POST", "/headless/{profile}/restart", s.handleRestartHeadless)
	s.registerPair("POST", "/headless/{profile}/run", s.handleRunHeadlessPrompt)
	s.registerPair("GET", "/headless/{profile}/logs", s.handleGetHeadlessLogs)
	s.registerPair("POST", "/exec", s.handleExecPrompts)

	s.register("GET", "/ui/tasks", s.handleUITasksDashboard)
	s.register("GET", "/ui/dispatch/tasks", s.handleUITasksDashboard)
	s.register("GET", "/ui/tasks/{id}/diff", s.handleUITaskDiff)
	s.register("GET", "/ui/dispatch/tasks/{id}/diff", s.handleUITaskDiff)

	s.register("POST", "/mcp", s.handleMCPJSONRPC)
	s.registerPair("POST", "/mcp", s.handleMCPJSONRPC)
	s.register("GET", "/mcp/sse", s.handleMCPSSE)
	s.registerPair("GET", "/mcp/sse", s.handleMCPSSE)
	s.register("POST", "/mcp/messages", s.handleMCPMessage)
	s.registerPair("POST", "/mcp/messages", s.handleMCPMessage)
}
