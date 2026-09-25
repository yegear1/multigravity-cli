package agent

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// Manager coordinates the creation, inspection, and termination of agent sessions.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*SessionInstance
}

var (
	defaultManager     *Manager
	defaultManagerOnce sync.Once
)

// GetDefaultManager returns the process-wide agent Session Manager singleton.
func GetDefaultManager() *Manager {
	defaultManagerOnce.Do(func() {
		defaultManager = NewManager()
	})
	return defaultManager
}

// NewManager creates an isolated Manager instance.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*SessionInstance),
	}
}

// GenerateSessionID produces a unique identifier for an agent session.
func GenerateSessionID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("pty-%d-%s", time.Now().Unix(), hex.EncodeToString(b))
}

// StartSession launches a new agent process within an isolated pseudo-terminal environment.
func (m *Manager) StartSession(opts CreateSessionOptions) (*SessionInstance, error) {
	if opts.Command == "" {
		return nil, errors.New("command is required to start agent session")
	}

	env, err := BuildAgentEnv(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to build agent environment: %w", err)
	}

	cmd := exec.Command(opts.Command, opts.Args...)
	cmd.Env = env
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}

	rows := opts.Rows
	if rows == 0 {
		rows = 24
	}
	cols := opts.Cols
	if cols == 0 {
		cols = 80
	}

	ptyDev, err := startPTY(cmd, rows, cols)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty process: %w", err)
	}

	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}

	agentType := opts.AgentType
	if agentType == "" {
		agentType = DetectAgentType(opts.Command)
	}

	now := time.Now()
	sessionID := GenerateSessionID()

	info := Session{
		ID:         sessionID,
		Profile:    opts.Profile,
		AgentType:  agentType,
		Command:    opts.Command,
		Args:       opts.Args,
		Cwd:        opts.Cwd,
		WorktreeID: opts.WorktreeID,
		Status:     StatusRunning,
		ExitCode:   -1,
		PID:        pid,
		CreatedAt:  now,
		StartedAt:  now,
		Rows:       rows,
		Cols:       cols,
		GatewayURL: opts.GatewayURL,
	}

	instance := newSessionInstance(info, cmd, ptyDev)

	m.mu.Lock()
	m.sessions[sessionID] = instance
	m.mu.Unlock()

	return instance, nil
}

// GetSession retrieves an active or past session by ID.
func (m *Manager) GetSession(id string) (*SessionInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	inst, exists := m.sessions[id]
	if !exists {
		return nil, fmt.Errorf("session %q not found", id)
	}
	return inst, nil
}

// ListSessions returns snapshots of sessions matching the provided filter.
func (m *Manager) ListSessions(filter SessionFilter) []Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []Session
	for _, inst := range m.sessions {
		info := inst.GetInfo()

		if filter.Profile != "" && info.Profile != filter.Profile {
			continue
		}
		if filter.Status != "" && info.Status != filter.Status {
			continue
		}
		if filter.AgentType != "" && info.AgentType != filter.AgentType {
			continue
		}

		results = append(results, info)
	}
	return results
}

// StopSession requests graceful shutdown of a session.
func (m *Manager) StopSession(id string) error {
	inst, err := m.GetSession(id)
	if err != nil {
		return err
	}
	return inst.Stop()
}

// KillSession forces termination of a session.
func (m *Manager) KillSession(id string) error {
	inst, err := m.GetSession(id)
	if err != nil {
		return err
	}
	return inst.Kill()
}

// WriteSessionInput passes raw input bytes to a session.
func (m *Manager) WriteSessionInput(id string, data []byte) error {
	inst, err := m.GetSession(id)
	if err != nil {
		return err
	}
	_, err = inst.WriteInput(data)
	return err
}

// ResizeSession updates terminal dimensions for a session.
func (m *Manager) ResizeSession(id string, rows, cols uint16) error {
	inst, err := m.GetSession(id)
	if err != nil {
		return err
	}
	return inst.Resize(rows, cols)
}

// GetSessionOutput returns buffered output for a session.
func (m *Manager) GetSessionOutput(id string, tailBytes int) ([]byte, error) {
	inst, err := m.GetSession(id)
	if err != nil {
		return nil, err
	}
	return inst.GetOutput(tailBytes), nil
}

// SubscribeSession returns a live output chunk channel for a session.
func (m *Manager) SubscribeSession(id string) (<-chan OutputChunk, func(), error) {
	inst, err := m.GetSession(id)
	if err != nil {
		return nil, nil, err
	}
	ch, unsub := inst.Subscribe()
	return ch, unsub, nil
}

// PruneSessions removes terminated sessions that ended longer than maxAge ago.
func (m *Manager) PruneSessions(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	pruned := 0

	for id, inst := range m.sessions {
		info := inst.GetInfo()
		if info.Status != StatusRunning && info.EndedAt != nil {
			if now.Sub(*info.EndedAt) > maxAge {
				delete(m.sessions, id)
				pruned++
			}
		}
	}

	return pruned
}
