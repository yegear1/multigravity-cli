package quota

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

const (
	// DefaultRunesPerToken is the legacy baseline ratio (4 runes per token).
	DefaultRunesPerToken = 4.0

	// MinRunesPerToken and MaxRunesPerToken bound the calibrated ratio to prevent extremes.
	MinRunesPerToken = 1.0
	MaxRunesPerToken = 8.0

	// PriorTokens is the Bayesian prior weight representing initial trust in the 4.0 baseline.
	PriorTokens = 20.0
	PriorRunes  = DefaultRunesPerToken * PriorTokens

	calibrationFileName = "token-calibration.json"
)

// ModelCalibration tracks token ratio observations for a specific model.
type ModelCalibration struct {
	Model                     string  `json:"model"`
	Samples                   int     `json:"samples"`
	PromptRunes               int64   `json:"prompt_runes"`
	PromptTokens              int64   `json:"prompt_tokens"`
	CompletionRunes           int64   `json:"completion_runes"`
	CompletionTokens          int64   `json:"completion_tokens"`
	TotalRunes                int64   `json:"total_runes"`
	TotalTokens               int64   `json:"total_tokens"`
	RunesPerToken             float64 `json:"runes_per_token"`
	PromptRunesPerToken       float64 `json:"prompt_runes_per_token,omitempty"`
	CompletionRunesPerToken   float64 `json:"completion_runes_per_token,omitempty"`
}

// CalibrationStats contains snapshot telemetry of calibrated estimation heuristics.
type CalibrationStats struct {
	Samples                 int                         `json:"samples"`
	RunesPerToken           float64                     `json:"runes_per_token"`
	PromptRunesPerToken     float64                     `json:"prompt_runes_per_token,omitempty"`
	CompletionRunesPerToken float64                     `json:"completion_runes_per_token,omitempty"`
	TotalRunes              int64                       `json:"total_runes"`
	TotalTokens             int64                       `json:"total_tokens"`
	Models                  map[string]ModelCalibration `json:"models,omitempty"`
	LastCalibrated          string                      `json:"last_calibrated,omitempty"`
}

// TokenCalibrator maintains running token estimation ratios across models and segments.
type TokenCalibrator struct {
	mu               sync.RWMutex
	samples          int
	promptRunes      int64
	promptTokens     int64
	completionRunes  int64
	completionTokens int64
	totalRunes       int64
	totalTokens      int64
	lastCalibrated   time.Time
	models           map[string]*ModelCalibration
	persistPath      string
	disablePersist   bool
}

// NewTokenCalibrator creates an initialized TokenCalibrator.
// If persistPath is empty, persistence is disabled (in-memory only).
func NewTokenCalibrator(persistPath string) *TokenCalibrator {
	c := &TokenCalibrator{
		models:         make(map[string]*ModelCalibration),
		persistPath:    persistPath,
		disablePersist: persistPath == "",
	}
	if persistPath != "" {
		_ = c.load()
	}
	return c
}

var defaultCalibrator = &TokenCalibrator{
	models: make(map[string]*ModelCalibration),
}

func (c *TokenCalibrator) getPersistPath() string {
	if c.disablePersist {
		return ""
	}
	if c.persistPath != "" {
		return c.persistPath
	}
	home := config.GetMultigravityHome()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".multigravity", calibrationFileName)
}

func clampRatio(r float64) float64 {
	if math.IsNaN(r) || math.IsInf(r, 0) || r <= 0 {
		return DefaultRunesPerToken
	}
	if r < MinRunesPerToken {
		return MinRunesPerToken
	}
	if r > MaxRunesPerToken {
		return MaxRunesPerToken
	}
	return r
}

func computeRatio(runes, tokens int64) float64 {
	effectiveRunes := float64(runes) + PriorRunes
	effectiveTokens := float64(tokens) + PriorTokens
	if effectiveTokens <= 0 {
		return DefaultRunesPerToken
	}
	return clampRatio(effectiveRunes / effectiveTokens)
}

