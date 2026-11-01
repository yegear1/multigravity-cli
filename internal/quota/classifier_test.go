package quota

import (
	"testing"
)

func TestHasLineRangedRead(t *testing.T) {
	tests := []struct {
		name     string
		entries  []string
		expected bool
	}{
		{
			name:     "empty slice",
			entries:  []string{},
			expected: false,
		},
		{
			name:     "plain file without line suffix",
			entries:  []string{"internal/quota/routing.go", "README.md"},
			expected: false,
		},
		{
			name:     "single line suffix",
			entries:  []string{"internal/quota/routing.go:10"},
			expected: true,
		},
		{
			name:     "range suffix",
			entries:  []string{"internal/quota/routing.go:10-42"},
			expected: true,
		},
		{
			name:     "range suffix with whitespace",
			entries:  []string{"  internal/quota/routing.go:10-42  "},
			expected: true,
		},
		{
			name:     "mixed list containing at least one ranged read",
			entries:  []string{"pkg/foo.go", "internal/quota/routing.go:15-30"},
			expected: true,
		},
		{
			name:     "colon without line number",
			entries:  []string{"internal/quota/routing.go:"},
			expected: false,
		},
		{
			name:     "colon with alpha string",
			entries:  []string{"internal/quota/routing.go:main"},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := HasLineRangedRead(tc.entries)
			if actual != tc.expected {
				t.Fatalf("expected HasLineRangedRead(%v) = %v, got %v", tc.entries, tc.expected, actual)
			}
		})
	}
}

