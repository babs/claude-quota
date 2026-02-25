//go:build darwin

package main

import "fmt"

func init() {
	// Disable real Keychain lookups during tests so that file-based tests
	// (which control credentialsPath) behave identically across all platforms.
	loadKeychainFn = func() (*credentialsFile, error) {
		return nil, fmt.Errorf("keychain disabled in tests")
	}
}
