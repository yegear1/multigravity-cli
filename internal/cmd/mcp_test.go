package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/mcp"
)

func TestMCPServeCommand(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tempHome)

	// Test stdio execution via mcpCmd
	in := bytes.NewBufferString("{\"jsonrpc\":\"2.0\",\"id\":99,\"method\":\"ping\"}\n")
	out := &bytes.Buffer{}

	mcpCmd.SetIn(in)
	mcpCmd.SetOut(out)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := newMCPServeCmd()
	cmd.SetIn(in)
	cmd.SetOut(out)
	cmd.SetContext(ctx)

	// Run stdio mode with a channel to cancel after receiving response
	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute()
	}()

	select {
	case <-time.After(500 * time.Millisecond):
		cancel()
	case err := <-done:
		if err != nil && err != context.Canceled {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("expected JSON-RPC response on stdout, got: %q", out.String())
	}

	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &resp); err != nil {
		t.Fatalf("failed to decode response: %v, raw: %q", err, lines[0])
	}
	if resp.ID != float64(99) {
		t.Errorf("expected ID 99, got %v", resp.ID)
	}
}

func TestMCPServerRootCmd(t *testing.T) {
	cmd := newMCPServerRootCmd()
	if cmd.Use != "mcp-server" {
		t.Errorf("expected Use 'mcp-server', got %s", cmd.Use)
	}
}
