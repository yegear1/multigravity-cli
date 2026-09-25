package quota

import (
	"testing"
	"time"
)

func TestClassifyWindow(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		bucket   QuotaBucket
		expected string
	}{
		{
			name: "Explicit windowType preserved",
			bucket: QuotaBucket{
				BucketID:   "custom-bucket",
				WindowType: Window5h,
			},
			expected: Window5h,
		},
		{
			name: "BucketID with 5h",
			bucket: QuotaBucket{
				BucketID: "gemini-5h",
			},
			expected: Window5h,
		},
		{
			name: "DisplayName with 5-Hour",
			bucket: QuotaBucket{
				BucketID:    "gemini_short",
				DisplayName: "Gemini 5-Hour Window",
			},
			expected: Window5h,
		},
		{
			name: "BucketID with weekly",
			bucket: QuotaBucket{
				BucketID: "gemini-weekly",
			},
			expected: WindowWeekly,
		},
		{
			name: "DisplayName with weekly",
			bucket: QuotaBucket{
				BucketID:    "claude-cap",
				DisplayName: "Claude Weekly Limit",
			},
			expected: WindowWeekly,
		},
		{
			name: "Mathematical delta: 3 hours remaining -> 5h window",
			bucket: QuotaBucket{
				BucketID:  "model-unknown-id",
				ResetTime: now.Add(3 * time.Hour).Format(time.RFC3339),
			},
			expected: Window5h,
		},
		{
			name: "Mathematical delta: 11 hours remaining -> 5h window",
			bucket: QuotaBucket{
				BucketID:  "model-unknown-id",
				ResetTime: now.Add(11 * time.Hour).Format(time.RFC3339),
			},
			expected: Window5h,
		},
		{
			name: "Mathematical delta: 24 hours remaining -> weekly window",
			bucket: QuotaBucket{
				BucketID:  "model-unknown-id",
				ResetTime: now.Add(24 * time.Hour).Format(time.RFC3339),
			},
			expected: WindowWeekly,
		},
		{
			name: "Mathematical delta: 5 days remaining -> weekly window",
			bucket: QuotaBucket{
				BucketID:  "model-unknown-id",
				ResetTime: now.Add(5 * 24 * time.Hour).Format(time.RFC3339),
			},
			expected: WindowWeekly,
		},
		{
			name: "Fallback 3p-5h",
			bucket: QuotaBucket{
				BucketID: "3p-5h",
			},
			expected: Window5h,
		},
		{
			name: "Fallback 3p-weekly",
			bucket: QuotaBucket{
				BucketID: "3p-weekly",
			},
			expected: WindowWeekly,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyWindow(tc.bucket, now)
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestCanWarm5hWindow(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

	parentHealthy := &QuotaBucket{
		BucketID:          "gemini-weekly",
		RemainingFraction: 0.80,
	}

	parentExhausted := &QuotaBucket{
		BucketID:          "gemini-weekly",
		RemainingFraction: 0.02,
	}

	// 1. Refreshed 5h bucket with healthy parent -> Can warm
	b1 := QuotaBucket{
		BucketID:          "gemini-5h",
		RemainingFraction: 1.0,
		ResetTime:         "",
	}
	if !CanWarm5hWindow(b1, parentHealthy, now) {
		t.Errorf("expected b1 to be warmable")
	}

	// 2. Refreshed 5h bucket with exhausted weekly parent -> Should NOT warm
	if CanWarm5hWindow(b1, parentExhausted, now) {
		t.Errorf("expected b1 NOT to be warmable when weekly quota is exhausted")
	}

	// 3. 5h bucket already active (consumed and countdown running) -> Should NOT warm
	bActive := QuotaBucket{
		BucketID:          "gemini-5h",
		RemainingFraction: 0.60,
		ResetTime:         now.Add(2 * time.Hour).Format(time.RFC3339),
	}
	if CanWarm5hWindow(bActive, parentHealthy, now) {
		t.Errorf("expected bActive NOT to be warmable when countdown is active")
	}

	// 4. Weekly bucket -> Should NOT warm (only 5h windows are warmable)
	bWeekly := QuotaBucket{
		BucketID:          "gemini-weekly",
		RemainingFraction: 1.0,
	}
	if CanWarm5hWindow(bWeekly, parentHealthy, now) {
		t.Errorf("expected weekly bucket NOT to be eligible for 5h warm-up")
	}
}
