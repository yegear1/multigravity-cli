package quota

import (
	"regexp"
	"strings"
)

var lineRangeRegex = regexp.MustCompile(`:\d+(?:-\d+)?$`)

var architectureKeywords = []string{
	"race condition",
	"deadlock",
	"heisenbug",
	"memory leak",
	"concurrency",
	"architecture",
	"multi-repo",
}

var microKeywords = []string{
	"typo",
	"readme",
	"comment",
	"lint",
	"format",
	"rename",
	"docstring",
}

// TaskComplexityInput represents the input task attributes used to determine execution complexity.
type TaskComplexityInput struct {
	Prompt  string   `json:"prompt,omitempty"`
	Paths   []string `json:"paths,omitempty"`
	Read    []string `json:"read,omitempty"`
	IsRetry bool     `json:"is_retry,omitempty"`
}

// ComplexityProfile holds the classified tier, routing weight, model recommendations, and rationale.
type ComplexityProfile struct {
	Weight            RouteWeight `json:"weight"`
	RecommendedModel  string      `json:"recommended_model"`
	RecommendedEffort string      `json:"recommended_effort"`
	Tier              int         `json:"tier"`
	TierName          string      `json:"tier_name"`
	Reason            string      `json:"reason"`
}

// HasLineRangedRead returns true if any entry in readEntries contains a :<start>-<end> or :<line> suffix.
func HasLineRangedRead(readEntries []string) bool {
	for _, entry := range readEntries {
		if lineRangeRegex.MatchString(strings.TrimSpace(entry)) {
			return true
		}
	}
	return false
}

func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// ClassifyTaskComplexity categorizes task complexity to optimize model routing and quota allocation.
func ClassifyTaskComplexity(input TaskComplexityInput) ComplexityProfile {
	promptLower := strings.ToLower(input.Prompt)

	// a) Retry escalation takes top priority
	if input.IsRetry {
		return ComplexityProfile{
			Tier:              2,
			TierName:          "reasoning",
			Weight:            WeightHeavy,
			RecommendedModel:  "gemini-4-argon-medium",
			RecommendedEffort: "medium",
			Reason:            "retry escalation",
		}
	}

	// b) Architecture / deep reasoning or wide path blast radius (> 3 paths)
	if containsAny(promptLower, architectureKeywords) || len(input.Paths) > 3 {
		return ComplexityProfile{
			Tier:              2,
			TierName:          "reasoning",
			Weight:            WeightHeavy,
			RecommendedModel:  "gemini-4-argon-medium",
			RecommendedEffort: "high",
			Reason:            "complex reasoning or >3 paths",
		}
	}

	// c) Scoped single-file with line ranged read OR micro/trivial keyword with <= 1 path
	isSinglePathWithLineRangedRead := len(input.Paths) == 1 && HasLineRangedRead(input.Read)
	isMicroKeywordMatch := len(input.Paths) <= 1 && containsAny(promptLower, microKeywords)
	if isSinglePathWithLineRangedRead || isMicroKeywordMatch {
		return ComplexityProfile{
			Tier:              0,
			TierName:          "micro",
			Weight:            WeightLight,
			RecommendedModel:  "gemini-3.8-flash-low",
			RecommendedEffort: "low",
			Reason:            "scoped single-file or micro task",
		}
	}

	// d) Bounded multi-file task (1 <= len(Paths) <= 3)
	if len(input.Paths) >= 1 && len(input.Paths) <= 3 {
		return ComplexityProfile{
			Tier:              1,
			TierName:          "standard",
			Weight:            WeightMedium,
			RecommendedModel:  "gemini-3.8-flash-medium",
			RecommendedEffort: "medium",
			Reason:            "bounded multi-file task",
		}
	}

	// e) Default standard task (len(Paths) == 0 unbounded/exploratory task without micro keywords)
	return ComplexityProfile{
		Tier:              1,
		TierName:          "standard",
		Weight:            WeightMedium,
		RecommendedModel:  "gemini-3.8-flash-medium",
		RecommendedEffort: "medium",
		Reason:            "default standard task",
	}
}
