package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
