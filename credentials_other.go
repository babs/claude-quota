//go:build !darwin

package main

// load reads credentials from the JSON file (non-macOS platforms).
func (oc *OAuthCredentials) load() error {
	return oc.loadFromFile()
}
