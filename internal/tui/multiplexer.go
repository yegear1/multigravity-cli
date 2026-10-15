package tui

import (
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/term"

	"github.com/ye-dev/multigravity-cli/internal/agent"
)

// InteractiveSession defines the minimal interface for an agent terminal session.
type InteractiveSession interface {
	GetInfo() agent.Session
	Subscribe() (<-chan agent.OutputChunk, func())
	WriteInput(data []byte) (int, error)
	Resize(rows, cols uint16) error
	Wait(timeout time.Duration) bool
}

// MultiplexOptions configures interactive multiplexer streams and parameters.
type MultiplexOptions struct {
	Stdin   io.Reader
	Stdout  io.Writer
	StdinFd uintptr
}

// MultiplexSession connects an interactive terminal to an agent session instance,
// handling raw terminal mode, resize event forwarding, stdin routing, and stdout streaming.
func MultiplexSession(sess InteractiveSession, opts MultiplexOptions) error {
	stdin := opts.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	stdinFd := opts.StdinFd
	if stdinFd == 0 {
		stdinFd = os.Stdin.Fd()
	}

	// Put terminal in raw mode if interactive
	if term.IsTerminal(int(stdinFd)) {
		oldState, err := term.MakeRaw(int(stdinFd))
		if err == nil {
			defer func() { _ = term.Restore(int(stdinFd), oldState) }()
		}
	}

	// Subscribe to output chunks
	ch, unsub := sess.Subscribe()
	defer unsub()

	// Forward terminal resize signals/events
	stopResize := WatchResize(stdinFd, func(rows, cols uint16) {
		_ = sess.Resize(rows, cols)
	})
	defer stopResize()

	// Forward stdin in a background goroutine
	inputErrChan := make(chan error, 1)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdin.Read(buf)
			if n > 0 {
				_, _ = sess.WriteInput(buf[:n])
			}
			if err != nil {
				inputErrChan <- err
				return
			}
		}
	}()

	// Stream stdout chunks
	for chunk := range ch {
		_, _ = io.WriteString(stdout, chunk.Data)
	}

	// Wait for process exit
	sess.Wait(1 * time.Second)
	finalInfo := sess.GetInfo()
	if finalInfo.ExitCode != 0 {
		return fmt.Errorf("agent exited with code %d", finalInfo.ExitCode)
	}
	return nil
}