// CalibrateTokens records an observed (ground-truth) request and completion measurement.
func (c *TokenCalibrator) CalibrateTokens(model, promptText string, promptTokens int, completionText string, completionTokens int) {
	if promptTokens < 0 {
		promptTokens = 0
	}
	if completionTokens < 0 {
		completionTokens = 0
	}
	totalTokens := promptTokens + completionTokens
	if totalTokens <= 0 {
		return
	}

	pRunes := int64(utf8.RuneCountInString(promptText))
	cRunes := int64(utf8.RuneCountInString(completionText))
	totRunes := pRunes + cRunes

	c.mu.Lock()
	defer c.mu.Unlock()

	c.samples++
	c.promptRunes += pRunes
	c.promptTokens += int64(promptTokens)
	c.completionRunes += cRunes
	c.completionTokens += int64(completionTokens)
	c.totalRunes += totRunes
	c.totalTokens += int64(totalTokens)
	c.lastCalibrated = time.Now().UTC()

	normModel := strings.TrimSpace(strings.ToLower(model))
	if normModel != "" {
		mc, ok := c.models[normModel]
		if !ok {
			mc = &ModelCalibration{
				Model: normModel,
			}
			c.models[normModel] = mc
		}
		mc.Samples++
		mc.PromptRunes += pRunes
		mc.PromptTokens += int64(promptTokens)
		mc.CompletionRunes += cRunes
		mc.CompletionTokens += int64(completionTokens)
		mc.TotalRunes += totRunes
		mc.TotalTokens += int64(totalTokens)

		mc.RunesPerToken = computeRatio(mc.TotalRunes, mc.TotalTokens)
		if mc.PromptTokens > 0 {
			mc.PromptRunesPerToken = computeRatio(mc.PromptRunes, mc.PromptTokens)
		}
		if mc.CompletionTokens > 0 {
			mc.CompletionRunesPerToken = computeRatio(mc.CompletionRunes, mc.CompletionTokens)
		}
	}

	if c.persistPath != "" {
		_ = c.saveLocked()
	}
}

// EstimateTokens calculates approximate token count using calibrated ratio.
func (c *TokenCalibrator) EstimateTokens(text string) int {
	return c.EstimateTokensForModel("", text)
}

// EstimateTokensForModel calculates approximate tokens for a given model.
func (c *TokenCalibrator) EstimateTokensForModel(model, text string) int {
	return c.EstimateSegmentTokens(model, "", text)
}

// EstimateSegmentTokens estimates tokens for a specific model and segment ("prompt", "completion", or "").
func (c *TokenCalibrator) EstimateSegmentTokens(model, segment, text string) int {
	n := utf8.RuneCountInString(text)
	if n == 0 {
		return 0
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	ratio := DefaultRunesPerToken
	normModel := strings.TrimSpace(strings.ToLower(model))

	if mc, ok := c.models[normModel]; ok && mc.Samples > 0 {
		switch strings.ToLower(segment) {
		case "prompt":
			if mc.PromptRunesPerToken > 0 {
				ratio = mc.PromptRunesPerToken
			} else {
				ratio = mc.RunesPerToken
			}
		case "completion":
			if mc.CompletionRunesPerToken > 0 {
				ratio = mc.CompletionRunesPerToken
			} else {
				ratio = mc.RunesPerToken
			}
		default:
			ratio = mc.RunesPerToken
		}
	} else if c.samples > 0 {
		switch strings.ToLower(segment) {
		case "prompt":
			if c.promptTokens > 0 {
				ratio = computeRatio(c.promptRunes, c.promptTokens)
			} else {
				ratio = computeRatio(c.totalRunes, c.totalTokens)
			}
		case "completion":
			if c.completionTokens > 0 {
				ratio = computeRatio(c.completionRunes, c.completionTokens)
			} else {
				ratio = computeRatio(c.totalRunes, c.totalTokens)
			}
		default:
			ratio = computeRatio(c.totalRunes, c.totalTokens)
		}
	}

	if ratio <= 0 {
		ratio = DefaultRunesPerToken
	}

	// Ceiling ensures 1 rune is at least 1 token
	return int(math.Ceil(float64(n) / ratio))
}

// GetStats returns a snapshot of active calibration heuristics.
func (c *TokenCalibrator) GetStats() CalibrationStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ratio := DefaultRunesPerToken
	var promptRatio, compRatio float64

	if c.samples > 0 {
		ratio = computeRatio(c.totalRunes, c.totalTokens)
		if c.promptTokens > 0 {
			promptRatio = computeRatio(c.promptRunes, c.promptTokens)
		}
		if c.completionTokens > 0 {
			compRatio = computeRatio(c.completionRunes, c.completionTokens)
		}
	}

	modelsCopy := make(map[string]ModelCalibration, len(c.models))
	for k, v := range c.models {
		modelsCopy[k] = *v
	}

	lastCal := ""
	if !c.lastCalibrated.IsZero() {
		lastCal = c.lastCalibrated.Format(time.RFC3339)
	}

	return CalibrationStats{
		Samples:                 c.samples,
		RunesPerToken:           ratio,
		PromptRunesPerToken:     promptRatio,
		CompletionRunesPerToken: compRatio,
		TotalRunes:              c.totalRunes,
		TotalTokens:             c.totalTokens,
		Models:                  modelsCopy,
		LastCalibrated:          lastCal,
	}
}

