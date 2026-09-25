package prime

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func TestPromptsLoadingAndSelection(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("REAL_HOME", tmpDir)

	prompts := LoadPrompts()
	if len(prompts) != 40 {
		t.Fatalf("expected 40 default prompts, got %d", len(prompts))
	}

	promptsFile := filepath.Join(tmpDir, ".local", "share", "multigravity", "prompts.json")
	if _, err := os.Stat(promptsFile); err != nil {
		t.Fatalf("prompts.json should have been created: %v", err)
	}

	used := make(map[string]bool)
	p1 := SelectRandomPrompt(prompts, used)
	used[p1] = true
	p2 := SelectRandomPrompt(prompts, used)
	if p1 == p2 {
		t.Errorf("p1 and p2 should be different when p1 is marked used")
	}
}

func TestStateLegacyMigration(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("REAL_HOME", tmpDir)

	stateFile := filepath.Join(tmpDir, ".local", "share", "multigravity", "prime_state.json")
	_ = os.MkdirAll(filepath.Dir(stateFile), 0755)

	legacyJSON := `{
		"profiles": {
			"work": {
				"last_reset_time": "2026-09-30T00:00:00Z",
				"last_primed_at": "2026-09-23T00:00:00Z",
				"last_cascade_id": "casc-old-123",
				"last_prompt": "ping"
			}
		}
	}`
	if err := os.WriteFile(stateFile, []byte(legacyJSON), 0644); err != nil {
		t.Fatal(err)
	}

	state, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	workState, ok := state.Profiles["work"]
	if !ok {
		t.Fatalf("profile 'work' not found in state")
	}

	geminiB, hasGemini := workState["gemini"]
	if !hasGemini {
		t.Fatalf("legacy state was not migrated into 'gemini' bucket")
	}

	if geminiB.LastResetTime != "2026-09-30T00:00:00Z" {
		t.Errorf("expected reset time preserved, got %s", geminiB.LastResetTime)
	}
	if geminiB.LastCascadeID != "casc-old-123" {
		t.Errorf("expected cascade id preserved, got %s", geminiB.LastCascadeID)
	}

	// Test saving and reloading migrated state
	geminiB.LastModel = "gemini-3.6-flash-low"
	workState["gemini"] = geminiB
	state.Profiles["work"] = workState

	if err := SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	reloaded, err := LoadState()
	if err != nil {
		t.Fatalf("reloading state failed: %v", err)
	}
	if reloaded.Profiles["work"]["gemini"].LastModel != "gemini-3.6-flash-low" {
		t.Errorf("expected model saved and reloaded")
	}
}

type mockQuotaClient struct {
	summaryFunc func(port int, csrf string) (*quota.QuotaSummaryResponse, error)
	startFunc   func(port int, csrf string) (string, error)
	sendFunc    func(port int, csrf string, cascadeID string, prompt string, model string) error
}

func (m *mockQuotaClient) RetrieveUserQuotaSummary(port int, csrf string) (*quota.QuotaSummaryResponse, error) {
	if m.summaryFunc != nil {
		return m.summaryFunc(port, csrf)
	}
	return &quota.QuotaSummaryResponse{}, nil
}

func (m *mockQuotaClient) StartCascade(port int, csrf string) (string, error) {
	if m.startFunc != nil {
		return m.startFunc(port, csrf)
	}
	return "mock-cascade-id", nil
}

func (m *mockQuotaClient) SendUserCascadeMessage(port int, csrf string, cascadeID string, prompt string, model string) error {
	if m.sendFunc != nil {
		return m.sendFunc(port, csrf, cascadeID, prompt, model)
	}
	return nil
}

