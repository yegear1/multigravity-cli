package quota

import (
	"testing"
)

func TestTierForModel(t *testing.T) {
	tests := []struct {
		model    string
		expected InferenceTier
	}{
		// TierMicro cases
		{"", TierMicro},
		{"   ", TierMicro},
		{"gemini-3.8-flash-low", TierMicro},
		{"GEMINI-3.8-FLASH-LOW", TierMicro},
		{"claude-3-haiku-20240307", TierMicro},
		{"gpt-4o-mini", TierMicro},
		{"custom-agent-low", TierMicro},

		// Argon and Opus with -low suffix should NOT be TierMicro
		{"gemini-4-argon-low", TierReasoning},
		{"claude-3-opus-low", TierArchitecture},

		// TierArchitecture cases
		{"claude-3-opus", TierArchitecture},
		{"CLAUDE-3-OPUS", TierArchitecture},
		{"gemini-2.5-pro", TierArchitecture},
		{"gemini-2.5-pro-preview", TierArchitecture},

		// TierReasoning cases
		{"gemini-4-argon-medium", TierReasoning},
		{"gemini-4-argon-high", TierReasoning},
		{"claude-3.7-sonnet", TierReasoning},
		{"claude-3-7-sonnet", TierReasoning},
		{"o1-preview", TierReasoning},
		{"o1-mini", TierReasoning},
		{"o3-mini", TierReasoning},

		// TierStandard cases (default / others)
		{"gemini-3.8-flash-medium", TierStandard},
		{"gemini-3.8-flash-high", TierStandard},
		{"gpt-4o", TierStandard},
		{"claude-3-5-sonnet", TierStandard},
		{"unknown-model", TierStandard},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := TierForModel(tt.model)
			if got != tt.expected {
				t.Errorf("TierForModel(%q) = %v (%s), expected %v (%s)",
					tt.model, got, got.String(), tt.expected, tt.expected.String())
			}
		})
	}
}

func TestDefaultModelForTier(t *testing.T) {
	tests := []struct {
		tier     InferenceTier
		expected string
	}{
		{TierMicro, "gemini-3.8-flash-low"},
		{TierStandard, "gemini-3.8-flash-medium"},
		{TierReasoning, "gemini-4-argon-medium"},
		{TierArchitecture, "claude-3-opus"},
		{InferenceTier(999), "gemini-3.8-flash-medium"},
	}

	for _, tt := range tests {
		t.Run(tt.tier.String(), func(t *testing.T) {
			got := DefaultModelForTier(tt.tier)
			if got != tt.expected {
				t.Errorf("DefaultModelForTier(%v) = %q, expected %q", tt.tier, got, tt.expected)
			}
		})
	}

	// Also test String() mappings
	if TierMicro.String() != "micro" {
		t.Errorf("TierMicro.String() = %q, want 'micro'", TierMicro.String())
	}
	if TierStandard.String() != "standard" {
		t.Errorf("TierStandard.String() = %q, want 'standard'", TierStandard.String())
	}
	if TierReasoning.String() != "reasoning" {
		t.Errorf("TierReasoning.String() = %q, want 'reasoning'", TierReasoning.String())
	}
	if TierArchitecture.String() != "architecture" {
		t.Errorf("TierArchitecture.String() = %q, want 'architecture'", TierArchitecture.String())
	}
	if InferenceTier(999).String() != "standard" {
		t.Errorf("InferenceTier(999).String() = %q, want 'standard'", InferenceTier(999).String())
	}
}

