//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func notifyExtraSignals(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGTERM)
}
