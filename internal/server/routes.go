package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/chat"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/doctor"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS, HEAD")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, X-Requested-With")
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

	// Sharing
	s.mux.HandleFunc("GET /api/v1/profiles/{name}/sharing", s.handleGetAllSharing)
	s.mux.HandleFunc("GET /api/profiles/{name}/sharing", s.handleGetAllSharing)
	s.mux.HandleFunc("GET /api/v1/profiles/{name}/sharing/{resource}", s.handleGetResourceSharing)
	s.mux.HandleFunc("GET /api/profiles/{name}/sharing/{resource}", s.handleGetResourceSharing)

	// AI Conversations
	s.mux.HandleFunc("GET /api/v1/profiles/{name}/conversations", s.handleGetConversations)
	s.mux.HandleFunc("GET /api/profiles/{name}/conversations", s.handleGetConversations)

	// Quota
	s.mux.HandleFunc("GET /api/v1/quota", s.handleQuota)
	s.mux.HandleFunc("GET /api/quota", s.handleQuota)
	s.mux.HandleFunc("GET /api/v1/quota/{profile}", s.handleQuotaProfile)
	s.mux.HandleFunc("GET /api/quota/{profile}", s.handleQuotaProfile)

	// Real-time Streaming (SSE)
	s.mux.HandleFunc("GET /events", s.handleEvents)
	s.mux.HandleFunc("GET /api/v1/events", s.handleEvents)
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

	switch resource {
	case "mcp":
		status, err = profile.GetMcpStatus(name)
	case "skills":
		status, err = profile.GetSkillsStatus(name)
	case "config":
		status, err = profile.GetConfigStatus(name)
	case "gh":
		status, err = profile.GetGhStatus(name)
	default:
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid resource %q: must be mcp, skills, config, or gh", resource))
		return
	}

	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, status)
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
	s.writeJSON(w, http.StatusOK, servers)
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
		"profile":      name,
		"status":       "cleaned",
		"size_before":  before,
		"size_after":   after,
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
