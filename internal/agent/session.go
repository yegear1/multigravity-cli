package agent

import (
	"errors"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

const maxBufferSize = 512 * 1024 // 512 KB sliding window

// SessionInstance manages the execution, I/O multiplexing, and lifecycle of a running agent process.
type SessionInstance struct {
	mu          sync.RWMutex
	info        Session
	cmd         *exec.Cmd
	ptyDev      ptyDevice
	outputBuf   []byte
	totalBytes  int64
	subscribers map[chan OutputChunk]struct{}
	exitChan    chan struct{}
	readDone    chan struct{}
	closed      bool
}

func newSessionInstance(info Session, cmd *exec.Cmd, dev ptyDevice) *SessionInstance {
	inst := &SessionInstance{
		info:        info,
		cmd:         cmd,
		ptyDev:      dev,
		outputBuf:   make([]byte, 0, 8192),
		subscribers: make(map[chan OutputChunk]struct{}),
		exitChan:    make(chan struct{}),
		readDone:    make(chan struct{}),
	}

	go inst.readLoop()
	go inst.waitLoop()

	return inst
}

func (s *SessionInstance) readLoop() {
	defer close(s.readDone)
	buf := make([]byte, 4096)
	for {
		n, err := s.ptyDev.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			s.broadcast(chunk)
		}
		if err != nil {
			break
		}
	}
}

func (s *SessionInstance) waitLoop() {
	err := s.cmd.Wait()

	// Wait up to 500ms for readLoop to drain any remaining output chunks from the PTY
	select {
	case <-s.readDone:
	case <-time.After(500 * time.Millisecond):
	}

	s.mu.Lock()
	now := time.Now()
	s.info.EndedAt = &now

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			s.info.ExitCode = exitErr.ExitCode()
		} else {
			s.info.ExitCode = -1
		}
		if s.info.Status != StatusStopped {
			if errors.As(err, &exitErr) {
				s.info.Status = StatusExited
			} else {
				s.info.Status = StatusFailed
			}
		}
	} else {
		s.info.ExitCode = 0
		if s.info.Status != StatusStopped {
			s.info.Status = StatusExited
		}
	}

	if s.ptyDev != nil {
		_ = s.ptyDev.Close()
	}

	// Notify exit and close subscribers safely under lock
	close(s.exitChan)
	s.closed = true
	for ch := range s.subscribers {
		close(ch)
	}
	s.subscribers = make(map[chan OutputChunk]struct{})
	s.mu.Unlock()
}

func (s *SessionInstance) broadcast(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	// Append to sliding buffer
	s.outputBuf = append(s.outputBuf, data...)
	if len(s.outputBuf) > maxBufferSize {
		excess := len(s.outputBuf) - maxBufferSize
		s.outputBuf = s.outputBuf[excess:]
	}

	offset := s.totalBytes
	s.totalBytes += int64(len(data))

	chunk := OutputChunk{
		SessionID: s.info.ID,
		Data:      string(data),
		Offset:    offset,
		Timestamp: time.Now(),
	}

	for ch := range s.subscribers {
		select {
		case ch <- chunk:
		default:
			// Non-blocking drop if consumer is too slow
		}
	}
}

// GetInfo returns a snapshot copy of the session metadata.
func (s *SessionInstance) GetInfo() Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.info
}

// WriteInput sends raw input bytes to the session's pseudo-terminal stdin.
func (s *SessionInstance) WriteInput(data []byte) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.info.Status != StatusRunning {
		return 0, errors.New("cannot write to terminated session")
	}
	if s.ptyDev == nil {
		return 0, errors.New("pty device is closed")
	}

	return s.ptyDev.Write(data)
}

// Resize updates the pseudo-terminal window dimensions.
func (s *SessionInstance) Resize(rows, cols uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.info.Status != StatusRunning {
		return errors.New("cannot resize terminated session")
	}

	s.info.Rows = rows
	s.info.Cols = cols

	if s.ptyDev != nil {
		return s.ptyDev.Resize(rows, cols)
	}
	return nil
}

// GetOutput returns the buffered output history, optionally limited to tailBytes.
func (s *SessionInstance) GetOutput(tailBytes int) []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if tailBytes <= 0 || tailBytes >= len(s.outputBuf) {
		res := make([]byte, len(s.outputBuf))
		copy(res, s.outputBuf)
		return res
	}

	start := len(s.outputBuf) - tailBytes
	res := make([]byte, tailBytes)
	copy(res, s.outputBuf[start:])
	return res
}

// Subscribe returns a channel that receives live output chunks and an unsubscribe function.
func (s *SessionInstance) Subscribe() (<-chan OutputChunk, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan OutputChunk, 100)

	// If already terminated, return closed channel
	if s.closed || s.info.Status != StatusRunning {
		close(ch)
		return ch, func() {}
	}

	s.subscribers[ch] = struct{}{}

	unsub := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.subscribers, ch)
	}

	return ch, unsub
}

// Stop sends an interrupt signal to stop the session process gracefully.
func (s *SessionInstance) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.info.Status != StatusRunning || s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	s.info.Status = StatusStopped
	return s.cmd.Process.Signal(syscall.SIGINT)
}

// Kill forces immediate termination of the session process.
func (s *SessionInstance) Kill() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.info.Status != StatusRunning || s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	s.info.Status = StatusStopped
	return s.cmd.Process.Kill()
}

// Wait blocks until the session process completes.
func (s *SessionInstance) Wait(timeout time.Duration) bool {
	if timeout <= 0 {
		<-s.exitChan
		return true
	}

	select {
	case <-s.exitChan:
		return true
	case <-time.After(timeout):
		return false
	}
}
