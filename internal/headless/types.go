package headless

import "time"

// HeadlessState represents the health and process status of a background headless server.
type HeadlessState string

const (
	StateRunning   HeadlessState = "running"
	StateStopped   HeadlessState = "stopped"
	StateUnhealthy HeadlessState = "unhealthy"
	StateFailed    HeadlessState = "failed"
)

// InstanceInfo holds serialized and live runtime metadata for a profile's headless server.
type InstanceInfo struct {
	Profile        string        `json:"profile"`
	PID            int           `json:"pid"`
	Port           int           `json:"port"`
	CSRFToken      string        `json:"csrf_token"`
	Status         HeadlessState `json:"status"`
	StartedAt      time.Time     `json:"started_at"`
	LogFile        string        `json:"log_file,omitempty"`
	HealthError    string        `json:"health_error,omitempty"`
	ExecutablePath string        `json:"executable_path,omitempty"`
}

// StartOptions specifies parameters for starting a background headless server.
type StartOptions struct {
	Profile      string        `json:"profile"`
	Port         int           `json:"port,omitempty"`
	Timeout      time.Duration `json:"timeout,omitempty"`
	ForceRestart bool          `json:"force_restart,omitempty"`
}

// AgentRunOptions defines execution parameters for a headless agent prompt.
type AgentRunOptions struct {
	Profile                    string        `json:"profile"`
	Prompt                     string        `json:"prompt"`
	Timeout                    time.Duration `json:"timeout,omitempty"`
	DangerouslySkipPermissions bool          `json:"dangerously_skip_permissions,omitempty"`
	OutputFormat               string        `json:"output_format,omitempty"`
}

// AgentRunResult models the structured output and telemetry of a headless agent run.
type AgentRunResult struct {
	Profile         string  `json:"profile"`
	Prompt          string  `json:"prompt"`
	Response        string  `json:"response"`
	TotalTokens     int     `json:"total_tokens,omitempty"`
	DurationSeconds float64 `json:"duration_seconds"`
	ExitCode        int     `json:"exit_code"`
	Error           string  `json:"error,omitempty"`
}
