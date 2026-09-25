//go:build windows

package agent

import (
	"io"
	"os/exec"
)

type ptyDevice interface {
	io.ReadWriteCloser
	Resize(rows, cols uint16) error
}

type windowsPipePTY struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (p *windowsPipePTY) Read(b []byte) (int, error) {
	return p.stdout.Read(b)
}

func (p *windowsPipePTY) Write(b []byte) (int, error) {
	return p.stdin.Write(b)
}

func (p *windowsPipePTY) Close() error {
	err1 := p.stdin.Close()
	err2 := p.stdout.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

func (p *windowsPipePTY) Resize(rows, cols uint16) error {
	// Terminal resizing is a no-op on standard pipes
	return nil
}

func startPTY(cmd *exec.Cmd, rows, cols uint16) (ptyDevice, error) {
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdinPipe.Close()
		return nil, err
	}
	cmd.Stderr = cmd.Stdout // combine stdout and stderr

	if err := cmd.Start(); err != nil {
		_ = stdinPipe.Close()
		_ = stdoutPipe.Close()
		return nil, err
	}

	return &windowsPipePTY{
		stdin:  stdinPipe,
		stdout: stdoutPipe,
	}, nil
}