func TestEscalateModel(t *testing.T) {
	tests := []struct {
		model         string
		expectedNext  string
		expectedValid bool
	}{
		{"", "gemini-3.8-flash-medium", true},
		{"   ", "gemini-3.8-flash-medium", true},
		{"gemini-3.8-flash-low", "gemini-3.8-flash-medium", true},
		{"claude-3-haiku", "gemini-3.8-flash-medium", true},
		{"gpt-4o-mini", "gemini-3.8-flash-medium", true},
		{"custom-low", "gemini-3.8-flash-medium", true},

		{"gemini-3.8-flash-medium", "gemini-4-argon-medium", true},
		{"gemini-3.8-flash-high", "gemini-4-argon-medium", true},
		{"gpt-4o", "gemini-4-argon-medium", true},
		{"claude-3-5-sonnet", "gemini-4-argon-medium", true},

		{"gemini-4-argon-low", "gemini-4-argon-medium", true},
		{"gemini-4-argon-medium", "gemini-4-argon-high", true},

		{"gemini-4-argon-high", "gemini-4-argon-high", false},
		{"claude-3-opus", "claude-3-opus", false},
		{"CLAUDE-3-OPUS", "claude-3-opus", false},
		{"claude-3.7-sonnet", "claude-3.7-sonnet", false},
		{"o1-preview", "o1-preview", false},
		{"gemini-2.5-pro", "gemini-2.5-pro", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			gotNext, gotValid := EscalateModel(tt.model)
			if gotNext != tt.expectedNext || gotValid != tt.expectedValid {
				t.Errorf("EscalateModel(%q) = (%q, %v), expected (%q, %v)",
					tt.model, gotNext, gotValid, tt.expectedNext, tt.expectedValid)
			}
		})
	}
}

func TestDescribeModelTier(t *testing.T) {
	tests := []struct {
		input       string
		expected    ModelTierSpec
	}{
		{
			input: "gemini-3.8-flash-low",
			expected: ModelTierSpec{
				Model:         "gemini-3.8-flash-low",
				Tier:          TierMicro,
				TierName:      "micro",
				Effort:        "low",
				BucketGroup:   "gemini",
				FallbackModel: "gemini-3.8-flash-medium",
			},
		},
		{
			input: "gemini-3.8-flash-medium",
			expected: ModelTierSpec{
				Model:         "gemini-3.8-flash-medium",
				Tier:          TierStandard,
				TierName:      "standard",
				Effort:        "medium",
				BucketGroup:   "gemini",
				FallbackModel: "gemini-4-argon-medium",
			},
		},
		{
			input: "gemini-3.8-flash-high",
			expected: ModelTierSpec{
				Model:         "gemini-3.8-flash-high",
				Tier:          TierStandard,
				TierName:      "standard",
				Effort:        "high",
				BucketGroup:   "gemini",
				FallbackModel: "gemini-4-argon-medium",
			},
		},
		{
			input: "gemini-4-argon-low",
			expected: ModelTierSpec{
				Model:         "gemini-4-argon-low",
				Tier:          TierReasoning,
				TierName:      "reasoning",
				Effort:        "low",
				BucketGroup:   "argon",
				FallbackModel: "gemini-4-argon-medium",
			},
		},
		{
			input: "gemini-4-argon-medium",
			expected: ModelTierSpec{
				Model:         "gemini-4-argon-medium",
				Tier:          TierReasoning,
				TierName:      "reasoning",
				Effort:        "high",
				BucketGroup:   "argon",
				FallbackModel: "gemini-4-argon-high",
			},
		},
		{
			input: "gemini-4-argon-high",
			expected: ModelTierSpec{
				Model:         "gemini-4-argon-high",
				Tier:          TierReasoning,
				TierName:      "reasoning",
				Effort:        "high",
				BucketGroup:   "argon",
				FallbackModel: "",
			},
		},
		{
			input: "claude-3-opus",
			expected: ModelTierSpec{
				Model:         "claude-3-opus",
				Tier:          TierArchitecture,
				TierName:      "architecture",
				Effort:        "high",
				BucketGroup:   "3p",
				FallbackModel: "",
			},
		},
		{
			input: "claude-3-haiku",
			expected: ModelTierSpec{
				Model:         "claude-3-haiku",
				Tier:          TierMicro,
				TierName:      "micro",
				Effort:        "low",
				BucketGroup:   "3p",
				FallbackModel: "gemini-3.8-flash-medium",
			},
		},
		{
			input: "gpt-4o",
			expected: ModelTierSpec{
				Model:         "gpt-4o",
				Tier:          TierStandard,
				TierName:      "standard",
				Effort:        "medium",
				BucketGroup:   "3p",
				FallbackModel: "gemini-4-argon-medium",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := DescribeModelTier(tt.input)
			if got != tt.expected {
				t.Errorf("DescribeModelTier(%q) = %+v, expected %+v", tt.input, got, tt.expected)
			}
		})
	}
}
