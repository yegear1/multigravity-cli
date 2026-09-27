package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/dispatch"
)

func setupTestGitRepoForServer(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git command %s failed: %v, output: %s", args, err, out)
		}
	}

	run("git", "init")
	run("git", "config", "user.name", "Test")
	run("git", "config", "user.email", "test@test.com")

	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("# Server Test Repo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "README.md")
	run("git", "commit", "-m", "initial commit")

	return dir
}

func TestDispatchPlanEndpoints(t *testing.T) {
	srv, home := setupTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	repoDir := setupTestGitRepoForServer(t)

	// Create profiles in MULTIGRAVITY_HOME
	for _, name := range []string{"plan-prof-1", "plan-prof-2"} {
		pDir := filepath.Join(home, name)
		if err := os.MkdirAll(pDir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// 1. POST /api/v1/dispatch/plans with 2 subtasks in disjoint worktrees
	planPayload := dispatch.PlanRequest{
		PlanID:   "plan-srv-test-1",
		RepoPath: repoDir,
		Workers:  2,
		Subtasks: []dispatch.SubtaskSpec{
			{
				ID:      "task-auth",
				Profile: "plan-prof-1",
				Command: "sh",
				Args:    []string{"-c", "mkdir -p auth && echo 'auth module' > auth/auth.go && git add -A"},
			},
			{
				ID:      "task-api",
				Profile: "plan-prof-2",
				Command: "sh",
				Args:    []string{"-c", "mkdir -p api && echo 'api module' > api/api.go && git add -A"},
			},
		},
	}
	payloadBytes, _ := json.Marshal(planPayload)

	postResp, err := http.Post(ts.URL+"/api/v1/dispatch/plans", "application/json", strings.NewReader(string(payloadBytes)))
	if err != nil {
		t.Fatalf("failed to POST /api/v1/dispatch/plans: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", postResp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.NewDecoder(postResp.Body).Decode(&apiResp); err != nil {
		t.Fatalf("failed to decode APIResponse: %v", err)
	}
	if !apiResp.Success {
		t.Fatalf("expected success, got error: %s", apiResp.Error)
	}

	dataBytes, _ := json.Marshal(apiResp.Data)
	var planResult dispatch.PlanResult
	if err := json.Unmarshal(dataBytes, &planResult); err != nil {
		t.Fatalf("failed to unmarshal PlanResult: %v", err)
	}

	if planResult.PlanID != "plan-srv-test-1" {
		t.Errorf("expected plan ID 'plan-srv-test-1', got %s", planResult.PlanID)
	}
	if planResult.Status != "completed" {
		t.Errorf("expected status 'completed', got %s", planResult.Status)
	}
	if planResult.Succeeded != 2 {
		t.Errorf("expected 2 succeeded subtasks, got %d", planResult.Succeeded)
	}
	if planResult.UnifiedSummary.TotalFilesChanged != 2 {
		t.Errorf("expected 2 files changed, got %d", planResult.UnifiedSummary.TotalFilesChanged)
	}
	if !planResult.UnifiedSummary.DisjointScopesClean {
		t.Errorf("expected DisjointScopesClean to be true")
	}

	// 2. GET /api/v1/dispatch/plans/{id}?repo=...
	getResp, err := http.Get(fmt.Sprintf("%s/api/v1/dispatch/plans/%s?repo=%s", ts.URL, "plan-srv-test-1", repoDir))
	if err != nil {
		t.Fatalf("failed to GET plan: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for GET plan, got %d", getResp.StatusCode)
	}

	var getAPIResp APIResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getAPIResp); err != nil {
		t.Fatalf("failed to decode GET APIResponse: %v", err)
	}
	getDataBytes, _ := json.Marshal(getAPIResp.Data)
	var getPlanResult dispatch.PlanResult
	if err := json.Unmarshal(getDataBytes, &getPlanResult); err != nil {
		t.Fatalf("failed to unmarshal retrieved plan: %v", err)
	}
	if getPlanResult.PlanID != "plan-srv-test-1" {
		t.Errorf("expected retrieved plan ID 'plan-srv-test-1', got %s", getPlanResult.PlanID)
	}

	// 3. GET /api/v1/dispatch/plans?repo=...
	listResp, err := http.Get(fmt.Sprintf("%s/api/v1/dispatch/plans?repo=%s", ts.URL, repoDir))
	if err != nil {
		t.Fatalf("failed to GET plans list: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for list plans, got %d", listResp.StatusCode)
	}

	// 4. Test error handling: invalid JSON
	badResp, err := http.Post(ts.URL+"/api/v1/dispatch/plans", "application/json", strings.NewReader("{invalid-json"))
	if err != nil {
		t.Fatalf("failed bad request: %v", err)
	}
	defer badResp.Body.Close()
	if badResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for bad json, got %d", badResp.StatusCode)
	}

	// 5. Test error handling: empty subtasks
	emptyResp, err := http.Post(ts.URL+"/api/v1/dispatch/plans", "application/json", strings.NewReader(`{"subtasks":[]}`))
	if err != nil {
		t.Fatalf("failed empty request: %v", err)
	}
	defer emptyResp.Body.Close()
	if emptyResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty subtasks, got %d", emptyResp.StatusCode)
	}
}
