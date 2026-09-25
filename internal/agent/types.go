package agent

import "time"

// SessionStatus represents the lifecycle state of an agent terminal session.
type SessionStatus string

const (
	StatusRunning SessionStatus = "running"
	StatusStopped SessionStatus = "stopped"
	StatusExited  SessionStatus = "exited"
	StatusFailed  SessionStatus = "failed"
)

// Session contains the serialized metadata and state of an agent session.
type Session struct {
	ID          string        `json:"id"`
	Profile     string        `json:"profile"`
	AgentType   string        `json:"agent_type"`
	Command     string        `json:"command"`
	Args        []string      `json:"args,omitempty"`
	Cwd         string        `json:"cwd"`
	WorktreeID  string        `json:"worktree_id,omitempty"`
	Status      SessionStatus `json:"status"`
	ExitCode    int           `json:"exit_code"`
	PID         int           `json:"pid"`
	CreatedAt   time.Time     `json:"created_at"`
	StartedAt   time.Time     `json:"started_at"`
	EndedAt     *time.Time    `json:"ended_at,omitempty"`
	Rows        uint16        `json:"rows"`
	Cols        uint16        `json:"cols"`
	GatewayURL  string        `json:"gateway_url,omitempty"`
}

// CreateSessionOptions specifies parameters for launching an agent session.
type CreateSessionOptions struct {
	Profile    string            `json:"profile"`
	AgentType  string            `json:"agent_type,omitempty"`
	Command    string            `json:"command"`
	Args       []string          `json:"args,omitempty"`
	Cwd        string            `json:"cwd,omitempty"`
	WorktreeID string            `json:"worktree_id,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	GatewayURL string            `json:"gateway_url,omitempty"`
	GatewayKey string            `json:"gateway_key,omitempty"`
	Rows       uint16            `json:"rows,omitempty"`
	Cols       uint16            `json:"cols,omitempty"`
	Detached   bool              `json:"detached,omitempty"`
}

// SessionFilter filters sessions during listing queries.
type SessionFilter struct {
	Profile   string
	Status    SessionStatus
	AgentType string
}

// OutputChunk is a data event emitted during terminal output streaming.
type OutputChunk struct {
	SessionID string    `json:"session_id"`
	Data      string    `json:"data"`
	Offset    int64     `json:"offset"`
	Timestamp time.Time `json:"timestamp"`
}

// ResizeOptions specifies new terminal dimensions.
type ResizeOptions struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

// SendInputRequest carries raw data to write to session stdin.
type SendInputRequest struct {
	Data string `json:"data"`
}
