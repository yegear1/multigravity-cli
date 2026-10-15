package quota

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

const (
	// DefaultDepletionThreshold is the fraction (10%) at which a quota is considered nearing depletion.
	DefaultDepletionThreshold = 0.10

	// DefaultMaxTokensPerHour is the baseline hourly budget alert threshold (50k tokens).
	DefaultMaxTokensPerHour = 50000

	// DefaultMaxTokensPerMinute is the baseline per-minute burn rate alert threshold (2k tokens).
	DefaultMaxTokensPerMinute = 2000

	// DefaultThrottleDelayMs is the standard pacing delay applied when auto-throttling (1000ms).
	DefaultThrottleDelayMs = 1000

	throttleConfigFileName = "throttle-policy.json"
)

// TokenBurnRate aggregates token consumption velocity over a defined sliding window.
type TokenBurnRate struct {
	Profile         string  `json:"profile"`
	Window          string  `json:"window"`
	TokensBurned    int     `json:"tokens_burned"`
	TokensPerMinute float64 `json:"tokens_per_minute"`
	TokensPerHour   float64 `json:"tokens_per_hour"`
	SampleCount     int     `json:"sample_count"`
	FirstSample     string  `json:"first_sample,omitempty"`
	LastSample      string  `json:"last_sample,omitempty"`
}

// ThrottlePolicy defines limits and throttling actions for a single profile or fleet default.
type ThrottlePolicy struct {
	Profile             string  `json:"profile,omitempty"`
	MaxTokensPerHour    int     `json:"max_tokens_per_hour,omitempty"`
	MaxTokensPerMinute  int     `json:"max_tokens_per_minute,omitempty"`
	DepletionThreshold  float64 `json:"depletion_threshold,omitempty"`
	ThrottleDelayMs     int     `json:"throttle_delay_ms,omitempty"`
	AutoThrottleEnabled bool    `json:"auto_throttle_enabled"`
}

// ThrottleDelay returns the configured throttle pacing duration.
func (p ThrottlePolicy) ThrottleDelay() time.Duration {
	if p.ThrottleDelayMs <= 0 {
		return time.Duration(DefaultThrottleDelayMs) * time.Millisecond
	}
	return time.Duration(p.ThrottleDelayMs) * time.Millisecond
}

// EffectiveDepletionThreshold returns the depletion fraction threshold.
func (p ThrottlePolicy) EffectiveDepletionThreshold() float64 {
	if p.DepletionThreshold <= 0 {
		return DefaultDepletionThreshold
	}
	return p.DepletionThreshold
}

// ThrottleConfig contains the fleet-wide default policy and per-profile overrides.
type ThrottleConfig struct {
	DefaultPolicy ThrottlePolicy            `json:"default_policy"`
	Profiles      map[string]ThrottlePolicy `json:"profiles,omitempty"`
}

// PolicyForProfile returns the effective policy for a given profile, falling back to defaults.
func (c *ThrottleConfig) PolicyForProfile(name string) ThrottlePolicy {
	if c == nil {
		return DefaultThrottlePolicy(name)
	}
	if p, ok := c.Profiles[name]; ok {
		if p.Profile == "" {
			p.Profile = name
		}
		if p.DepletionThreshold <= 0 {
			p.DepletionThreshold = c.DefaultPolicy.EffectiveDepletionThreshold()
		}
		if p.ThrottleDelayMs <= 0 {
			p.ThrottleDelayMs = c.DefaultPolicy.ThrottleDelayMs
		}
		if p.MaxTokensPerHour <= 0 && c.DefaultPolicy.MaxTokensPerHour > 0 {
			p.MaxTokensPerHour = c.DefaultPolicy.MaxTokensPerHour
		}
		if p.MaxTokensPerMinute <= 0 && c.DefaultPolicy.MaxTokensPerMinute > 0 {
			p.MaxTokensPerMinute = c.DefaultPolicy.MaxTokensPerMinute
		}
		return p
	}
	pol := c.DefaultPolicy
	pol.Profile = name
	return pol
}

// DefaultThrottlePolicy returns a ThrottlePolicy populated with system defaults.
func DefaultThrottlePolicy(profileName string) ThrottlePolicy {
	return ThrottlePolicy{
		Profile:             profileName,
		MaxTokensPerHour:    DefaultMaxTokensPerHour,
		MaxTokensPerMinute:  DefaultMaxTokensPerMinute,
		DepletionThreshold:  DefaultDepletionThreshold,
		ThrottleDelayMs:     DefaultThrottleDelayMs,
		AutoThrottleEnabled: true,
	}
}

