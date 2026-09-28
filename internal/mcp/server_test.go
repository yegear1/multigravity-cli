package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/agent"
	"github.com/ye-dev/multigravity-cli/internal/dispatch"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func setupTestEnvironment(t *testing.T) (*Server, string) {
	t.Helper()
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", filepath.Join(tempHome, "shortcuts"))

	srv := NewDefaultServer()
	return srv, tempHome
}

func TestInitialize(t *testing.T) {
	srv, _ := setupTestEnvironment(t)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}`),
	}

	resp := srv.HandleRequest(context.Background(), req)
	if resp == nil {
		t.Fatalf("expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	initRes, ok := resp.Result.(InitializeResult)
	if !ok {
		t.Fatalf("expected InitializeResult, got %T", resp.Result)
	}

	if initRes.ProtocolVersion != ProtocolVersion {
		t.Errorf("expected protocol version %s, got %s", ProtocolVersion, initRes.ProtocolVersion)
	}
	if initRes.ServerInfo.Name != ServerName {
		t.Errorf("expected server name %s, got %s", ServerName, initRes.ServerInfo.Name)
	}
	if initRes.Capabilities.Tools == nil {
		t.Errorf("expected tools capability to be declared")
	}
}

func TestPing(t *testing.T) {
	srv, _ := setupTestEnvironment(t)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "ping-1",
		Method:  "ping",
	}

	resp := srv.HandleRequest(context.Background(), req)
	if resp == nil || resp.Error != nil {
		t.Fatalf("unexpected response or error: %v", resp)
	}
}

func TestToolsList(t *testing.T) {
	srv, _ := setupTestEnvironment(t)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	resp := srv.HandleRequest(context.Background(), req)
	if resp == nil || resp.Error != nil {
		t.Fatalf("unexpected response: %v", resp)
	}

	res, ok := resp.Result.(ListToolsResult)
	if !ok {
		t.Fatalf("expected ListToolsResult, got %T", resp.Result)
	}

	expectedTools := []string{
		"dispatch_task",
		"dispatch_plan",
		"dispatch_list",
		"dispatch_status",
		"dispatch_cancel",
		"dispatch_logs",
		"profile_list",
		"profile_status",
		"doctor_diagnose",
		"workspace_list",
		"alerts_list",
		"quota_summary",
		"quota_history",
		"prime_status",
		"prime_trigger",
		"task_diff",
		"worktree_diff",
	}

	foundMap := make(map[string]bool)
	for _, tool := range res.Tools {
		foundMap[tool.Name] = true
		if tool.Description == "" {
			t.Errorf("tool %s is missing description", tool.Name)
		}
		if tool.InputSchema.Type != "object" {
			t.Errorf("tool %s inputSchema type should be object, got %s", tool.Name, tool.InputSchema.Type)
		}
	}

	for _, name := range expectedTools {
		if !foundMap[name] {
			t.Errorf("expected tool %s was not found in registered tools", name)
		}
	}
}

func TestToolsCall(t *testing.T) {
	srv, _ := setupTestEnvironment(t)

	// 1. Call profile_list
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      10,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"profile_list","arguments":{}}`),
	}

	resp := srv.HandleRequest(context.Background(), req)
	if resp == nil || resp.Error != nil {
		t.Fatalf("profile_list call error: %v", resp)
	}

	res, ok := resp.Result.(*CallToolResult)
	if !ok || res.IsError {
		t.Fatalf("expected successful CallToolResult, got %+v", res)
	}
	if len(res.Content) == 0 || !strings.Contains(res.Content[0].Text, "[") {
		t.Errorf("expected JSON array response, got %s", res.Content[0].Text)
	}

	// 2. Call doctor_diagnose
	reqDoc := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      11,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"doctor_diagnose","arguments":{}}`),
	}

	respDoc := srv.HandleRequest(context.Background(), reqDoc)
	if respDoc == nil || respDoc.Error != nil {
		t.Fatalf("doctor_diagnose call error: %v", respDoc)
	}
	resDoc, ok := respDoc.Result.(*CallToolResult)
	if !ok || resDoc.IsError {
		t.Fatalf("expected successful CallToolResult, got %+v", resDoc)
	}

	// 3. Call unknown tool
	reqUnknown := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      12,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"non_existent_tool","arguments":{}}`),
	}
	respUnknown := srv.HandleRequest(context.Background(), reqUnknown)
	if respUnknown == nil {
		t.Fatalf("expected response for unknown tool")
	}
	resUnknown, ok := respUnknown.Result.(*CallToolResult)
	if !ok || !resUnknown.IsError {
		t.Fatalf("expected isError: true for unknown tool call, got %+v", resUnknown)
	}

	// 4. Call dispatch_plan with empty subtasks
	reqPlanEmpty := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      13,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"dispatch_plan","arguments":{"subtasks":[]}}`),
	}
	respPlanEmpty := srv.HandleRequest(context.Background(), reqPlanEmpty)
	if respPlanEmpty == nil {
		t.Fatalf("expected response for dispatch_plan call")
	}
	resPlanEmpty, ok := respPlanEmpty.Result.(*CallToolResult)
	if !ok || !resPlanEmpty.IsError {
		t.Fatalf("expected isError: true for empty subtasks, got %+v", resPlanEmpty)
	}
	if !strings.Contains(resPlanEmpty.Content[0].Text, "subtasks list cannot be empty") {
		t.Errorf("unexpected error message: %s", resPlanEmpty.Content[0].Text)
	}
}

func TestStdioTransport(t *testing.T) {
	srv, _ := setupTestEnvironment(t)

	inputLines := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
	}
	input := strings.Join(inputLines, "\n") + "\n"

	in := bytes.NewBufferString(input)
	out := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := srv.ServeStdio(ctx, in, out)
	if err != nil {
		t.Fatalf("ServeStdio error: %v", err)
	}

	output := out.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 response lines, got %d: %q", len(lines), output)
	}

	var resp1 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &resp1); err != nil {
		t.Fatalf("failed to decode response 1: %v", err)
	}
	if resp1.Error != nil || resp1.ID != float64(1) {
		t.Errorf("unexpected response 1: %+v", resp1)
	}

	var resp2 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[1]), &resp2); err != nil {
		t.Fatalf("failed to decode response 2: %v", err)
	}
	if resp2.Error != nil || resp2.ID != float64(2) {
		t.Errorf("unexpected response 2: %+v", resp2)
	}
}

func TestHTTPJSONRPC(t *testing.T) {
	srv, _ := setupTestEnvironment(t)
	handler := NewHTTPHandler(srv)

	reqBody := `{"jsonrpc":"2.0","id":"http-1","method":"tools/list"}`
	httpReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(reqBody))
	rec := httptest.NewRecorder()

	handler.HandleJSONRPC(rec, httpReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected json-rpc error: %+v", resp.Error)
	}
	if resp.ID != "http-1" {
		t.Errorf("expected ID 'http-1', got %v", resp.ID)
	}
}

func TestHTTPSSETransport(t *testing.T) {
	srv, _ := setupTestEnvironment(t)
	handler := NewHTTPHandler(srv)

	sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/sse") {
			handler.HandleSSE(w, r)
		} else if strings.HasPrefix(r.URL.Path, "/messages") {
			handler.HandleMessage(w, r)
		}
	}))
	defer sseServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sseReq, err := http.NewRequestWithContext(ctx, http.MethodGet, sseServer.URL+"/sse", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	sseResp, err := http.DefaultClient.Do(sseReq)
	if err != nil {
		t.Fatalf("failed to execute SSE request: %v", err)
	}
	defer sseResp.Body.Close()

	// Read initial endpoint event
	reader := bufio.NewReader(sseResp.Body)
	var endpointLine string
	for i := 0; i < 5; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("error reading SSE: %v", err)
		}
		if strings.HasPrefix(line, "data: ") {
			endpointLine = strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			break
		}
	}

	if !strings.Contains(endpointLine, "sessionId=") {
		t.Fatalf("expected endpoint event with sessionId, got: %q", endpointLine)
	}

	// Post message to the endpoint returned
	msgURL := sseServer.URL + endpointLine
	postBody := `{"jsonrpc":"2.0","id":"sse-call-1","method":"ping"}`
	postResp, err := http.Post(msgURL, "application/json", strings.NewReader(postBody))
	if err != nil {
		t.Fatalf("POST message failed: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", postResp.StatusCode)
	}

	var jsonResp JSONRPCResponse
	if err := json.NewDecoder(postResp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if jsonResp.ID != "sse-call-1" {
		t.Errorf("expected ID sse-call-1, got %v", jsonResp.ID)
	}
}

func TestMoreTools(t *testing.T) {
	srv, _ := setupTestEnvironment(t)

	// Call workspace_list
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      100,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"workspace_list","arguments":{}}`),
	}
	resp := srv.HandleRequest(context.Background(), req)
	if resp == nil || resp.Error != nil {
		t.Fatalf("workspace_list call error: %v", resp)
	}

	// Call alerts_list
	reqAlerts := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      101,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"alerts_list","arguments":{}}`),
	}
	respAlerts := srv.HandleRequest(context.Background(), reqAlerts)
	if respAlerts == nil || respAlerts.Error != nil {
		t.Fatalf("alerts_list call error: %v", respAlerts)
	}

	// Call quota_summary
	reqQuota := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      102,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"quota_summary","arguments":{}}`),
	}
	respQuota := srv.HandleRequest(context.Background(), reqQuota)
	if respQuota == nil || respQuota.Error != nil {
		t.Fatalf("quota_summary call error: %v", respQuota)
	}
}

