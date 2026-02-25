//go:build darwin

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"os/user"
	"strings"
)

// keychainService is the service name used by Claude Code (via keytar) in the macOS Keychain.
// The account is the current OS username.
const keychainService = "Claude Code-credentials"

// loadKeychainFn is the function used to fetch credentials from the Keychain.
// It can be overridden in tests to bypass the real Keychain lookup.
var loadKeychainFn = loadFromKeychain

// load tries to read credentials from the macOS Keychain first.
// If the Keychain lookup fails for any reason, it falls back to the JSON file.
func (oc *OAuthCredentials) load() error {
	creds, err := loadKeychainFn()
	if err != nil {
		log.Printf("Keychain lookup failed (%v), falling back to credentials file", err)
		return oc.loadFromFile()
	}
	if creds.ClaudeAiOauth.AccessToken == "" {
		log.Printf("Keychain entry found but accessToken is empty, falling back to credentials file")
		return oc.loadFromFile()
	}
	oc.accessToken = creds.ClaudeAiOauth.AccessToken
	oc.expiresAt = creds.ClaudeAiOauth.ExpiresAt
	return nil
}

// loadFromKeychain retrieves OAuth credentials stored by Claude Code in the macOS Keychain.
// The account is the current OS username; the password is a JSON blob with the full
// credentials structure: {"claudeAiOauth": {"accessToken": "...", "expiresAt": ..., ...}}
func loadFromKeychain() (*credentialsFile, error) {
	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("cannot determine current user: %w", err)
	}

	out, err := exec.Command("security", "find-generic-password",
		"-s", keychainService, "-a", u.Username, "-w").Output()
	if err != nil {
		return nil, fmt.Errorf("security command failed: %w", err)
	}

	var creds credentialsFile
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &creds); err != nil {
		return nil, fmt.Errorf("cannot parse keychain JSON: %w", err)
	}
	return &creds, nil
}
