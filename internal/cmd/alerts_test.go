package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/alert"
	"github.com/ye-dev/multigravity-cli/internal/headless"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func TestAlertsCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "work", ".multigravity"), 0o755); err != nil {
		t.Fatal(err)
	}
	recordQuotaCmd(t, "work", 0.50)
	recordQuotaCmd(t, "work", 0.04)

	info := headless.InstanceInfo{Profile: "work", PID: 5151, Port: 9, Status: headless.StateRunning}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "work", ".multigravity", "headless.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	restore := headless.SetTestHooks(nil, nil, func(int) bool { return false }, nil, nil)
	defer restore()

	text, err := executeCommand(rootCmd, "alerts", "work")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, alert.KindQuotaCritical) || !strings.Contains(text, alert.KindQuotaDrop) || !strings.Contains(text, "pid=5151") {
		t.Fatalf("text: %s", text)
	}

	raw, err := executeCommand(rootCmd, "alerts", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var report alert.Report
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		t.Fatalf("json %v: %s", err, raw)
	}
	if len(report.Alerts) != 2 {
		t.Fatalf("second pass should keep quota alerts only, got %+v", report.Alerts)
	}
}

func recordQuotaCmd(t *testing.T, profileName string, fraction float64) {
	t.Helper()
	err := quota.RecordQuotaSnapshot(profileName, &quota.QuotaSummaryResponse{
		Response: quota.QuotaResponse{
			Groups: []quota.QuotaGroup{{
				Buckets: []quota.QuotaBucket{{
					BucketID:          "gemini-weekly",
					RemainingFraction: fraction,
					ResetTime:         "2026-10-01T00:00:00Z",
				}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}
