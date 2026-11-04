package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/catalog"
	"github.com/ye-dev/multigravity-cli/internal/doctor"
	"github.com/ye-dev/multigravity-cli/internal/idelog"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

// HealthResponse holds the health check status
type HealthResponse struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptime_seconds"`
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

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		name = queryString(r, "profile")
	}
	rep, err := catalog.Inspect(name)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			s.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, rep)
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

func (s *Server) handleGetIDELogs(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	tail := queryInt(r, "tail", 100)
	if tail < 0 {
		tail = 100
	}
	follow := queryBool(r, "follow")
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
