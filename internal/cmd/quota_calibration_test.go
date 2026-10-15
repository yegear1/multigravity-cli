package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func TestQuotaCalibrationCommand(t *testing.T) {
	quota.ResetCalibration()
	defer quota.ResetCalibration()

	// 1. Initially uncalibrated
	text, err := executeCommand(rootCmd, "quota", "calibration")
	if err != nil {
		t.Fatalf("quota calibration: %v", err)
	}
	if !strings.Contains(text, "Observations:     0 samples") || !strings.Contains(text, "4.00 runes/token") {
		t.Fatalf("unexpected uncalibrated output:\n%s", text)
	}

	// 2. Calibrate with live observation
	quota.CalibrateTokens("gemini-2.5-pro", "01234567890123456789", 10, "01234567890123456789", 10)

	textAfter, err := executeCommand(rootCmd, "quota", "calibration")
	if err != nil {
		t.Fatalf("quota calibration after observation: %v", err)
	}
	if !strings.Contains(textAfter, "Observations:     1 samples") || !strings.Contains(textAfter, "gemini-2.5-pro") {
		t.Fatalf("unexpected calibrated output:\n%s", textAfter)
	}

	// 3. JSON contract
	jsonOut, err := executeCommand(rootCmd, "quota", "calibration", "--json")
	if err != nil {
		t.Fatalf("quota calibration --json: %v", err)
	}

	var stats quota.CalibrationStats
	if err := json.Unmarshal([]byte(jsonOut), &stats); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v\n%s", err, jsonOut)
	}
	if stats.Samples != 1 || stats.TotalTokens != 20 || stats.TotalRunes != 40 {
		t.Fatalf("unexpected stats payload: %+v", stats)
	}
	if mc, ok := stats.Models["gemini-2.5-pro"]; !ok || mc.Samples != 1 {
		t.Fatalf("expected model gemini-2.5-pro in payload: %+v", stats.Models)
	}
}
