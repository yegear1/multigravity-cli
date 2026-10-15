//go:build !windows

package tui

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/term"
)

// WatchResize monitors terminal window size changes on POSIX systems via SIGWINCH.
func WatchResize(fd uintptr, onResize func(rows, cols uint16)) func() {
	if onResize == nil {
		return func() {}
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)

	stopChan := make(chan struct{})
	var once sync.Once

	go func() {
		for {
			select {
			case <-sigChan:
				if w, h, err := term.GetSize(int(fd)); err == nil && w > 0 && h > 0 {
					onResize(uint16(h), uint16(w))
				}
			case <-stopChan:
				return
			}
		}
	}()

	return func() {
		once.Do(func() {
			signal.Stop(sigChan)
			close(stopChan)
		})
	}
}
