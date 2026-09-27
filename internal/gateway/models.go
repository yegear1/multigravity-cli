package gateway

import (
	"strings"
	"time"
)

// SupportedModels defines the official catalog of models exposed by the gateway
var SupportedModels = []string{
	"gemini-2.5-pro",
	"gemini-2.5-flash",
	"gemini-2.5-flash-lite",
	"gemini-3.5-flash",
	"gemini-3.5-flash-high",
	"gemini-3.5-flash-medium",
	"gemini-3.5-flash-low",
	"gemini-3.6-flash-high",
	"gemini-3.6-flash-medium",
	"gemini-3.6-flash-low",
	"gemini-3.7-flash-tiered",
	"gemini-3.8-flash-tiered",
	"gemini-pro-agent",
	"gemini-3.1-pro-low",
	"claude-sonnet-4-6",
	"claude-opus-4-6",
	"claude-opus-4-6-thinking",
	"claude-3-5-sonnet",
	"claude-3-7-sonnet",
	"claude-3-opus",
	"gpt-4o",
	"gpt-4o-mini",
	"gpt-oss-120b-medium",
}

// ModelAliases maps client/tool-facing aliases to internal backend model identifiers
var ModelAliases = map[string]string{
	// OpenAI mappings
	"gpt-4o":        "gemini-2.5-pro",
	"gpt-4o-mini":   "gemini-2.5-flash",
	"gpt-4-turbo":   "gemini-2.5-pro",
	"gpt-4":         "gemini-2.5-pro",
	"gpt-3.5-turbo": "gemini-2.5-flash",

	// Anthropic / Claude mappings
	"claude-3-5-sonnet":          "claude-sonnet-4-6",
	"claude-3.5-sonnet":          "claude-sonnet-4-6",
	"claude-3-5-sonnet-latest":   "claude-sonnet-4-6",
	"claude-3-5-sonnet-20241022": "claude-sonnet-4-6",
	"claude-3-7-sonnet":          "gemini-3.6-flash-high",
	"claude-3.7-sonnet":          "gemini-3.6-flash-high",
	"claude-3-7-sonnet-latest":   "gemini-3.6-flash-high",
	"claude-3-7-sonnet-20250219": "gemini-3.6-flash-high",
	"claude-3-opus":              "claude-opus-4-6",
	"claude-3-opus-latest":       "claude-opus-4-6",
	"claude-3-opus-20240229":     "claude-opus-4-6",
	"claude-3-5-haiku":           "gemini-3.5-flash-low",
	"claude-3.5-haiku":           "gemini-3.5-flash-low",
	"claude-3-5-haiku-20241022":  "gemini-3.5-flash-low",
	"claude-3-haiku":             "gemini-3.5-flash-medium",
	"claude-3-haiku-20240307":    "gemini-3.5-flash-medium",
	"claude-sonnet":              "claude-sonnet-4-6",
	"claude-opus":                "claude-opus-4-6",
	"claude-haiku":               "gemini-3.5-flash-low",
	// Retired wire id. The live catalog replaces it with gemini-pro-agent.
	"gemini-3.1-pro-high": "gemini-pro-agent",

	// Gemini 3.7 and 3.8 publish one wire id. The menu level is thinkingConfig.
	"gemini-3.8-flash-low":    "gemini-3.8-flash-tiered",
	"gemini-3.8-flash-medium": "gemini-3.8-flash-tiered",
	"gemini-3.8-flash-high":   "gemini-3.8-flash-tiered",
	"gemini-3.7-flash-low":    "gemini-3.7-flash-tiered",
	"gemini-3.7-flash-medium": "gemini-3.7-flash-tiered",
	"gemini-3.7-flash-high":   "gemini-3.7-flash-tiered",
}

// NormalizeModel resolves model aliases and ensures a valid model identifier for upstream
func NormalizeModel(model string) string {
	m := strings.TrimSpace(strings.ToLower(model))
	if resolved, ok := ModelAliases[m]; ok {
		return resolved
	}
	if isSupportedModel(m) {
		return m
	}

	// Dynamic prefix / keyword heuristics
	if strings.Contains(m, "claude") {
		if strings.Contains(m, "3-7") || strings.Contains(m, "3.7") {
			return "gemini-3.6-flash-high"
		}
		if strings.Contains(m, "opus") {
			return "claude-opus-4-6"
		}
		if strings.Contains(m, "haiku") {
			if strings.Contains(m, "3-5") || strings.Contains(m, "3.5") {
				return "gemini-3.5-flash-low"
			}
			return "gemini-3.5-flash-medium"
		}
		return "claude-sonnet-4-6"
	}
	if strings.Contains(m, "gpt") {
		if strings.Contains(m, "mini") {
			return "gemini-2.5-flash"
		}
		return "gemini-2.5-pro"
	}
	if strings.Contains(m, "pro") {
		return "gemini-2.5-pro"
	}
	if strings.Contains(m, "flash") {
		if strings.Contains(m, "lite") {
			return "gemini-2.5-flash-lite"
		}
		return "gemini-2.5-flash"
	}

	if m != "" {
		return m
	}
	return "gemini-2.5-pro"
}

// UpstreamModel returns the Cloud Code model id and, for tiered Flash, the thinking level.
// Gemini 3.6 keeps Low/Medium/High in the model id. Gemini 3.7 and 3.8 share one id, so the
// suffix or reasoning_effort becomes generationConfig.thinkingConfig.thinkingLevel.
// A bare tiered id leaves the level empty and the upstream default (medium) applies.
func UpstreamModel(model, reasoningEffort string) (string, string) {
	raw := strings.TrimSpace(strings.ToLower(model))
	wire := NormalizeModel(raw)
	if !tieredFlash(wire) {
		return wire, ""
	}
	if level := thinkingLevel(reasoningEffort); level != "" {
		return wire, level
	}
	return wire, thinkingLevel(raw)
}

func tieredFlash(model string) bool {
	return model == "gemini-3.8-flash-tiered" || model == "gemini-3.7-flash-tiered"
}

func thinkingLevel(name string) string {
	switch {
	case strings.HasSuffix(name, "-low"), strings.EqualFold(name, "low"):
		return "LOW"
	case strings.HasSuffix(name, "-medium"), strings.EqualFold(name, "medium"):
		return "MEDIUM"
	case strings.HasSuffix(name, "-high"), strings.EqualFold(name, "high"):
		return "HIGH"
	default:
		return ""
	}
}

func setGenerationConfig(body *CloudCodeRequestBody, maxTokens int, temperature float64, thinkingLevel string) {
	if maxTokens <= 0 && temperature <= 0 && thinkingLevel == "" {
		return
	}
	cfg := &CloudCodeGenerationConfig{
		MaxOutputTokens: maxTokens,
		Temperature:     temperature,
	}
	if thinkingLevel != "" {
		cfg.ThinkingConfig = &CloudCodeThinkingConfig{ThinkingLevel: thinkingLevel}
	}
	body.GenerationConfig = cfg
}

func isSupportedModel(model string) bool {
	for _, id := range SupportedModels {
		if id == model {
			return true
		}
	}
	return false
}

// ListSupportedModels returns the catalog in standard OpenAI ModelList format
func ListSupportedModels() ModelListResponse {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	var models []ModelObject
	for _, m := range SupportedModels {
		models = append(models, ModelObject{
			ID:      m,
			Object:  "model",
			Created: created,
			OwnedBy: "multigravity",
		})
	}
	return ModelListResponse{
		Object: "list",
		Data:   models,
	}
}
