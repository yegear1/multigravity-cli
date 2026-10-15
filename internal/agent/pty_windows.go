//go:build windows

package agent

import (
	"io"
	"os"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

type ptyDevice interface {
	io.ReadWriteCloser
	Resize(rows, cols uint16) error
}

type windowsPipePTY struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
	rows   uint16
	cols   uint16
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
	if rows == 0 {
		rows = 24
	}
	if cols == 0 {
		cols = 80
	}
	p.rows = rows
	p.cols = cols
	return nil
}

type windowsConPTY struct {
	hPC    windows.Handle
	stdin  *os.File
	stdout *os.File
	rows   uint16
	cols   uint16
}

func (p *windowsConPTY) Read(b []byte) (int, error) {
	return p.stdout.Read(b)
}

func (p *windowsConPTY) Write(b []byte) (int, error) {
	return p.stdin.Write(b)
}

func (p *windowsConPTY) Close() error {
	if p.hPC != 0 {
		windows.ClosePseudoConsole(p.hPC)
		p.hPC = 0
	}
	err1 := p.stdin.Close()
	err2 := p.stdout.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

func (p *windowsConPTY) Resize(rows, cols uint16) error {
	if rows == 0 {
		rows = 24
	}
	if cols == 0 {
		cols = 80
	}
	p.rows = rows
	p.cols = cols
	if p.hPC != 0 {
		coord := windows.Coord{X: int16(cols), Y: int16(rows)}
		return windows.ResizePseudoConsole(p.hPC, coord)
	}
	return nil
}

func createUnicodeEnvironmentBlock(env []string) *uint16 {
	if len(env) == 0 {
		return nil
	}
	var u16 []uint16
	for _, entry := range env {
		p, err := windows.UTF16FromString(entry)
		if err == nil {
			u16 = append(u16, p...)
		}
	}
	u16 = append(u16, 0)
	return &u16[0]
}

func startConPTY(cmd *exec.Cmd, rows, cols uint16) (ptyDevice, error) {
	var inPipeRead, inPipeWrite windows.Handle
	if err := windows.CreatePipe(&inPipeRead, &inPipeWrite, nil, 0); err != nil {
		return nil, err
	}

	var outPipeRead, outPipeWrite windows.Handle
	if err := windows.CreatePipe(&outPipeRead, &outPipeWrite, nil, 0); err != nil {
		_ = windows.CloseHandle(inPipeRead)
		_ = windows.CloseHandle(inPipeWrite)
		return nil, err
	}

	coord := windows.Coord{X: int16(cols), Y: int16(rows)}
	var hPC windows.Handle
	if err := windows.CreatePseudoConsole(coord, inPipeRead, outPipeWrite, 0, &hPC); err != nil {
		_ = windows.CloseHandle(inPipeRead)
		_ = windows.CloseHandle(inPipeWrite)
		_ = windows.CloseHandle(outPipeRead)
		_ = windows.CloseHandle(outPipeWrite)
		return nil, err
	}

	// Pseudo console duplicates or takes ownership of these pipe ends
	_ = windows.CloseHandle(inPipeRead)
	_ = windows.CloseHandle(outPipeWrite)

	attrList, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		windows.ClosePseudoConsole(hPC)
		_ = windows.CloseHandle(inPipeWrite)
		_ = windows.CloseHandle(outPipeRead)
		return nil, err
	}
	defer attrList.Delete()

	if err := attrList.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, unsafe.Pointer(hPC), unsafe.Sizeof(hPC)); err != nil {
		windows.ClosePseudoConsole(hPC)
		_ = windows.CloseHandle(inPipeWrite)
		_ = windows.CloseHandle(outPipeRead)
		return nil, err
	}

	var siEx windows.StartupInfoEx
	siEx.Cb = uint32(unsafe.Sizeof(siEx))
	siEx.ProcThreadAttributeList = attrList.List()

	cmdArgs := cmd.Args
	if len(cmdArgs) == 0 {
		cmdArgs = []string{cmd.Path}
	}
	cmdLine := windows.ComposeCommandLine(cmdArgs)
	cmdLinePtr, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		windows.ClosePseudoConsole(hPC)
		_ = windows.CloseHandle(inPipeWrite)
		_ = windows.CloseHandle(outPipeRead)
		return nil, err
	}

	var dirPtr *uint16
	if cmd.Dir != "" {
		dirPtr, _ = windows.UTF16PtrFromString(cmd.Dir)
	}

	envPtr := createUnicodeEnvironmentBlock(cmd.Env)
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT)

	var pi windows.ProcessInformation
	if err := windows.CreateProcess(nil, cmdLinePtr, nil, nil, false, flags, envPtr, dirPtr, &siEx.StartupInfo, &pi); err != nil {
		windows.ClosePseudoConsole(hPC)
		_ = windows.CloseHandle(inPipeWrite)
		_ = windows.CloseHandle(outPipeRead)
		return nil, err
	}

	_ = windows.CloseHandle(pi.Thread)
	cmd.Process, _ = os.FindProcess(int(pi.ProcessId))

	return &windowsConPTY{
		hPC:    hPC,
		stdin:  os.NewFile(uintptr(inPipeWrite), "conpty-stdin"),
		stdout: os.NewFile(uintptr(outPipeRead), "conpty-stdout"),
		rows:   rows,
		cols:   cols,
	}, nil
}

func startPipePTY(cmd *exec.Cmd, rows, cols uint16) (ptyDevice, error) {
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdinPipe.Close()
		return nil, err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		_ = stdinPipe.Close()
		_ = stdoutPipe.Close()
		return nil, err
	}

	return &windowsPipePTY{
		stdin:  stdinPipe,
		stdout: stdoutPipe,
		rows:   rows,
		cols:   cols,
	}, nil
}

func startPTY(cmd *exec.Cmd, rows, cols uint16) (ptyDevice, error) {
	if rows == 0 {
		rows = 24
	}
	if cols == 0 {
		cols = 80
	}

	dev, err := startConPTY(cmd, rows, cols)
	if err == nil {
		return dev, nil
	}

	return startPipePTY(cmd, rows, cols)
}
