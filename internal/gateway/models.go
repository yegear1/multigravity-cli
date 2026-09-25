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
	"gemini-3.1-pro-high",
	"claude-sonnet-4-6",
	"claude-opus-4-6",
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
	"gpt-4o":                 "gemini-2.5-pro",
	"gpt-4o-mini":            "gemini-2.5-flash",
	"gpt-4-turbo":            "gemini-2.5-pro",
	"gpt-4":                  "gemini-2.5-pro",
	"gpt-3.5-turbo":          "gemini-2.5-flash",

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
}

// NormalizeModel resolves model aliases and ensures a valid model identifier for upstream
func NormalizeModel(model string) string {
	m := strings.TrimSpace(strings.ToLower(model))
	if resolved, ok := ModelAliases[m]; ok {
		return resolved
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
