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
	"github.com/ye-dev/multigravity-cli/internal/prime"
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, X-Requested-With, X-Profile")
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
	s.mux.HandleFunc("GET /api/v1/quota/{profile}", s.handleQuotaProfile)
	s.mux.HandleFunc("GET /api/quota/{profile}", s.handleQuotaProfile)

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
}

func (s *Server) handleGatewayChatCompletions(w http.ResponseWriter, r *http.Request) {
	if s.gateway != nil {
		s.gateway.HandleChatCompletions(w, r)
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