func TestClassifyTaskComplexity(t *testing.T) {
	t.Run("rule a: is_retry takes precedence", func(t *testing.T) {
		input := TaskComplexityInput{
			Prompt:  "fix typo in documentation",
			Paths:   []string{"README.md"},
			Read:    []string{"README.md:1-10"},
			IsRetry: true,
		}
		profile := ClassifyTaskComplexity(input)
		if profile.Tier != 2 || profile.TierName != "reasoning" || profile.Weight != WeightHeavy ||
			profile.RecommendedModel != "gemini-4-argon-medium" || profile.RecommendedEffort != "medium" ||
			profile.Reason != "retry escalation" {
			t.Fatalf("unexpected profile for retry: %+v", profile)
		}
	})

	t.Run("rule b: architecture keyword matching", func(t *testing.T) {
		keywords := []string{
			"race condition",
			"deadlock",
			"heisenbug",
			"memory leak",
			"concurrency",
			"architecture",
			"multi-repo",
		}
		for _, kw := range keywords {
			input := TaskComplexityInput{
				Prompt: "Investigating " + kw + " in session store",
				Paths:  []string{"session.go"},
			}
			profile := ClassifyTaskComplexity(input)
			if profile.Tier != 2 || profile.TierName != "reasoning" || profile.Weight != WeightHeavy ||
				profile.RecommendedModel != "gemini-4-argon-medium" || profile.RecommendedEffort != "high" ||
				profile.Reason != "complex reasoning or >3 paths" {
				t.Fatalf("expected reasoning profile for keyword %q, got: %+v", kw, profile)
			}
		}
	})

	t.Run("rule b: paths blast radius > 3", func(t *testing.T) {
		input := TaskComplexityInput{
			Prompt: "refactor service boundaries",
			Paths:  []string{"a.go", "b.go", "c.go", "d.go"},
		}
		profile := ClassifyTaskComplexity(input)
		if profile.Tier != 2 || profile.TierName != "reasoning" || profile.Weight != WeightHeavy ||
			profile.RecommendedModel != "gemini-4-argon-medium" || profile.RecommendedEffort != "high" ||
			profile.Reason != "complex reasoning or >3 paths" {
			t.Fatalf("unexpected profile for >3 paths: %+v", profile)
		}
	})

	t.Run("rule c: single path with line ranged read", func(t *testing.T) {
		input := TaskComplexityInput{
			Prompt: "update constant definition",
			Paths:  []string{"internal/quota/routing.go"},
			Read:   []string{"internal/quota/routing.go:10-42"},
		}
		profile := ClassifyTaskComplexity(input)
		if profile.Tier != 0 || profile.TierName != "micro" || profile.Weight != WeightLight ||
			profile.RecommendedModel != "gemini-3.8-flash-low" || profile.RecommendedEffort != "low" ||
			profile.Reason != "scoped single-file or micro task" {
			t.Fatalf("unexpected profile for single path with line ranged read: %+v", profile)
		}
	})

	t.Run("rule c: micro keywords with <= 1 path", func(t *testing.T) {
		microKeywords := []string{
			"typo",
			"readme",
			"comment",
			"lint",
			"format",
			"rename",
			"docstring",
		}
		for _, kw := range microKeywords {
			// Zero paths case
			input0 := TaskComplexityInput{
				Prompt: "Fix " + kw + " please",
				Paths:  []string{},
			}
			profile0 := ClassifyTaskComplexity(input0)
			if profile0.Tier != 0 || profile0.TierName != "micro" || profile0.Weight != WeightLight ||
				profile0.RecommendedModel != "gemini-3.8-flash-low" || profile0.RecommendedEffort != "low" ||
				profile0.Reason != "scoped single-file or micro task" {
				t.Fatalf("expected micro profile for kw %q with 0 paths, got: %+v", kw, profile0)
			}

			// 1 path case
			input1 := TaskComplexityInput{
				Prompt: "Address " + kw + " issue",
				Paths:  []string{"file.go"},
			}
			profile1 := ClassifyTaskComplexity(input1)
			if profile1.Tier != 0 || profile1.TierName != "micro" || profile1.Weight != WeightLight ||
				profile1.RecommendedModel != "gemini-3.8-flash-low" || profile1.RecommendedEffort != "low" ||
				profile1.Reason != "scoped single-file or micro task" {
				t.Fatalf("expected micro profile for kw %q with 1 path, got: %+v", kw, profile1)
			}
		}
	})

	t.Run("rule d: bounded multi-file task (1 <= len(Paths) <= 3)", func(t *testing.T) {
		// 1 path without micro keywords or line ranged read
		input1 := TaskComplexityInput{
			Prompt: "implement user cache handler",
			Paths:  []string{"handler.go"},
		}
		profile1 := ClassifyTaskComplexity(input1)
		if profile1.Tier != 1 || profile1.TierName != "standard" || profile1.Weight != WeightMedium ||
			profile1.RecommendedModel != "gemini-3.8-flash-medium" || profile1.RecommendedEffort != "medium" ||
			profile1.Reason != "bounded multi-file task" {
			t.Fatalf("unexpected profile for 1 path: %+v", profile1)
		}

		// 2 paths
		input2 := TaskComplexityInput{
			Prompt: "sync user status across services",
			Paths:  []string{"user.go", "status.go"},
		}
		profile2 := ClassifyTaskComplexity(input2)
		if profile2.Tier != 1 || profile2.TierName != "standard" || profile2.Weight != WeightMedium ||
			profile2.RecommendedModel != "gemini-3.8-flash-medium" || profile2.RecommendedEffort != "medium" ||
			profile2.Reason != "bounded multi-file task" {
			t.Fatalf("unexpected profile for 2 paths: %+v", profile2)
		}

		// 3 paths
		input3 := TaskComplexityInput{
			Prompt: "wire up database repository",
			Paths:  []string{"repo.go", "model.go", "conn.go"},
		}
		profile3 := ClassifyTaskComplexity(input3)
		if profile3.Tier != 1 || profile3.TierName != "standard" || profile3.Weight != WeightMedium ||
			profile3.RecommendedModel != "gemini-3.8-flash-medium" || profile3.RecommendedEffort != "medium" ||
			profile3.Reason != "bounded multi-file task" {
			t.Fatalf("unexpected profile for 3 paths: %+v", profile3)
		}
	})

	t.Run("rule e: unbounded/exploratory task with 0 paths and no micro keywords", func(t *testing.T) {
		input := TaskComplexityInput{
			Prompt: "investigate why system throughput drops at peak hours",
			Paths:  []string{},
		}
		profile := ClassifyTaskComplexity(input)
		if profile.Tier != 1 || profile.TierName != "standard" || profile.Weight != WeightMedium ||
			profile.RecommendedModel != "gemini-3.8-flash-medium" || profile.RecommendedEffort != "medium" ||
			profile.Reason != "default standard task" {
			t.Fatalf("unexpected profile for exploratory task: %+v", profile)
		}
	})
}
