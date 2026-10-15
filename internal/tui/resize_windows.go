//go:build windows

package tui

import (
	"sync"
	"time"

	"golang.org/x/term"
)

var resizePollInterval = 150 * time.Millisecond

// WatchResize monitors terminal window size changes on Windows by tracking dimension changes.
func WatchResize(fd uintptr, onResize func(rows, cols uint16)) func() {
	if onResize == nil {
		return func() {}
	}

	stopChan := make(chan struct{})
	var once sync.Once

	go func() {
		lastW, lastH := 0, 0
		if w, h, err := term.GetSize(int(fd)); err == nil && w > 0 && h > 0 {
			lastW, lastH = w, h
		}

		ticker := time.NewTicker(resizePollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if w, h, err := term.GetSize(int(fd)); err == nil && w > 0 && h > 0 {
					if w != lastW || h != lastH {
						lastW, lastH = w, h
						onResize(uint16(h), uint16(w))
					}
				}
			case <-stopChan:
				return
			}
		}
	}()

	return func() {
		once.Do(func() {
			close(stopChan)
		})
	}
}
