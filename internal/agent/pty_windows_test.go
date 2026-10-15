//go:build windows

package agent

import (
	"bytes"
	"io"
	"testing"
)

type dummyReadWriteCloser struct {
	io.Reader
	io.Writer
	closed bool
}

func (d *dummyReadWriteCloser) Close() error {
	d.closed = true
	return nil
}

func TestWindowsPipePTY_ResizeAndLifecycle(t *testing.T) {
	inBuf := new(bytes.Buffer)
	outBuf := bytes.NewBufferString("hello windows pty")

	dummyIn := &dummyReadWriteCloser{Reader: bytes.NewReader(nil), Writer: inBuf}
	dummyOut := &dummyReadWriteCloser{Reader: outBuf, Writer: bytes.NewBuffer(nil)}

	pty := &windowsPipePTY{
		stdin:  dummyIn,
		stdout: dummyOut,
		rows:   24,
		cols:   80,
	}

	// 1. Read
	readBuf := make([]byte, 32)
	n, err := pty.Read(readBuf)
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if string(readBuf[:n]) != "hello windows pty" {
		t.Errorf("expected read 'hello windows pty', got %q", string(readBuf[:n]))
	}

	// 2. Write
	_, err = pty.Write([]byte("input data"))
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if inBuf.String() != "input data" {
		t.Errorf("expected written data 'input data', got %q", inBuf.String())
	}

	// 3. Resize
	if err := pty.Resize(40, 120); err != nil {
		t.Errorf("unexpected resize error: %v", err)
	}
	if pty.rows != 40 || pty.cols != 120 {
		t.Errorf("expected rows=40 cols=120, got rows=%d cols=%d", pty.rows, pty.cols)
	}

	// Default fallbacks on zero dimensions
	if err := pty.Resize(0, 0); err != nil {
		t.Errorf("unexpected resize error on zero: %v", err)
	}
	if pty.rows != 24 || pty.cols != 80 {
		t.Errorf("expected default rows=24 cols=80, got rows=%d cols=%d", pty.rows, pty.cols)
	}

	// 4. Close
	if err := pty.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if !dummyIn.closed || !dummyOut.closed {
		t.Errorf("expected pipes to be closed")
	}
}
