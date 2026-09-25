//go:build !windows

package agent

import (
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type ptyDevice interface {
	io.ReadWriteCloser
	Resize(rows, cols uint16) error
}

type unixPTY struct {
	file *os.File
}

func (p *unixPTY) Read(b []byte) (int, error) {
	return p.file.Read(b)
}

func (p *unixPTY) Write(b []byte) (int, error) {
	return p.file.Write(b)
}

func (p *unixPTY) Close() error {
	return p.file.Close()
}

func (p *unixPTY) Resize(rows, cols uint16) error {
	sz := &pty.Winsize{
		Rows: rows,
		Cols: cols,
	}
	return pty.Setsize(p.file, sz)
}

func startPTY(cmd *exec.Cmd, rows, cols uint16) (ptyDevice, error) {
	if rows == 0 {
		rows = 24
	}
	if cols == 0 {
		cols = 80
	}
	sz := &pty.Winsize{
		Rows: rows,
		Cols: cols,
	}
	f, err := pty.StartWithSize(cmd, sz)
	if err != nil {
		return nil, err
	}
	return &unixPTY{file: f}, nil
}