func TestGetProfilePrimeStatus(t *testing.T) {
	homeDir := t.TempDir()
	profilesDir := filepath.Join(homeDir, "profiles")
	t.Setenv("MULTIGRAVITY_HOME", profilesDir)
	t.Setenv("REAL_HOME", homeDir)

	profDir := filepath.Join(profilesDir, "dev")
	if err := os.MkdirAll(profDir, 0755); err != nil {
		t.Fatal(err)
	}

	cleanup := SetTestHooks(
		func(profile string) ([]quota.ActiveServer, error) {
			return []quota.ActiveServer{
				{
					Profile: "dev",
					PID:     9999,
					Port:    5544,
					CSRF:    "csrf-dev",
					Data: &quota.QuotaSummaryResponse{
						Response: quota.QuotaResponse{
							Groups: []quota.QuotaGroup{
								{
									Buckets: []quota.QuotaBucket{
										{
											BucketID:          "gemini-weekly",
											DisplayName:       "Gemini",
											RemainingFraction: 1.0,
											ResetTime:         "2026-10-01T00:00:00Z",
										},
									},
								},
							},
						},
					},
				},
			}, nil
		},
		func(profile string) (*quota.HeadlessInstance, error) {
			return nil, fmt.Errorf("headless server mock offline")
		},
		func(timeout time.Duration) QuotaClient {
			return &mockQuotaClient{}
		},
		0,
	)
	defer cleanup()

	// 1. Status for dev profile
	st, err := GetProfilePrimeStatus("dev")
	if err != nil {
		t.Fatalf("GetProfilePrimeStatus failed: %v", err)
	}

	if st.Profile != "dev" {
		t.Errorf("expected profile dev, got %s", st.Profile)
	}
	if st.PID != 9999 || st.Port != 5544 {
		t.Errorf("expected pid 9999 and port 5544, got %d, %d", st.PID, st.Port)
	}
	if len(st.Buckets) != len(BucketConfigs) {
		t.Errorf("expected %d buckets, got %d", len(BucketConfigs), len(st.Buckets))
	}

	// 2. Non-existent profile should fail
	_, err = GetProfilePrimeStatus("nonexistent")
	if err == nil {
		t.Errorf("expected error for nonexistent profile")
	}

	// 3. GetAllProfilesPrimeStatus
	all, err := GetAllProfilesPrimeStatus()
	if err != nil {
		t.Fatalf("GetAllProfilesPrimeStatus failed: %v", err)
	}
	if len(all) != 1 || all[0].Profile != "dev" {
		t.Errorf("expected 1 status for dev, got %+v", all)
	}
}

func TestExecutePrimeSuccessAndProgressEvents(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", homeDir)
	t.Setenv("REAL_HOME", homeDir)

	profDir := filepath.Join(homeDir, "worker")
	if err := os.MkdirAll(profDir, 0755); err != nil {
		t.Fatal(err)
	}

	sentPrompts := []string{}
	client := &mockQuotaClient{
		summaryFunc: func(port int, csrf string) (*quota.QuotaSummaryResponse, error) {
			return &quota.QuotaSummaryResponse{
				Response: quota.QuotaResponse{
					Groups: []quota.QuotaGroup{
						{
							Buckets: []quota.QuotaBucket{
								{
									BucketID:          "gemini-weekly",
									DisplayName:       "Gemini Weekly",
									RemainingFraction: 1.0,
									ResetTime:         "2026-10-02T00:00:00Z",
								},
							},
						},
					},
				},
			}, nil
		},
		startFunc: func(port int, csrf string) (string, error) {
			return "cascade-worker-abc", nil
		},
		sendFunc: func(port int, csrf string, cascadeID string, prompt string, model string) error {
			sentPrompts = append(sentPrompts, prompt)
			return nil
		},
	}

	cleanup := SetTestHooks(
		func(profile string) ([]quota.ActiveServer, error) {
			return []quota.ActiveServer{
				{
					Profile: "worker",
					PID:     1122,
					Port:    8899,
					CSRF:    "csrf-worker",
				},
			}, nil
		},
		nil,
		func(timeout time.Duration) QuotaClient {
			return client
		},
		0,
	)
	defer cleanup()

	var events []PrimeProgressEvent
	cb := func(ev PrimeProgressEvent) {
		events = append(events, ev)
	}

	opts := PrimeOptions{
		Profile:  "worker",
		NoJitter: true,
	}

	res, err := ExecutePrime(opts, cb)
	if err != nil {
		t.Fatalf("ExecutePrime failed: %v", err)
	}

	if res.Profile != "worker" {
		t.Errorf("expected worker, got %s", res.Profile)
	}
	if len(res.Buckets) != 1 {
		t.Fatalf("expected 1 bucket result, got %d", len(res.Buckets))
	}
	bRes := res.Buckets[0]
	if bRes.Status != "primed" || bRes.CascadeID != "cascade-worker-abc" {
		t.Errorf("expected primed with cascade-worker-abc, got %+v", bRes)
	}
	if len(sentPrompts) != 1 {
		t.Errorf("expected 1 sent prompt, got %d", len(sentPrompts))
	}

	// Verify progress events
	stages := make(map[string]bool)
	for _, ev := range events {
		stages[ev.Stage] = true
	}
	for _, expectedStage := range []string{"initializing", "server_ready", "dispatching", "primed", "completed"} {
		if !stages[expectedStage] {
			t.Errorf("missing expected progress stage %s", expectedStage)
		}
	}

	// Verify state saved
	state, err := LoadState()
	if err != nil {
		t.Fatal(err)
	}
	workerState := state.Profiles["worker"]
	if workerState["gemini"].LastCascadeID != "cascade-worker-abc" {
		t.Errorf("expected cascade id saved in state")
	}
}

