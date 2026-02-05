package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	refreshURL         = "https://console.anthropic.com/v1/oauth/token"
	claudeCodeClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
)

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

// OAuthCredentials manages Claude Code OAuth credentials with auto-refresh.
type OAuthCredentials struct {
	mu           sync.Mutex
	accessToken  string
	refreshToken string
	expiresAt    int64 // ms since epoch
	client       *http.Client
}

// NewOAuthCredentials loads credentials from disk and returns a manager.
func NewOAuthCredentials(client *http.Client) (*OAuthCredentials, error) {
	oc := &OAuthCredentials{client: client}
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

	if creds.ClaudeAiOauth.AccessToken == "" || creds.ClaudeAiOauth.RefreshToken == "" {
		return fmt.Errorf("missing OAuth tokens in %s\nRun 'claude login' to authenticate Claude Code first", credentialsPath)
	}

	oc.accessToken = creds.ClaudeAiOauth.AccessToken
	oc.refreshToken = creds.ClaudeAiOauth.RefreshToken
	oc.expiresAt = creds.ClaudeAiOauth.ExpiresAt
	return nil
}

// isExpired checks if the access token is expired (with 60s margin).
func (oc *OAuthCredentials) isExpired() bool {
	nowMs := time.Now().UnixMilli()
	return nowMs >= (oc.expiresAt - 60_000)
}

// refresh obtains a new access token using the refresh token.
func (oc *OAuthCredentials) refresh() error {
	log.Println("Refreshing OAuth token...")

	body, _ := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": oc.refreshToken,
		"client_id":     claudeCodeClientID,
	})

	resp, err := oc.client.Post(refreshURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("token refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token refresh failed: HTTP %d", resp.StatusCode)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"` // seconds
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse token response: %w", err)
	}

	if result.ExpiresIn == 0 {
		result.ExpiresIn = 3600
	}

	oc.accessToken = result.AccessToken
	oc.refreshToken = result.RefreshToken
	oc.expiresAt = time.Now().UnixMilli() + result.ExpiresIn*1000

	// Persist back to credentials file so Claude Code stays in sync.
	if err := oc.persist(); err != nil {
		log.Printf("Failed to persist refreshed token: %v", err)
	} else {
		log.Printf("Persisted refreshed token to %s", credentialsPath)
	}

	return nil
}

// persist writes the current tokens back to ~/.claude/.credentials.json.
func (oc *OAuthCredentials) persist() error {
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	oauth := map[string]interface{}{
		"accessToken":  oc.accessToken,
		"refreshToken": oc.refreshToken,
		"expiresAt":    oc.expiresAt,
	}
	oauthData, err := json.Marshal(oauth)
	if err != nil {
		return err
	}
	raw["claudeAiOauth"] = oauthData

	out, err := json.Marshal(raw)
	if err != nil {
		return err
	}

	return writeFileSecure(credentialsPath, out)
}

// GetAccessToken returns a valid access token, refreshing if needed.
func (oc *OAuthCredentials) GetAccessToken() (string, error) {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	if oc.isExpired() {
		if err := oc.refresh(); err != nil {
			return "", err
		}
	}
	return oc.accessToken, nil
}
