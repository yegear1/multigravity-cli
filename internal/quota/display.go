package quota

import (
	"fmt"
	"io"
	"math"
	"strings"
	"time"
)

func FormatProgressBar(remainingPct float64, barLen int) string {
	filled := int(math.Round(float64(barLen) * (remainingPct / 100.0)))
	if filled < 0 {
		filled = 0
	}
	if filled > barLen {
		filled = barLen
	}
	return "[" + strings.Repeat("=", filled) + strings.Repeat(" ", barLen-filled) + "]"
}

func FormatResetCountdown(resetTimeStr string, now time.Time) (string, int) {
	if resetTimeStr == "" {
		return "", 0
	}

	rt, err := time.Parse(time.RFC3339, resetTimeStr)
	if err != nil {
		// Try parsing ISO8601 without timezone or with +00:00
		rt, err = time.Parse("2006-01-02T15:04:05Z07:00", resetTimeStr)
		if err != nil {
			return " | Reset: " + resetTimeStr, 0
		}
	}

	deltaSec := int(rt.Sub(now).Seconds())
	if deltaSec > 0 {
		hours := deltaSec / 3600
		mins := (deltaSec % 3600) / 60
		return fmt.Sprintf(" | Resets in: %dh %dm", hours, mins), deltaSec
	}
	return " | Quota refreshed!", deltaSec
}

func RenderQuotaStatus(w io.Writer, servers []ActiveServer) {
	now := time.Now().UTC()
	for _, s := range servers {
		fmt.Fprintf(w, "\nQuota Status — Profile: %s (PID %d, Port %d)\n", s.Profile, s.PID, s.Port)
		fmt.Fprintf(w, "%s\n", strings.Repeat("=", 60))

		if s.Data == nil || len(s.Data.Response.Groups) == 0 {
			fmt.Fprintln(w, "  No quota information returned.")
			continue
		}

		for _, g := range s.Data.Response.Groups {
			fmt.Fprintf(w, "\n• %s (%s):\n", g.DisplayName, g.Description)
			for _, b := range g.Buckets {
				remainingPct := math.Round(b.RemainingFraction*1000) / 10
				usedPct := math.Round((100.0-remainingPct)*10) / 10

				timeLeftStr, _ := FormatResetCountdown(b.ResetTime, now)
				bar := FormatProgressBar(remainingPct, 20)

				bName := b.DisplayName
				if bName == "" {
					bName = "Limit"
				}

				fmt.Fprintf(w, "  - %s:\n", bName)
				fmt.Fprintf(w, "    %s Remaining: %.1f%% | Used: %.1f%%%s\n", bar, remainingPct, usedPct, timeLeftStr)
				if b.Description != "" {
					fmt.Fprintf(w, "    Details: %s\n", b.Description)
				}
			}
		}
	}
	fmt.Fprintln(w)
}