// Reset clears all calibration observations.
func (c *TokenCalibrator) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.samples = 0
	c.promptRunes = 0
	c.promptTokens = 0
	c.completionRunes = 0
	c.completionTokens = 0
	c.totalRunes = 0
	c.totalTokens = 0
	c.lastCalibrated = time.Time{}
	c.models = make(map[string]*ModelCalibration)

	path := c.getPersistPath()
	if path != "" {
		_ = os.Remove(path)
	}
}

func (c *TokenCalibrator) saveLocked() error {
	path := c.getPersistPath()
	if path == "" {
		return nil
	}

	stats := CalibrationStats{
		Samples:                 c.samples,
		RunesPerToken:           computeRatio(c.totalRunes, c.totalTokens),
		PromptRunesPerToken:     computeRatio(c.promptRunes, c.promptTokens),
		CompletionRunesPerToken: computeRatio(c.completionRunes, c.completionTokens),
		TotalRunes:              c.totalRunes,
		TotalTokens:             c.totalTokens,
		Models:                  make(map[string]ModelCalibration, len(c.models)),
	}
	if !c.lastCalibrated.IsZero() {
		stats.LastCalibrated = c.lastCalibrated.Format(time.RFC3339)
	}
	for k, v := range c.models {
		stats.Models[k] = *v
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (c *TokenCalibrator) load() error {
	path := c.getPersistPath()
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var stats CalibrationStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.samples = stats.Samples
	c.totalRunes = stats.TotalRunes
	c.totalTokens = stats.TotalTokens
	if stats.LastCalibrated != "" {
		t, _ := time.Parse(time.RFC3339, stats.LastCalibrated)
		c.lastCalibrated = t
	}

	c.models = make(map[string]*ModelCalibration, len(stats.Models))
	for k, v := range stats.Models {
		cp := v
		c.models[k] = &cp
	}

	return nil
}

// Package-level functions delegating to defaultCalibrator

// CalibrateTokens records an observed token response against default calibrator.
func CalibrateTokens(model, promptText string, promptTokens int, completionText string, completionTokens int) {
	defaultCalibrator.CalibrateTokens(model, promptText, promptTokens, completionText, completionTokens)
}

// EstimateTokensForModel estimates tokens using calibrated ratios for the given model.
func EstimateTokensForModel(model, text string) int {
	return defaultCalibrator.EstimateTokensForModel(model, text)
}

// EstimateSegmentTokens estimates tokens for a model and segment ("prompt" / "completion").
func EstimateSegmentTokens(model, segment, text string) int {
	return defaultCalibrator.EstimateSegmentTokens(model, segment, text)
}

// GetCalibrationStats returns a snapshot of the current calibration stats.
func GetCalibrationStats() CalibrationStats {
	return defaultCalibrator.GetStats()
}

// ResetCalibration clears all observations from the default calibrator.
func ResetCalibration() {
	defaultCalibrator.Reset()
}
