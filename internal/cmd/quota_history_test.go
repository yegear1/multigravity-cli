package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func TestQuotaHistoryCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := quota.RecordTokens("work", quota.SourceGateway, "gemini-2.5-pro", 2, 3, 5, true); err != nil {
		t.Fatal(err)
	}

	text, err := executeCommand(rootCmd, "quota", "history", "work")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "tokens=5") || !strings.Contains(text, "estimated") {
		t.Fatalf("text output: %s", text)
	}

	out, err := executeCommand(rootCmd, "quota", "history", "work", "--json")
	if err != nil {
		t.Fatalf("quota history --json: %v\n%s", err, out)
	}
	var series quota.Series
	if err := json.Unmarshal([]byte(out), &series); err != nil {
		t.Fatalf("parse: %v\n%s", err, out)
	}
	if series.Profile != "work" || series.Summary.TotalTokens != 5 {
		t.Fatalf("series: %+v", series)
	}
}