func TestExecutePrimeSkippingAndDryRun(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", homeDir)
	t.Setenv("REAL_HOME", homeDir)

	profDir := filepath.Join(homeDir, "testprof")
	if err := os.MkdirAll(profDir, 0755); err != nil {
		t.Fatal(err)
	}

	client := &mockQuotaClient{
		summaryFunc: func(port int, csrf string) (*quota.QuotaSummaryResponse, error) {
			return &quota.QuotaSummaryResponse{
				Response: quota.QuotaResponse{
					Groups: []quota.QuotaGroup{
						{
							Buckets: []quota.QuotaBucket{
								{
									BucketID:          "gemini-weekly",
									RemainingFraction: 1.0,
									ResetTime:         "2026-10-05T00:00:00Z",
								},
							},
						},
					},
				},
			}, nil
		},
	}

	cleanup := SetTestHooks(
		func(profile string) ([]quota.ActiveServer, error) {
			return []quota.ActiveServer{
				{Profile: "testprof", PID: 1, Port: 8000, CSRF: "tok"},
			}, nil
		},
		nil,
		func(timeout time.Duration) QuotaClient {
			return client
		},
		0,
	)
	defer cleanup()

	// 1. Dry run check
	checkOpts := PrimeOptions{
		Profile: "testprof",
		Check:   true,
	}
	resCheck, err := ExecutePrime(checkOpts, nil)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if len(resCheck.Buckets) != 1 || resCheck.Buckets[0].Status != "ready" {
		t.Errorf("expected ready status for dry run, got %+v", resCheck.Buckets)
	}

	// 2. Already primed
	state, _ := LoadState()
	state.Profiles["testprof"] = ProfileState{
		"gemini": BucketState{
			LastResetTime: "2026-10-05T00:00:00Z",
		},
	}
	_ = SaveState(state)

	// Quota is at 0.8 (already consuming this cycle)
	client.summaryFunc = func(port int, csrf string) (*quota.QuotaSummaryResponse, error) {
		return &quota.QuotaSummaryResponse{
			Response: quota.QuotaResponse{
				Groups: []quota.QuotaGroup{
					{
						Buckets: []quota.QuotaBucket{
							{
								BucketID:          "gemini-weekly",
								RemainingFraction: 0.8,
								ResetTime:         "2026-10-05T00:00:00Z",
							},
						},
					},
				},
			},
		}, nil
	}

	resPrimed, err := ExecutePrime(PrimeOptions{Profile: "testprof"}, nil)
	if err != nil {
		t.Fatalf("prime failed: %v", err)
	}
	if len(resPrimed.Buckets) != 1 || resPrimed.Buckets[0].Status != "skipped" {
		t.Errorf("expected skipped status for already primed cycle, got %+v", resPrimed.Buckets)
	}
}
