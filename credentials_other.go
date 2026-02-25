//go:build !darwin

package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
)

// load reads credentials from the JSON file (non-macOS platforms).
func (oc *OAuthCredentials) load() error {
	return oc.loadFromFile()
}

// credentialsPreCheck verifies the credentials file exists before loading.
// On Windows it additionally prints WSL guidance.
func credentialsPreCheck() {
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		fmt.Println("Claude Code credentials not found.")
		fmt.Printf("Expected: %s\n", credentialsPath)
		fmt.Println("\nRun 'claude login' to authenticate Claude Code first.")
		if runtime.GOOS == "windows" {
			fmt.Println("\nIf Claude Code is installed in WSL, use -claude-home to point to")
			fmt.Println(`the WSL home directory, e.g.:`)
			fmt.Println(`  claude-quota -claude-home \\wsl$\<distro>\home\<username>`)
			fmt.Println(`Run "wsl -l -q" to list available WSL distributions.`)
			fmt.Print("\nPress enter to continue...")
			bufio.NewReader(os.Stdin).ReadBytes('\n')
		}
		os.Exit(1)
	}
}
