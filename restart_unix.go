//go:build !windows

package main

import (
	"log"
	"os"
	"syscall"
)

// execSelf replaces the current process with a new instance using the same arguments.
func execSelf() {
	log.Printf("Restarting %s %v", executablePath, os.Args[1:])
	if err := syscall.Exec(executablePath, os.Args, os.Environ()); err != nil {
		log.Fatalf("Restart failed: %v", err)
	}
}
