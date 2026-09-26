package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func TestQuotaHistoryEndpoints(t *testing.T) {
	srv, home := setupTestServer(t)
	if err := os.MkdirAll(filepath.Join(home, "hist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := quota.RecordTokens("hist", quota.SourceGateway, "gemini-2.5-pro", 4, 6, 10, false); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/api/v1/quota/hist/history")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("profile history status %d", resp.StatusCode)
	}
	var wrapped struct {
		Success bool         `json:"success"`
		Data    quota.Series `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapped); err != nil {
		t.Fatal(err)
	}
	if !wrapped.Success || wrapped.Data.Summary.TotalTokens != 10 || len(wrapped.Data.Samples) != 1 {
		t.Fatalf("series: %+v", wrapped.Data)
	}

	fleetResp, err := http.Get(ts.URL + "/api/v1/quota/history?since=24h")
	if err != nil {
		t.Fatal(err)
	}
	defer fleetResp.Body.Close()
	if fleetResp.StatusCode != http.StatusOK {
		t.Fatalf("fleet status %d", fleetResp.StatusCode)
	}
	var fleetWrapped struct {
		Success bool              `json:"success"`
		Data    quota.FleetSeries `json:"data"`
	}
	if err := json.NewDecoder(fleetResp.Body).Decode(&fleetWrapped); err != nil {
		t.Fatal(err)
	}
	if len(fleetWrapped.Data.Profiles) != 1 || fleetWrapped.Data.Summary.TotalTokens != 10 {
		t.Fatalf("fleet: %+v", fleetWrapped.Data)
	}

	missing, err := http.Get(ts.URL + "/api/v1/quota/missing/history")
	if err != nil {
		t.Fatal(err)
	}
	defer missing.Body.Close()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", missing.StatusCode)
	}

	bad, err := http.Get(ts.URL + "/api/v1/quota/history?since=nope")
	if err != nil {
		t.Fatal(err)
	}
	defer bad.Body.Close()
	if bad.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", bad.StatusCode)
	}
}
