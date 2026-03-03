package main

import (
	"crypto/sha256"
	"encoding/hex"
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
		AccessToken      string `json:"accessToken"`
		RefreshToken     string `json:"refreshToken"`
		ExpiresAt        int64  `json:"expiresAt"` // milliseconds since epoch
		SubscriptionType string `json:"subscriptionType"`
		RateLimitTier    string `json:"rateLimitTier"`
	} `json:"claudeAiOauth"`
}

// OAuthCredentials manages Claude Code OAuth credentials.
type OAuthCredentials struct {
	mu               sync.Mutex
	accessToken      string
	refreshToken     string
	expiresAt        int64 // ms since epoch
	subscriptionType string
	rateLimitTier    string
}

// NewOAuthCredentials loads credentials from disk and returns a manager.
func NewOAuthCredentials() (*OAuthCredentials, error) {
	oc := &OAuthCredentials{}
	if err := oc.load(); err != nil {
		return nil, err
	}
	return oc, nil
}

// loadFromFile reads credentials from ~/.claude/.credentials.json.
func (oc *OAuthCredentials) loadFromFile() error {
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
	oc.refreshToken = creds.ClaudeAiOauth.RefreshToken
	oc.expiresAt = creds.ClaudeAiOauth.ExpiresAt
	oc.subscriptionType = creds.ClaudeAiOauth.SubscriptionType
	oc.rateLimitTier = creds.ClaudeAiOauth.RateLimitTier
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

// CredentialSnapshot holds a consistent point-in-time view of credentials.
type CredentialSnapshot struct {
	Changed          bool
	AccessToken      string
	RefreshTokenHash string
	SubscriptionType string
	RateLimitTier    string
}

// ReloadAndSnapshot re-reads credentials from disk and returns a consistent
// snapshot under a single lock, avoiding TOCTOU between reload/token/hash.
func (oc *OAuthCredentials) ReloadAndSnapshot() (CredentialSnapshot, error) {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	prev := oc.refreshToken
	if err := oc.load(); err != nil {
		return CredentialSnapshot{}, err
	}
	if oc.isExpired() {
		return CredentialSnapshot{}, ErrTokenExpired
	}

	var hash string
	if oc.refreshToken != "" {
		h := sha256.Sum256([]byte(oc.refreshToken))
		hash = hex.EncodeToString(h[:])
	}

	return CredentialSnapshot{
		Changed:          oc.refreshToken != prev,
		AccessToken:      oc.accessToken,
		RefreshTokenHash: hash,
		SubscriptionType: oc.subscriptionType,
		RateLimitTier:    oc.rateLimitTier,
	}, nil
}

// RefreshTokenHash returns SHA256 hex of the refresh token, or empty if absent.
func (oc *OAuthCredentials) RefreshTokenHash() string {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	if oc.refreshToken == "" {
		return ""
	}
	h := sha256.Sum256([]byte(oc.refreshToken))
	return hex.EncodeToString(h[:])
}

// SubscriptionType returns the subscription type from the credentials file.
func (oc *OAuthCredentials) SubscriptionType() string {
	oc.mu.Lock()
	defer oc.mu.Unlock()
	return oc.subscriptionType
}

// RateLimitTier returns the rate limit tier from the credentials file.
func (oc *OAuthCredentials) RateLimitTier() string {
	oc.mu.Lock()
	defer oc.mu.Unlock()
	return oc.rateLimitTier
}
