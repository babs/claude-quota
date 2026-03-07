//go:build windows

package main

import (
	"log"
	"os"
)

// execSelf starts a new instance with the same arguments and exits.
func execSelf() {
	log.Printf("Restarting %s %v", executablePath, os.Args[1:])
	proc, err := os.StartProcess(executablePath, os.Args, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Env:   os.Environ(),
	})
	if err != nil {
		log.Fatalf("Restart failed: %v", err)
	}
	_ = proc.Release()
	os.Exit(0)
}