// DefaultThrottleConfig returns a ThrottleConfig initialized with default fleet policy.
func DefaultThrottleConfig() *ThrottleConfig {
	return &ThrottleConfig{
		DefaultPolicy: ThrottlePolicy{
			MaxTokensPerHour:    DefaultMaxTokensPerHour,
			MaxTokensPerMinute:  DefaultMaxTokensPerMinute,
			DepletionThreshold:  DefaultDepletionThreshold,
			ThrottleDelayMs:     DefaultThrottleDelayMs,
			AutoThrottleEnabled: true,
		},
		Profiles: make(map[string]ThrottlePolicy),
	}
}

var (
	throttleMu             sync.RWMutex
	customThrottleFilePath string
)

// SetTestThrottleConfigPath overrides the configuration file path for testing.
func SetTestThrottleConfigPath(path string) func() {
	throttleMu.Lock()
	prev := customThrottleFilePath
	customThrottleFilePath = path
	throttleMu.Unlock()
	return func() {
		throttleMu.Lock()
		customThrottleFilePath = prev
		throttleMu.Unlock()
	}
}

func getThrottleConfigPath() string {
	throttleMu.RLock()
	defer throttleMu.RUnlock()
	if customThrottleFilePath != "" {
		return customThrottleFilePath
	}
	home := config.GetMultigravityHome()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".multigravity", throttleConfigFileName)
}

// LoadThrottleConfig reads the configuration file or returns defaults if absent.
func LoadThrottleConfig() (*ThrottleConfig, error) {
	path := getThrottleConfigPath()
	if path == "" {
		return DefaultThrottleConfig(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultThrottleConfig(), nil
		}
		return nil, err
	}
	var cfg ThrottleConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse throttle config: %w", err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]ThrottlePolicy)
	}
	if cfg.DefaultPolicy.DepletionThreshold <= 0 {
		cfg.DefaultPolicy.DepletionThreshold = DefaultDepletionThreshold
	}
	if cfg.DefaultPolicy.ThrottleDelayMs <= 0 {
		cfg.DefaultPolicy.ThrottleDelayMs = DefaultThrottleDelayMs
	}
	return &cfg, nil
}

// SaveThrottleConfig persists the configuration to disk.
func SaveThrottleConfig(cfg *ThrottleConfig) error {
	if cfg == nil {
		return fmt.Errorf("cannot save nil throttle config")
	}
	path := getThrottleConfigPath()
	if path == "" {
		return fmt.Errorf("multigravity home directory is unavailable")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// CalculateTokenBurnRate computes token burn rate for a profile over the given window.
func CalculateTokenBurnRate(profileName string, window time.Duration, now time.Time) (*TokenBurnRate, error) {
	if err := config.ValidateProfileName(profileName); err != nil {
		return nil, err
	}
	if !profile.ProfileExists(profileName) {
		return nil, fmt.Errorf("profile %q does not exist", profileName)
	}
	if window <= 0 {
		window = time.Hour
	}
	if now.IsZero() {
		now = historyNow()
	}

	since := now.Add(-window)
	samples, err := readSamples(profileName, HistoryQuery{
		Since: since,
		Until: now,
		Limit: maxHistorySamples,
	})
	if err != nil {
		return nil, err
	}

	burn := &TokenBurnRate{
		Profile: profileName,
		Window:  window.String(),
	}

	var firstTime, lastTime time.Time
	for _, sample := range samples {
		if sample.Tokens == nil {
			continue
		}
		total := sample.Tokens.TotalTokens
		if total <= 0 {
			total = sample.Tokens.PromptTokens + sample.Tokens.CompletionTokens
		}
		if total <= 0 {
			continue
		}
		burn.TokensBurned += total
		burn.SampleCount++
		if firstTime.IsZero() || sample.Timestamp.Before(firstTime) {
			firstTime = sample.Timestamp
		}
		if lastTime.IsZero() || sample.Timestamp.After(lastTime) {
			lastTime = sample.Timestamp
		}
	}

	if !firstTime.IsZero() {
		burn.FirstSample = firstTime.UTC().Format(time.RFC3339)
	}
	if !lastTime.IsZero() {
		burn.LastSample = lastTime.UTC().Format(time.RFC3339)
	}

	minutes := window.Minutes()
	if minutes > 0 {
		burn.TokensPerMinute = float64(burn.TokensBurned) / minutes
	}
	hours := window.Hours()
	if hours > 0 {
		burn.TokensPerHour = float64(burn.TokensBurned) / hours
	}

	return burn, nil
}

// CalculateFleetBurnRates computes token burn rates across all profiles.
func CalculateFleetBurnRates(window time.Duration, now time.Time) (map[string]*TokenBurnRate, error) {
	names, err := profile.ListProfiles()
	if err != nil {
		return nil, err
	}
	fleet := make(map[string]*TokenBurnRate)
	for _, name := range names {
		rate, err := CalculateTokenBurnRate(name, window, now)
		if err != nil {
			return nil, err
		}
		fleet[name] = rate
	}
	return fleet, nil
}
