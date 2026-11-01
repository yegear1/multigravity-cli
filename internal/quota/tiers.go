package quota

import (
	"strings"
)

// InferenceTier represents the compute / reasoning capability tier of a model.
type InferenceTier int

const (
	TierMicro        InferenceTier = 0
	TierStandard     InferenceTier = 1
	TierReasoning    InferenceTier = 2
	TierArchitecture InferenceTier = 3
)

// String returns the string representation of the inference tier.
func (t InferenceTier) String() string {
	switch t {
	case TierMicro:
		return "micro"
	case TierStandard:
		return "standard"
	case TierReasoning:
		return "reasoning"
	case TierArchitecture:
		return "architecture"
	default:
		return "standard"
	}
}

// ModelTierSpec describes the tier properties, effort level, and fallback targets of a model.
type ModelTierSpec struct {
	Model         string        `json:"model"`
	Tier          InferenceTier `json:"tier"`
	TierName      string        `json:"tier_name"`
	Effort        string        `json:"effort"`
	BucketGroup   string        `json:"bucket_group"`
	FallbackModel string        `json:"fallback_model,omitempty"`
}

// DefaultModelForTier returns the recommended default model for a given inference tier.
func DefaultModelForTier(t InferenceTier) string {
	switch t {
	case TierMicro:
		return "gemini-3.8-flash-low"
	case TierStandard:
		return "gemini-3.8-flash-medium"
	case TierReasoning:
		return "gemini-4-argon-medium"
	case TierArchitecture:
		return "claude-3-opus"
	default:
		return "gemini-3.8-flash-medium"
	}
}

// TierForModel classifies a model identifier into its corresponding InferenceTier.
func TierForModel(model string) InferenceTier {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" || m == "gemini-3.8-flash-low" || strings.Contains(m, "haiku") || strings.Contains(m, "gpt-4o-mini") || (strings.HasSuffix(m, "-low") && !strings.Contains(m, "argon") && !strings.Contains(m, "opus")) {
		return TierMicro
	}
	if strings.Contains(m, "opus") || strings.Contains(m, "gemini-2.5-pro") {
		return TierArchitecture
	}
	if strings.Contains(m, "argon") || strings.Contains(m, "claude-3.7-sonnet") || strings.Contains(m, "claude-3-7-sonnet") || strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3") {
		return TierReasoning
	}
	return TierStandard
}

// EscalateModel determines the next higher-capability model for a given model.
func EscalateModel(model string) (nextModel string, escalated bool) {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "gemini-4-argon-low" {
		return "gemini-4-argon-medium", true
	}
	if m == "gemini-4-argon-medium" {
		return "gemini-4-argon-high", true
	}
	tier := TierForModel(m)
	switch tier {
	case TierMicro:
		return "gemini-3.8-flash-medium", true
	case TierStandard:
		return "gemini-4-argon-medium", true
	default:
		return m, false
	}
}

// DescribeModelTier builds a ModelTierSpec for the specified model.
func DescribeModelTier(model string) ModelTierSpec {
	m := strings.ToLower(strings.TrimSpace(model))
	tier := TierForModel(m)

	effort := "medium"
	if tier == TierMicro || strings.HasSuffix(m, "-low") {
		effort = "low"
	} else if tier == TierReasoning || tier == TierArchitecture || strings.HasSuffix(m, "-high") {
		effort = "high"
	}

	spec := ModelTierSpec{
		Model:       m,
		Tier:        tier,
		TierName:    tier.String(),
		Effort:      effort,
		BucketGroup: BucketGroupForModel(m),
	}

	if nextModel, escalated := EscalateModel(m); escalated {
		spec.FallbackModel = nextModel
	}

	return spec
}
