package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/chat"
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, HEAD")
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

	// Doctor
	s.mux.HandleFunc("GET /api/v1/doctor", s.handleDoctor)

	// Profiles
	s.mux.HandleFunc("GET /api/v1/profiles", s.handleListProfiles)
	s.mux.HandleFunc("GET /api/v1/profiles/{name}", s.handleGetProfile)
	s.mux.HandleFunc("GET /api/v1/profiles/{name}/stats", s.handleGetProfileStats)
	s.mux.HandleFunc("GET /api/v1/stats", s.handleStats)

	// Sharing
	s.mux.HandleFunc("GET /api/v1/profiles/{name}/sharing", s.handleGetAllSharing)
	s.mux.HandleFunc("GET /api/v1/profiles/{name}/sharing/{resource}", s.handleGetResourceSharing)

	// AI Conversations
	s.mux.HandleFunc("GET /api/v1/profiles/{name}/conversations", s.handleGetConversations)

	// Quota
	s.mux.HandleFunc("GET /api/v1/quota", s.handleQuota)
	s.mux.HandleFunc("GET /api/v1/quota/{profile}", s.handleQuotaProfile)

	// Actions (stop, clean)
	s.mux.HandleFunc("POST /api/v1/profiles/{name}/stop", s.handleStopProfile)
	s.mux.HandleFunc("POST /api/v1/profiles/{name}/clean", s.handleCleanProfile)

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
	convs, err := chat.GetConversations(name)
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
