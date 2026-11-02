package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/agent"
	"github.com/ye-dev/multigravity-cli/internal/dispatch"
	"github.com/ye-dev/multigravity-cli/internal/headless"
	"github.com/ye-dev/multigravity-cli/internal/server/ui"
	"github.com/ye-dev/multigravity-cli/internal/workspace"
	"github.com/ye-dev/multigravity-cli/internal/worktree"
)

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

func (s *Server) handleDispatchPlan(w http.ResponseWriter, r *http.Request) {
	var req dispatch.PlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid plan payload: %v", err))
		return
	}

	// Override timeout or repo from query param if provided and empty in body
	if qRepo := r.URL.Query().Get("repo"); qRepo != "" && req.RepoPath == "" {
		req.RepoPath = qRepo
	}
	if qTimeout := r.URL.Query().Get("timeout"); qTimeout != "" && req.Timeout == "" {
		req.Timeout = qTimeout
	}

	result, err := dispatch.GetDefaultTaskManager().ExecutePlan(r.Context(), req)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if s.broker != nil {
		s.broker.Broadcast(SSEEvent{
			Event: "action",
			Time:  time.Now().UTC().Format(time.RFC3339),
			Data: map[string]any{
				"action":         "plan_dispatch",
				"status":         result.Status,
				"plan_id":        result.PlanID,
				"total_subtasks": result.TotalSubtasks,
				"succeeded":      result.Succeeded,
				"failed":         result.Failed,
			},
		})
	}

	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleListDispatchPlans(w http.ResponseWriter, r *http.Request) {
	repoPath := r.URL.Query().Get("repo")
	plans, err := dispatch.GetDefaultTaskManager().ListPlans(repoPath)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, plans)
}

func (s *Server) handleGetDispatchPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repoPath := r.URL.Query().Get("repo")
	plan, err := dispatch.GetDefaultTaskManager().GetPlan(repoPath, id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, plan)
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
	Model                      string `json:"model,omitempty"`
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
		Model:                      req.Model,
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
	Model                      string   `json:"model,omitempty"`
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
		Model:                      req.Model,
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
