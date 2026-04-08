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
func credentialsPreCheck(provider Provider, path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("%s credentials not found.\n", providerDisplayName(provider))
		fmt.Printf("Expected: %s\n", path)
		fmt.Printf("\nRun '%s' to authenticate %s first.\n", loginCommand(provider), providerDisplayName(provider))
		if runtime.GOOS == "windows" {
			if provider == ProviderClaude {
				fmt.Println("\nIf Claude Code is installed in WSL, use -claude-home to point to")
				fmt.Println(`the WSL home directory, e.g.:`)
				fmt.Println(`  claude-quota -claude-home \\wsl$\<distro>\home\<username>`)
			} else if provider == ProviderCodex {
				fmt.Println("\nIf Codex is installed in WSL, use -codex-home to point to")
				fmt.Println(`the WSL home directory, e.g.:`)
				fmt.Println(`  claude-quota -provider codex -codex-home \\wsl$\<distro>\home\<username>`)
			}
			fmt.Println(`Run "wsl -l -q" to list available WSL distributions.`)
			fmt.Print("\nPress enter to continue...")
			bufio.NewReader(os.Stdin).ReadBytes('\n')
		}
		os.Exit(1)
	}
}
