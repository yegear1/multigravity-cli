package quota

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

const (
	SourceQuota    = "quota"
	SourceGateway  = "gateway"
	SourceHeadless = "headless"

	historyFileName       = "quota-history.jsonl"
	maxHistorySamples     = 2000
	maxHistoryAge         = 30 * 24 * time.Hour
	quotaSnapshotInterval = time.Minute
)

// historyCompactBytes triggers a rewrite that drops expired samples and
// keeps the newest maxHistorySamples. Tests set this to 1 to force compaction.
var historyCompactBytes int64 = 768 * 1024

var historyNow = time.Now

// BucketPoint is one quota bucket at a single instant.
type BucketPoint struct {
	BucketID          string  `json:"bucket_id"`
	DisplayName       string  `json:"display_name,omitempty"`
	WindowType        string  `json:"window_type,omitempty"`
	RemainingFraction float64 `json:"remaining_fraction"`
	ResetTime         string  `json:"reset_time,omitempty"`
}

// TokenPoint is token consumption attributed to one request.
type TokenPoint struct {
	Model            string `json:"model,omitempty"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	Estimated        bool   `json:"estimated,omitempty"`
}

// Sample is one row in the per-profile quota and token series.
type Sample struct {
	Timestamp time.Time     `json:"timestamp"`
	Profile   string        `json:"profile"`
	Source    string        `json:"source"`
	Buckets   []BucketPoint `json:"buckets,omitempty"`
	Tokens    *TokenPoint   `json:"tokens,omitempty"`
}

// HistoryQuery filters the stored series.
type HistoryQuery struct {
	Since  time.Time
	Until  time.Time
	Source string
	Limit  int
}

// HistorySummary aggregates the samples returned for a window.
type HistorySummary struct {
	Samples          int           `json:"samples"`
	PromptTokens     int           `json:"prompt_tokens"`
	CompletionTokens int           `json:"completion_tokens"`
	TotalTokens      int           `json:"total_tokens"`
	FirstTimestamp   string        `json:"first_timestamp,omitempty"`
	LastTimestamp    string        `json:"last_timestamp,omitempty"`
	LatestBuckets    []BucketPoint `json:"latest_buckets,omitempty"`
}

// Series is the time series for one profile.
type Series struct {
	Profile string         `json:"profile"`
	Samples []Sample       `json:"samples"`
	Summary HistorySummary `json:"summary"`
}

// FleetSeries is the dashboard payload across profiles.
type FleetSeries struct {
	Profiles []Series       `json:"profiles"`
	Summary  HistorySummary `json:"summary"`
}

// EstimateTokens approximates a token count from text length (about 4 runes per token).
func EstimateTokens(text string) int {
	n := utf8.RuneCountInString(text)
	if n == 0 {
		return 0
	}
	return (n + 3) / 4
}

// RecordTokens appends a token sample. A missing profile is ignored so callers
// that run without a real profile directory do not create one.
func RecordTokens(profileName, source, model string, prompt, completion, total int, estimated bool) error {
	if total <= 0 {
		total = prompt + completion
	}
	if total <= 0 {
		return nil
	}
	return appendSample(profileName, Sample{
		Timestamp: historyNow().UTC(),
		Profile:   profileName,
		Source:    source,
		Tokens: &TokenPoint{
			Model:            model,
			PromptTokens:     prompt,
			CompletionTokens: completion,
			TotalTokens:      total,
			Estimated:        estimated,
		},
	})
}

// RecordQuotaSnapshot appends the live bucket fractions for a profile.
// Identical fractions inside quotaSnapshotInterval are not stored again.
func RecordQuotaSnapshot(profileName string, data *QuotaSummaryResponse) error {
	if data == nil {
		return nil
	}
	points := bucketsFromSummary(data)
	if len(points) == 0 {
		return nil
	}
	return appendSample(profileName, Sample{
		Timestamp: historyNow().UTC(),
		Profile:   profileName,
		Source:    SourceQuota,
		Buckets:   points,
	})
}

// RecordLiveSnapshots stores a quota point for every server that returned data.
func RecordLiveSnapshots(servers []ActiveServer) {
	for _, srv := range servers {
		if srv.Profile == "" || srv.Data == nil {
			continue
		}
		_ = RecordQuotaSnapshot(srv.Profile, srv.Data)
	}
}

// LoadSeries reads one profile's history.
func LoadSeries(profileName string, q HistoryQuery) (*Series, error) {
	if err := config.ValidateProfileName(profileName); err != nil {
		return nil, err
	}
	if !profile.ProfileExists(profileName) {
		return nil, fmt.Errorf("profile %q does not exist", profileName)
	}
	samples, err := readSamples(profileName, q)
	if err != nil {
		return nil, err
	}
	return &Series{
		Profile: profileName,
		Samples: samples,
		Summary: summarize(samples),
	}, nil
}

// LoadFleetSeries reads every profile that has samples in the window.
func LoadFleetSeries(q HistoryQuery) (*FleetSeries, error) {
	names, err := profile.ListProfiles()
	if err != nil {
		return nil, err
	}
	fleet := &FleetSeries{Profiles: []Series{}}
	var all []Sample
	for _, name := range names {
		samples, err := readSamples(name, q)
		if err != nil {
			return nil, err
		}
		if len(samples) == 0 {
			continue
		}
		series := Series{
			Profile: name,
			Samples: samples,
			Summary: summarize(samples),
		}
		fleet.Profiles = append(fleet.Profiles, series)
		all = append(all, samples...)
	}
	fleet.Summary = summarize(all)
	return fleet, nil
}

// ParseHistoryQuery accepts since/until as RFC3339 or a duration (24h, 7d).
func ParseHistoryQuery(sinceRaw, untilRaw, sourceRaw, limitRaw string) (HistoryQuery, error) {
	now := historyNow()
	var q HistoryQuery
	var err error
	q.Since, err = parseHistoryTime(sinceRaw, now, true)
	if err != nil {
		return HistoryQuery{}, fmt.Errorf("invalid since: %w", err)
	}
	q.Until, err = parseHistoryTime(untilRaw, now, false)
	if err != nil {
		return HistoryQuery{}, fmt.Errorf("invalid until: %w", err)
	}
	sourceRaw = strings.TrimSpace(sourceRaw)
	switch sourceRaw {
	case "", SourceQuota, SourceGateway, SourceHeadless:
		q.Source = sourceRaw
	default:
		return HistoryQuery{}, fmt.Errorf("invalid source %q", sourceRaw)
	}
	if strings.TrimSpace(limitRaw) == "" {
		q.Limit = 500
		return q, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(limitRaw))
	if err != nil || n < 1 {
		return HistoryQuery{}, fmt.Errorf("invalid limit %q", limitRaw)
	}
	if n > maxHistorySamples {
		n = maxHistorySamples
	}
	q.Limit = n
	return q, nil
}

func parseHistoryTime(raw string, now time.Time, isSince bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	if strings.HasSuffix(raw, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(raw, "d"))
		if err != nil || n < 0 {
			return time.Time{}, fmt.Errorf("%q is not a duration or RFC3339 timestamp", raw)
		}
		if !isSince {
			return now.Add(time.Duration(n) * 24 * time.Hour), nil
		}
		return now.Add(-time.Duration(n) * 24 * time.Hour), nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return time.Time{}, fmt.Errorf("%q is not a duration or RFC3339 timestamp", raw)
	}
	if isSince {
		return now.Add(-d), nil
	}
	return now.Add(d), nil
}

func bucketsFromSummary(data *QuotaSummaryResponse) []BucketPoint {
	now := historyNow()
	var points []BucketPoint
	for _, group := range data.Response.Groups {
		for _, bucket := range group.Buckets {
			window := bucket.WindowType
			if window == "" {
				window = ClassifyWindow(bucket, now)
			}
			points = append(points, BucketPoint{
				BucketID:          bucket.BucketID,
				DisplayName:       bucket.DisplayName,
				WindowType:        window,
				RemainingFraction: bucket.RemainingFraction,
				ResetTime:         bucket.ResetTime,
			})
		}
	}
	return points
}

func historyPath(profileName string) string {
	return filepath.Join(config.GetProfileDir(profileName), ".multigravity", historyFileName)
}

func appendSample(profileName string, sample Sample) error {
	if err := config.ValidateProfileName(profileName); err != nil {
		return err
	}
	if !profile.ProfileExists(profileName) {
		return nil
	}
	dir := filepath.Join(config.GetProfileDir(profileName), ".multigravity")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(historyPath(profileName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := lockFile(f); err != nil {
		return err
	}
	defer unlockFile(f)

	if sample.Source == SourceQuota {
		skip, err := quotaSnapshotUnchanged(f, sample)
		if err != nil {
			return err
		}
		if skip {
			return nil
		}
	}

	line, err := json.Marshal(sample)
	if err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > historyCompactBytes {
		return compactLocked(f)
	}
	return nil
}

func quotaSnapshotUnchanged(f *os.File, sample Sample) (bool, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return false, err
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		if len(bytes.TrimSpace(lines[i])) == 0 {
			continue
		}
		var prev Sample
		if err := json.Unmarshal(lines[i], &prev); err != nil {
			continue
		}
		if prev.Source != SourceQuota {
			continue
		}
		if sample.Timestamp.Sub(prev.Timestamp) > quotaSnapshotInterval {
			return false, nil
		}
		return sameBuckets(prev.Buckets, sample.Buckets), nil
	}
	return false, nil
}

func sameBuckets(a, b []BucketPoint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].BucketID != b[i].BucketID || a[i].RemainingFraction != b[i].RemainingFraction || a[i].ResetTime != b[i].ResetTime {
			return false
		}
	}
	return true
}

func compactLocked(f *os.File) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	cutoff := historyNow().Add(-maxHistoryAge)
	var kept [][]byte
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var sample Sample
		if err := json.Unmarshal(line, &sample); err != nil {
			continue
		}
		if !sample.Timestamp.IsZero() && sample.Timestamp.Before(cutoff) {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) > maxHistorySamples {
		kept = kept[len(kept)-maxHistorySamples:]
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	for _, line := range kept {
		if _, err := f.Write(append(line, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func readSamples(profileName string, q HistoryQuery) ([]Sample, error) {
	path := historyPath(profileName)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Sample{}, nil
		}
		return nil, err
	}
	defer f.Close()

	limit := q.Limit
	if limit <= 0 {
		limit = 500
	}
	var matched []Sample
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var sample Sample
		if err := json.Unmarshal([]byte(line), &sample); err != nil {
			continue
		}
		if q.Source != "" && sample.Source != q.Source {
			continue
		}
		if !q.Since.IsZero() && sample.Timestamp.Before(q.Since) {
			continue
		}
		if !q.Until.IsZero() && sample.Timestamp.After(q.Until) {
			continue
		}
		matched = append(matched, sample)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(matched) > limit {
		matched = matched[len(matched)-limit:]
	}
	if matched == nil {
		matched = []Sample{}
	}
	return matched, nil
}

func summarize(samples []Sample) HistorySummary {
	sum := HistorySummary{
		Samples:       len(samples),
		LatestBuckets: []BucketPoint{},
	}
	if len(samples) == 0 {
		return sum
	}
	sum.FirstTimestamp = samples[0].Timestamp.UTC().Format(time.RFC3339)
	sum.LastTimestamp = samples[len(samples)-1].Timestamp.UTC().Format(time.RFC3339)
	for _, sample := range samples {
		if len(sample.Buckets) > 0 {
			sum.LatestBuckets = sample.Buckets
		}
		if sample.Tokens == nil {
			continue
		}
		sum.PromptTokens += sample.Tokens.PromptTokens
		sum.CompletionTokens += sample.Tokens.CompletionTokens
		if sample.Tokens.TotalTokens > 0 {
			sum.TotalTokens += sample.Tokens.TotalTokens
		} else {
			sum.TotalTokens += sample.Tokens.PromptTokens + sample.Tokens.CompletionTokens
		}
	}
	if sum.LatestBuckets == nil {
		sum.LatestBuckets = []BucketPoint{}
	}
	return sum
}
