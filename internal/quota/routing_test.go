package quota

import "testing"
import "time"

func TestRoutingFractionUsesTightestWindow(t *testing.T) {
	now := time.Date(2026, 9, 27, 3, 0, 0, 0, time.UTC)
	data := &QuotaSummaryResponse{
		Response: QuotaResponse{
			Groups: []QuotaGroup{{
				Buckets: []QuotaBucket{
					{BucketID: "gemini-weekly", RemainingFraction: 0.40, ResetTime: now.Add(48 * time.Hour).Format(time.RFC3339)},
					{BucketID: "gemini-5h", RemainingFraction: 0.15, ResetTime: now.Add(2 * time.Hour).Format(time.RFC3339)},
					{BucketID: "other", RemainingFraction: 0.01, ResetTime: ""},
				},
			}},
		},
	}

	fraction, reset, ok := RoutingFraction(data, now)
	if !ok || fraction != 0.15 || reset.IsZero() {
		t.Fatalf("fraction=%v reset=%v ok=%v", fraction, reset, ok)
	}

	if _, _, ok := RoutingFraction(&QuotaSummaryResponse{}, now); ok {
		t.Fatal("empty summary reported a quota")
	}
}
