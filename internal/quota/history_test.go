package quota

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupHistoryHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	return home
}

func TestHistorySeriesFiltersAndSummary(t *testing.T) {
	setupHistoryHome(t)
	if err := os.MkdirAll(filepath.Join(os.Getenv("MULTIGRAVITY_HOME"), "work"), 0o755); err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	historyNow = func() time.Time { return base }
	t.Cleanup(func() { historyNow = time.Now })

	snap := &QuotaSummaryResponse{
		Response: QuotaResponse{
			Groups: []QuotaGroup{{
				Buckets: []QuotaBucket{{
					BucketID:          "gemini-5h",
					DisplayName:       "Gemini 5h",
					RemainingFraction: 0.80,
					ResetTime:         base.Add(2 * time.Hour).Format(time.RFC3339),
				}},
			}},
		},
	}
	if err := RecordQuotaSnapshot("work", snap); err != nil {
		t.Fatal(err)
	}
	// Same fractions inside the quiet interval must not append a second point.
	historyNow = func() time.Time { return base.Add(20 * time.Second) }
	if err := RecordQuotaSnapshot("work", snap); err != nil {
		t.Fatal(err)
	}

	historyNow = func() time.Time { return base.Add(2 * time.Minute) }
	snap.Response.Groups[0].Buckets[0].RemainingFraction = 0.55
	if err := RecordQuotaSnapshot("work", snap); err != nil {
		t.Fatal(err)
	}

	historyNow = func() time.Time { return base.Add(3 * time.Minute) }
	if err := RecordTokens("work", SourceGateway, "gemini-2.5-pro", 4, 8, 12, false); err != nil {
		t.Fatal(err)
	}
	if err := RecordTokens("work", SourceHeadless, "", 0, 0, 7, false); err != nil {
		t.Fatal(err)
	}
	if err := RecordTokens("missing-prof", SourceGateway, "gemini-2.5-pro", 1, 1, 2, true); err != nil {
		t.Fatal(err)
	}

	series, err := LoadSeries("work", HistoryQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Samples) != 4 {
		t.Fatalf("expected 4 samples, got %d", len(series.Samples))
	}
	if series.Summary.TotalTokens != 19 || series.Summary.PromptTokens != 4 || series.Summary.CompletionTokens != 8 {
		t.Fatalf("unexpected summary: %+v", series.Summary)
	}
	if len(series.Summary.LatestBuckets) != 1 || series.Summary.LatestBuckets[0].RemainingFraction != 0.55 {
		t.Fatalf("latest buckets: %+v", series.Summary.LatestBuckets)
	}
	if series.Summary.LatestBuckets[0].WindowType != Window5h {
		t.Fatalf("expected 5h window, got %q", series.Summary.LatestBuckets[0].WindowType)
	}

	historyNow = func() time.Time { return base.Add(4 * time.Minute) }
	q, err := ParseHistoryQuery("24h", "", SourceGateway, "10")
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := LoadSeries("work", q)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Samples) != 1 || filtered.Samples[0].Tokens.TotalTokens != 12 {
		t.Fatalf("gateway filter: %+v", filtered.Samples)
	}

	fleet, err := LoadFleetSeries(HistoryQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(fleet.Profiles) != 1 || fleet.Summary.TotalTokens != 19 {
		t.Fatalf("fleet: %+v", fleet)
	}
}

func TestHistoryCompactionDropsExpiredSamples(t *testing.T) {
	setupHistoryHome(t)
	if err := os.MkdirAll(filepath.Join(os.Getenv("MULTIGRAVITY_HOME"), "old"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := historyCompactBytes
	historyCompactBytes = 1
	t.Cleanup(func() { historyCompactBytes = orig })

	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	historyNow = func() time.Time { return old }
	t.Cleanup(func() { historyNow = time.Now })
	if err := RecordTokens("old", SourceGateway, "m", 1, 1, 2, false); err != nil {
		t.Fatal(err)
	}

	recent := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	historyNow = func() time.Time { return recent }
	if err := RecordTokens("old", SourceGateway, "m", 3, 1, 4, true); err != nil {
		t.Fatal(err)
	}

	series, err := LoadSeries("old", HistoryQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Samples) != 1 || series.Samples[0].Tokens.TotalTokens != 4 {
		t.Fatalf("expected only the recent sample, got %+v", series.Samples)
	}
}

func TestParseHistoryQueryRejectsBadInput(t *testing.T) {
	if _, err := ParseHistoryQuery("nope", "", "", ""); err == nil {
		t.Fatal("expected invalid since")
	}
	if _, err := ParseHistoryQuery("", "", "other", ""); err == nil {
		t.Fatal("expected invalid source")
	}
	if _, err := ParseHistoryQuery("", "", "", "0"); err == nil {
		t.Fatal("expected invalid limit")
	}
}

func TestEstimateTokens(t *testing.T) {
	if EstimateTokens("") != 0 {
		t.Fatal("empty text")
	}
	if EstimateTokens("abcd") != 1 {
		t.Fatalf("got %d", EstimateTokens("abcd"))
	}
	if EstimateTokens("abcde") != 2 {
		t.Fatalf("got %d", EstimateTokens("abcde"))
	}
}
