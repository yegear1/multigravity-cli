package quota

import "time"

// RoutingFraction returns the tightest live quota among weekly and 5h buckets.
// Buckets that classify as neither window are ignored. ok is false when no
// scored bucket is present, so callers can keep the profile quota unknown.
func RoutingFraction(data *QuotaSummaryResponse, now time.Time) (fraction float64, reset time.Time, ok bool) {
	if data == nil {
		return 0, time.Time{}, false
	}

	var best *QuotaBucket
	for _, group := range data.Response.Groups {
		for i := range group.Buckets {
			bucket := group.Buckets[i]
			window := ClassifyWindow(bucket, now)
			if window != WindowWeekly && window != Window5h {
				continue
			}
			if best == nil || bucket.RemainingFraction < best.RemainingFraction {
				copied := bucket
				best = &copied
			}
		}
	}
	if best == nil {
		return 0, time.Time{}, false
	}

	reset, _ = time.Parse(time.RFC3339, best.ResetTime)
	if reset.IsZero() {
		reset, _ = time.Parse(time.RFC3339Nano, best.ResetTime)
	}
	return best.RemainingFraction, reset, true
}
