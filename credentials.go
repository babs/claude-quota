package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrTokenExpired = errors.New("OAuth token has expired — run 'claude login' to re-authenticate")

var credentialsPath string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	credentialsPath = filepath.Join(home, ".claude", ".credentials.json")
}

// credentialsFile represents the on-disk ~/.claude/.credentials.json structure.
type credentialsFile struct {
	ClaudeAiOauth struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresAt    int64  `json:"expiresAt"` // milliseconds since epoch
	} `json:"claudeAiOauth"`
}

// OAuthCredentials manages Claude Code OAuth credentials.
type OAuthCredentials struct {
	mu          sync.Mutex
	accessToken string
	expiresAt   int64 // ms since epoch
}

// NewOAuthCredentials loads credentials from disk and returns a manager.
func NewOAuthCredentials() (*OAuthCredentials, error) {
	oc := &OAuthCredentials{}
	if err := oc.load(); err != nil {
		return nil, err
	}
	return oc, nil
}

// load reads credentials from ~/.claude/.credentials.json.
func (oc *OAuthCredentials) load() error {
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		return fmt.Errorf("cannot read Claude credentials from %s: %w\nRun 'claude login' to authenticate Claude Code first", credentialsPath, err)
	}

	var creds credentialsFile
	if err := json.Unmarshal(data, &creds); err != nil {
		return fmt.Errorf("cannot parse Claude credentials from %s: %w\nRun 'claude login' to authenticate Claude Code first", credentialsPath, err)
	}

	if creds.ClaudeAiOauth.AccessToken == "" {
		return fmt.Errorf("missing OAuth access token in %s\nRun 'claude login' to authenticate Claude Code first", credentialsPath)
	}

	oc.accessToken = creds.ClaudeAiOauth.AccessToken
	oc.expiresAt = creds.ClaudeAiOauth.ExpiresAt
	return nil
}

// isExpired checks if the access token is expired (with 60s margin).
// expiresAt == 0 means unknown expiry; assume valid.
func (oc *OAuthCredentials) isExpired() bool {
	if oc.expiresAt == 0 {
		return false
	}
	nowMs := time.Now().UnixMilli()
	return nowMs >= (oc.expiresAt - 60_000)
}

// GetAccessToken returns a valid access token. On expiry, re-reads the
// credentials file in case Claude Code refreshed the token externally.
func (oc *OAuthCredentials) GetAccessToken() (string, error) {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	if oc.isExpired() {
		log.Println("OAuth token expired, reloading credentials from disk...")
		if err := oc.load(); err != nil {
			return "", fmt.Errorf("%w (reload failed: %v)", ErrTokenExpired, err)
		}
		if oc.isExpired() {
			return "", ErrTokenExpired
		}
		log.Println("Reloaded valid token from disk")
	}
	return oc.accessToken, nil
}
