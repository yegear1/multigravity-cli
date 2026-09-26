package quota

import (
	"strings"
	"time"
)

const (
	Window5h            = "5h"
	WindowWeekly        = "weekly"
	WindowUnknown       = "unknown"
	MinWeeklyResetHours = 12.0 // Optimal threshold: resets > 12h are guaranteed weekly; <= 12h are 5h rolling windows
	// MinRemainingFraction is the depleted-quota guard. Warm-up refuses a 5h prime
	// at or below this fraction, and quota alerts use the same line.
	MinRemainingFraction = 0.05
)

// ClassifyWindow determines if a bucket represents a 5-hour rolling window or a weekly window
func ClassifyWindow(b QuotaBucket, now time.Time) string {
	// If explicit windowType is already assigned, preserve it
	if b.WindowType == Window5h || b.WindowType == WindowWeekly {
		return b.WindowType
	}

	bID := strings.ToLower(b.BucketID)
	dName := strings.ToLower(b.DisplayName)

	// 1. Textual heuristics on ID or Display Name
	if strings.Contains(bID, "5h") || strings.Contains(bID, "5-hour") || strings.Contains(bID, "5_hour") ||
		strings.Contains(dName, "5-hour") || strings.Contains(dName, "5 hour") || strings.Contains(dName, "5h") ||
		strings.Contains(bID, "rolling") || strings.Contains(dName, "rolling") ||
		strings.Contains(bID, "sliding") || strings.Contains(dName, "sliding") ||
		strings.Contains(bID, "hourly") || strings.Contains(dName, "hourly") {
		return Window5h
	}

	if strings.Contains(bID, "weekly") || strings.Contains(dName, "weekly") ||
		strings.Contains(bID, "week") || strings.Contains(dName, "week") ||
		strings.Contains(bID, "7d") || strings.Contains(dName, "7-day") ||
		strings.Contains(dName, "tier") {
		return WindowWeekly
	}

	// 2. Mathematical time delta heuristic
	if b.ResetTime != "" {
		rt, err := time.Parse(time.RFC3339, b.ResetTime)
		if err != nil {
			rt, err = time.Parse("2006-01-02T15:04:05Z07:00", b.ResetTime)
		}

		if err == nil && rt.After(now) {
			deltaHours := rt.Sub(now).Hours()
			if deltaHours > MinWeeklyResetHours {
				return WindowWeekly
			}
			if deltaHours > 0 {
				return Window5h
			}
		}
	}

	// 3. Fallback: if empty reset or refreshed
	if bID == "gemini-5h" || bID == "3p-5h" {
		return Window5h
	}
	if bID == "gemini-weekly" || bID == "3p-weekly" {
		return WindowWeekly
	}

	return WindowUnknown
}

// CanWarm5hWindow checks whether a 5-hour rolling window is eligible for proactive warm-up
func CanWarm5hWindow(b QuotaBucket, parentWeekly *QuotaBucket, now time.Time) bool {
	wType := ClassifyWindow(b, now)
	if wType != Window5h {
		return false
	}

	// If countdown is active and quota was consumed, clock is already running
	_, deltaSec := FormatResetCountdown(b.ResetTime, now)
	if deltaSec > 0 && b.RemainingFraction < 0.999 {
		return false
	}

	// Quota must be refreshed / full (100% or deltaSec <= 0)
	isRefreshed := (b.RemainingFraction >= 0.999) || (deltaSec <= 0)
	if !isRefreshed {
		return false
	}

	// Safety Guard: weekly parent quota must have remaining budget (> 5%)
	if parentWeekly != nil && parentWeekly.RemainingFraction <= MinRemainingFraction {
		return false
	}

	return true
}