func TestQuotaSummary_OmitsCSRF(t *testing.T) {
	srv, _ := setupTestEnvironment(t)

	cleanup := quota.SetTestHooks(func(profile string) ([]quota.ActiveServer, error) {
		return []quota.ActiveServer{
			{
				Profile: "mcp-prof",
				PID:     5555,
				Port:    6666,
				CSRF:    "mcp-super-secret-csrf-token",
			},
		}, nil
	})
	defer cleanup()

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      200,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"quota_summary","arguments":{}}`),
	}
	resp := srv.HandleRequest(context.Background(), req)
	if resp == nil || resp.Error != nil {
		t.Fatalf("quota_summary call failed: %v", resp)
	}

	callResult, ok := resp.Result.(*CallToolResult)
	if !ok {
		t.Fatalf("expected *CallToolResult, got %T", resp.Result)
	}
	if len(callResult.Content) == 0 {
		t.Fatalf("expected non-empty content in CallToolResult")
	}

	text := callResult.Content[0].Text
	if strings.Contains(text, "mcp-super-secret-csrf-token") {
		t.Errorf("tool result text contains CSRF secret: %s", text)
	}
	if strings.Contains(text, `"csrf"`) {
		t.Errorf("tool result text contains 'csrf' key: %s", text)
	}
	if !strings.Contains(text, "mcp-prof") {
		t.Errorf("tool result text missing profile name: %s", text)
	}
}

func TestDispatchTask_ArgsAndPrompt(t *testing.T) {
	srv, _ := setupTestEnvironment(t)

	// Create test profile
	if err := profile.CreateProfile(profile.CreateOptions{Name: "dev"}); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	// Intercept process creation hermetically - do not execute agy for real
	var capturedOpts []agent.CreateSessionOptions
	cleanupAgent := agent.SetTestHooks(func(opts agent.CreateSessionOptions) *exec.Cmd {
		capturedOpts = append(capturedOpts, opts)
		return exec.Command("true")
	})
	defer cleanupAgent()

	// 1. Schema check
	t.Run("SchemaProperties", func(t *testing.T) {
		req := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      300,
			Method:  "tools/list",
			Params:  json.RawMessage(`{}`),
		}
		resp := srv.HandleRequest(context.Background(), req)
		res := resp.Result.(ListToolsResult)
		var dt *Tool
		for i := range res.Tools {
			if res.Tools[i].Name == "dispatch_task" {
				dt = &res.Tools[i]
				break
			}
		}
		if dt == nil {
			t.Fatalf("dispatch_task tool not found")
		}
		if _, ok := dt.InputSchema.Properties["args"]; !ok {
			t.Errorf("expected 'args' property in dispatch_task schema")
		}
		if _, ok := dt.InputSchema.Properties["prompt"]; !ok {
			t.Errorf("expected 'prompt' property in dispatch_task schema")
		}
		reqProps := dt.InputSchema.Required
		hasProfile := false
		hasCommand := false
		for _, p := range reqProps {
			if p == "profile" {
				hasProfile = true
			}
			if p == "command" {
				hasCommand = true
			}
		}
		if !hasProfile || !hasCommand {
			t.Errorf("expected 'command' and 'profile' in required properties, got %v", reqProps)
		}
	})

	// 2. agy with prompt and without args produces argv equivalent to -p plus prompt
	t.Run("AgyWithPromptNoArgs", func(t *testing.T) {
		capturedOpts = nil
		req := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      301,
			Method:  "tools/call",
			Params:  json.RawMessage(`{"name":"dispatch_task","arguments":{"profile":"dev","command":"agy","prompt":"fix issue","background":true}}`),
		}
		resp := srv.HandleRequest(context.Background(), req)
		if resp == nil || resp.Error != nil {
			t.Fatalf("unexpected error response: %v", resp)
		}
		callResult, ok := resp.Result.(*CallToolResult)
		if !ok || callResult.IsError {
			t.Fatalf("expected successful CallToolResult, got %+v", callResult)
		}

		var task dispatch.Task
		if err := json.Unmarshal([]byte(callResult.Content[0].Text), &task); err != nil {
			t.Fatalf("failed to decode task response: %v", err)
		}
		if task.Command != "agy" {
			t.Errorf("expected task.Command 'agy', got %q", task.Command)
		}
		if task.Prompt != "fix issue" {
			t.Errorf("expected task.Prompt 'fix issue', got %q", task.Prompt)
		}
		if len(task.Args) != 2 || task.Args[0] != "-p" || task.Args[1] != "fix issue" {
			t.Errorf("expected task.Args [-p fix issue], got %v", task.Args)
		}
		if len(capturedOpts) > 0 {
			if capturedOpts[0].Command != "agy" || len(capturedOpts[0].Args) != 2 || capturedOpts[0].Args[0] != "-p" || capturedOpts[0].Args[1] != "fix issue" {
				t.Errorf("unexpected capturedOpts: %s %v", capturedOpts[0].Command, capturedOpts[0].Args)
			}
		}
	})

	// 3. agy with explicit args does not duplicate -p
	t.Run("AgyWithExplicitArgs", func(t *testing.T) {
		capturedOpts = nil
		req := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      302,
			Method:  "tools/call",
			Params:  json.RawMessage(`{"name":"dispatch_task","arguments":{"profile":"dev","command":"agy","prompt":"fix issue","args":["-p","custom prompt","--print"],"background":true}}`),
		}
		resp := srv.HandleRequest(context.Background(), req)
		if resp == nil || resp.Error != nil {
			t.Fatalf("unexpected error response: %v", resp)
		}
		callResult, ok := resp.Result.(*CallToolResult)
		if !ok || callResult.IsError {
			t.Fatalf("expected successful CallToolResult, got %+v", callResult)
		}

		var task dispatch.Task
		if err := json.Unmarshal([]byte(callResult.Content[0].Text), &task); err != nil {
			t.Fatalf("failed to decode task response: %v", err)
		}
		if task.Command != "agy" {
			t.Errorf("expected task.Command 'agy', got %q", task.Command)
		}
		if len(task.Args) != 3 || task.Args[0] != "-p" || task.Args[1] != "custom prompt" || task.Args[2] != "--print" {
			t.Errorf("expected task.Args [-p custom prompt --print], got %v", task.Args)
		}
	})

	// 4. command with spaces is not sliced into argv
	t.Run("CommandWithSpacesNotSliced", func(t *testing.T) {
		capturedOpts = nil
		req := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      303,
			Method:  "tools/call",
			Params:  json.RawMessage(`{"name":"dispatch_task","arguments":{"profile":"dev","command":"echo hello world","background":true}}`),
		}
		resp := srv.HandleRequest(context.Background(), req)
		if resp == nil || resp.Error != nil {
			t.Fatalf("unexpected error response: %v", resp)
		}
		callResult, ok := resp.Result.(*CallToolResult)
		if !ok || callResult.IsError {
			t.Fatalf("expected successful CallToolResult, got %+v", callResult)
		}

		var task dispatch.Task
		if err := json.Unmarshal([]byte(callResult.Content[0].Text), &task); err != nil {
			t.Fatalf("failed to decode task response: %v", err)
		}
		if task.Command != "echo hello world" {
			t.Errorf("expected task.Command 'echo hello world', got %q", task.Command)
		}
		if len(task.Args) != 0 {
			t.Errorf("expected empty task.Args, got %v", task.Args)
		}
	})

	// 5. missing profile returns error
	t.Run("MissingProfileError", func(t *testing.T) {
		req := &JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      304,
			Method:  "tools/call",
			Params:  json.RawMessage(`{"name":"dispatch_task","arguments":{"command":"agy"}}`),
		}
		resp := srv.HandleRequest(context.Background(), req)
		if resp == nil {
			t.Fatalf("expected response")
		}
		callResult, ok := resp.Result.(*CallToolResult)
		if !ok || !callResult.IsError {
			t.Fatalf("expected isError: true when profile is missing, got %+v", callResult)
		}
		if !strings.Contains(callResult.Content[0].Text, "profile is required") {
			t.Errorf("expected 'profile is required' error, got %s", callResult.Content[0].Text)
		}
	})
}
