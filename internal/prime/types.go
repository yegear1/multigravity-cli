package prime

import (
	"time"

	"github.com/ye-dev/multigravity-cli/internal/quota"
)

// PrimeProgressEvent represents a stage transition or milestone during priming
type PrimeProgressEvent struct {
	Profile   string         `json:"profile"`
	Bucket    string         `json:"bucket,omitempty"`
	BucketID  string         `json:"bucket_id,omitempty"`
	Stage     string         `json:"stage"` // "initializing", "server_ready", "checking", "jitter_waiting", "dispatching", "primed", "linked", "skipped", "aborted", "ready", "error", "completed"
	Message   string         `json:"message"`
	Timestamp string         `json:"timestamp"`
	Details   map[string]any `json:"details,omitempty"`
}

// ProgressCallback is called as priming proceeds
type ProgressCallback func(event PrimeProgressEvent)

// BucketStatusReport contains cycle status and telemetry for a single bucket
type BucketStatusReport struct {
	Key               string  `json:"key"`
	BucketID          string  `json:"bucket_id"`
	Name              string  `json:"name"`
	Period            string  `json:"period"`
	Model             string  `json:"model"`
	ModelLabel        string  `json:"model_label"`
	Available         bool    `json:"available"`
	RemainingFraction float64 `json:"remaining_fraction"`
	RemainingPercent  float64 `json:"remaining_percent"`
	ResetTime         string  `json:"reset_time"`
	TimeLeft          string  `json:"time_left"`
	SecondsUntilReset int64   `json:"seconds_until_reset"`
	IsRefreshed       bool    `json:"is_refreshed"`
	CycleStatus       string  `json:"cycle_status"`
	LastPrimedAt      string  `json:"last_primed_at"`
	LastModel         string  `json:"last_model"`
	LastPrompt        string  `json:"last_prompt"`
	LastResetTime     string  `json:"last_reset_time"`
	TargetPrimeTime   string  `json:"target_prime_time"`
}

// ProfilePrimeStatus contains full priming status for a profile
type ProfilePrimeStatus struct {
	Profile        string               `json:"profile"`
	ServerMode     string               `json:"server_mode"` // "active", "headless", "offline"
	PID            int                  `json:"pid,omitempty"`
	Port           int                  `json:"port,omitempty"`
	Buckets        []BucketStatusReport `json:"buckets"`
	CrontabDesc    string               `json:"crontab_status,omitempty"`
	SystemdDesc    string               `json:"systemd_status,omitempty"`
	PromptsCatalog string               `json:"prompts_catalog"`
	PromptsCount   int                  `json:"prompts_count"`
	Error          string               `json:"error,omitempty"`
}

// BucketPrimeResult records the outcome of priming a single bucket
type BucketPrimeResult struct {
	Key        string `json:"key"`
	BucketID   string `json:"bucket_id"`
	Name       string `json:"name"`
	Model      string `json:"model"`
	ModelLabel string `json:"model_label"`
	Status     string `json:"status"` // "primed", "linked", "skipped", "ready", "aborted", "error"
	Reason     string `json:"reason,omitempty"`
	Prompt     string `json:"prompt,omitempty"`
	CascadeID  string `json:"cascade_id,omitempty"`
	ResetTime  string `json:"reset_time,omitempty"`
	PrimedAt   string `json:"primed_at,omitempty"`
}

// ProfilePrimeResult records the outcome of priming a profile
type ProfilePrimeResult struct {
	Profile    string              `json:"profile"`
	ServerMode string              `json:"server_mode"`
	PID        int                 `json:"pid,omitempty"`
	Port       int                 `json:"port,omitempty"`
	Buckets    []BucketPrimeResult `json:"buckets"`
	Error      string              `json:"error,omitempty"`
}

// QuotaClient defines language server communication for priming
type QuotaClient interface {
	RetrieveUserQuotaSummary(port int, csrf string) (*quota.QuotaSummaryResponse, error)
	StartCascade(port int, csrf string) (string, error)
	SendUserCascadeMessage(port int, csrf string, cascadeID string, prompt string, model string) error
}

var (
	findActiveServersFn = quota.FindActiveServers
	startHeadlessFn     = quota.StartHeadlessServer
	newQuotaClientFn    = func(timeout time.Duration) QuotaClient {
		return quota.NewClient(timeout)
	}
	graceDelay = 3 * time.Second
)

// SetTestHooks configures test overrides and returns a cleanup function
func SetTestHooks(
	findServers func(profile string) ([]quota.ActiveServer, error),
	startHeadless func(profile string) (*quota.HeadlessInstance, error),
	newClient func(timeout time.Duration) QuotaClient,
	delay time.Duration,
) func() {
	origFind := findActiveServersFn
	origHeadless := startHeadlessFn
	origClient := newQuotaClientFn
	origDelay := graceDelay

	if findServers != nil {
		findActiveServersFn = findServers
	}
	if startHeadless != nil {
		startHeadlessFn = startHeadless
	}
	if newClient != nil {
		newQuotaClientFn = newClient
	}
	graceDelay = delay

	return func() {
		findActiveServersFn = origFind
		startHeadlessFn = origHeadless
		newQuotaClientFn = origClient
		graceDelay = origDelay
	}
}
