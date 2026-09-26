//go:build windows

package cmd

import "os"

func notifyWindowChange(chan<- os.Signal) {}
