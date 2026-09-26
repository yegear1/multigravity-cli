package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/alert"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func TestAlertsEndpoints(t *testing.T) {
	srv, home := setupTestServer(t)
	if err := os.MkdirAll(filepath.Join(home, "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := quota.RecordQuotaSnapshot("work", &quota.QuotaSummaryResponse{
		Response: quota.QuotaResponse{
			Groups: []quota.QuotaGroup{{
				Buckets: []quota.QuotaBucket{{
					BucketID:          "gemini-weekly",
					RemainingFraction: 0.02,
					ResetTime:         time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
				}},
			}},
		},
	}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/api/v1/alerts/work")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var wrapped struct {
		Success bool         `json:"success"`
		Data    alert.Report `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapped); err != nil {
		t.Fatal(err)
	}
	if !wrapped.Success || len(wrapped.Data.Alerts) != 1 || wrapped.Data.Alerts[0].Kind != alert.KindQuotaCritical {
		t.Fatalf("report: %+v", wrapped.Data)
	}

	missing, err := http.Get(ts.URL + "/api/v1/alerts/missing")
	if err != nil {
		t.Fatal(err)
	}
	defer missing.Body.Close()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", missing.StatusCode)
	}
}
