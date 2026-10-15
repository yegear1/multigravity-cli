package tui

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/agent"
)

type mockSession struct {
	mu         sync.Mutex
	info       agent.Session
	chunks     chan agent.OutputChunk
	written    bytes.Buffer
	resizeRows uint16
	resizeCols uint16
}

func newMockSession(exitCode int) *mockSession {
	return &mockSession{
		info: agent.Session{
			ID:       "mock-sess-1",
			Status:   agent.StatusExited,
			ExitCode: exitCode,
		},
		chunks: make(chan agent.OutputChunk, 10),
	}
}

func (m *mockSession) GetInfo() agent.Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.info
}

func (m *mockSession) Subscribe() (<-chan agent.OutputChunk, func()) {
	return m.chunks, func() {}
}

func (m *mockSession) WriteInput(data []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.written.Write(data)
}

func (m *mockSession) Resize(rows, cols uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resizeRows = rows
	m.resizeCols = cols
	return nil
}

func (m *mockSession) Wait(timeout time.Duration) bool {
	return true
}

func TestMultiplexSession_Success(t *testing.T) {
	sess := newMockSession(0)

	stdin := bytes.NewBufferString("hello agent\n")
	stdout := new(bytes.Buffer)

	// Emit chunk and close channel to simulate process finishing output
	go func() {
		time.Sleep(10 * time.Millisecond)
		sess.chunks <- agent.OutputChunk{
			SessionID: "mock-sess-1",
			Data:      "agent output response\n",
			Timestamp: time.Now(),
		}
		time.Sleep(10 * time.Millisecond)
		close(sess.chunks)
	}()

	opts := MultiplexOptions{
		Stdin:   stdin,
		Stdout:  stdout,
		StdinFd: 999999, // Non-terminal FD to avoid MakeRaw in tests
	}

	err := MultiplexSession(sess, opts)
	if err != nil {
		t.Fatalf("unexpected MultiplexSession error: %v", err)
	}

	// Verify stdout received chunk
	if !strings.Contains(stdout.String(), "agent output response") {
		t.Errorf("expected stdout to contain 'agent output response', got %q", stdout.String())
	}

	// Verify stdin was forwarded
	sess.mu.Lock()
	gotInput := sess.written.String()
	sess.mu.Unlock()
	if !strings.Contains(gotInput, "hello agent") {
		t.Errorf("expected session input to contain 'hello agent', got %q", gotInput)
	}
}

func TestMultiplexSession_NonZeroExit(t *testing.T) {
	sess := newMockSession(127)
	close(sess.chunks)

	opts := MultiplexOptions{
		Stdin:   bytes.NewReader(nil),
		Stdout:  new(bytes.Buffer),
		StdinFd: 999999,
	}

	err := MultiplexSession(sess, opts)
	if err == nil {
		t.Fatalf("expected error on non-zero exit code, got nil")
	}
	if !strings.Contains(err.Error(), "code 127") {
		t.Errorf("expected error message to contain 'code 127', got %q", err.Error())
	}
}

func TestWatchResize_Lifecycle(t *testing.T) {
	var called bool
	cancel := WatchResize(0, func(rows, cols uint16) {
		called = true
	})

	// Cancel should stop watching cleanly without panic
	cancel()
	// Idempotent cancel
	cancel()
	_ = called
}
