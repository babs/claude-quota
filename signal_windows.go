//go:build windows

package main

import "os"

func notifyExtraSignals(_ chan<- os.Signal) {
	// No extra signals on Windows; os.Interrupt covers Ctrl+C.
}
