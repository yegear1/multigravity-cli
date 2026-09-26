//go:build !windows

package cmd

import (
	"os"
	"os/signal"
	"syscall"
)

func notifyWindowChange(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGWINCH)
}
