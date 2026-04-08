package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"sync"
	"time"
)

var ErrTokenExpired = errors.New("OAuth token expired")

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

type codexAuthFile struct {
	AuthMode string `json:"auth_mode"`
	Tokens   struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		AccountID    string `json:"account_id"`
	} `json:"tokens"`
	LastRefresh string `json:"last_refresh"`
}

// OAuthCredentials manages Claude Code or Codex OAuth credentials.
type OAuthCredentials struct {
	mu               sync.Mutex
	provider         Provider
	path             string
	accessToken      string
	refreshToken     string
	accountID        string
	expiresAt        int64 // ms since epoch
	subscriptionType string
	rateLimitTier    string
}

// NewOAuthCredentials loads credentials from disk and returns a manager.
func NewOAuthCredentials(provider Provider) (*OAuthCredentials, error) {
	oc := &OAuthCredentials{provider: provider}
	if err := oc.load(); err != nil {
		return nil, err
	}
	return oc, nil
}

func (oc *OAuthCredentials) credentialPath() string {
	if oc.path != "" {
		return oc.path
	}
	if oc.provider == ProviderCodex {
		return codexAuthPath
	}
	return claudeCredentialsPath
}

func (oc *OAuthCredentials) loginCommand() string {
	if oc.provider == ProviderCodex {
		return "codex login"
	}
	return "claude login"
}

// loadFromFile reads provider-specific credentials from disk.
func (oc *OAuthCredentials) loadFromFile() error {
	path := oc.credentialPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read %s credentials from %s: %w\nRun '%s' to authenticate %s first",
			providerDisplayName(oc.provider), path, err, oc.loginCommand(), providerDisplayName(oc.provider))
	}

	switch oc.provider {
	case ProviderCodex:
		var auth codexAuthFile
		if err := json.Unmarshal(data, &auth); err != nil {
			return fmt.Errorf("cannot parse %s credentials from %s: %w\nRun '%s' to authenticate %s first",
				providerDisplayName(oc.provider), path, err, oc.loginCommand(), providerDisplayName(oc.provider))
		}
		if auth.Tokens.AccessToken == "" {
			return fmt.Errorf("missing OAuth access token in %s\nRun '%s' to authenticate %s first",
				path, oc.loginCommand(), providerDisplayName(oc.provider))
		}
		oc.accessToken = auth.Tokens.AccessToken
		oc.refreshToken = auth.Tokens.RefreshToken
		oc.accountID = auth.Tokens.AccountID
		oc.expiresAt = jwtExpiryMillis(auth.Tokens.AccessToken)
		oc.subscriptionType = auth.AuthMode
		oc.rateLimitTier = ""
	default:
		var creds credentialsFile
		if err := json.Unmarshal(data, &creds); err != nil {
			return fmt.Errorf("cannot parse %s credentials from %s: %w\nRun '%s' to authenticate %s first",
				providerDisplayName(oc.provider), path, err, oc.loginCommand(), providerDisplayName(oc.provider))
		}
		if creds.ClaudeAiOauth.AccessToken == "" {
			return fmt.Errorf("missing OAuth access token in %s\nRun '%s' to authenticate %s first",
				path, oc.loginCommand(), providerDisplayName(oc.provider))
		}
		oc.accessToken = creds.ClaudeAiOauth.AccessToken
		oc.refreshToken = creds.ClaudeAiOauth.RefreshToken
		oc.accountID = ""
		oc.expiresAt = creds.ClaudeAiOauth.ExpiresAt
		oc.subscriptionType = creds.ClaudeAiOauth.SubscriptionType
		oc.rateLimitTier = creds.ClaudeAiOauth.RateLimitTier
	}

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
// credentials file in case the CLI refreshed the token externally.
func (oc *OAuthCredentials) GetAccessToken() (string, error) {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	if oc.isExpired() {
		log.Printf("%s OAuth token expired, reloading credentials from disk...", providerDisplayName(oc.provider))
		if err := oc.load(); err != nil {
			return "", fmt.Errorf("%w (reload failed: %v)", ErrTokenExpired, err)
		}
		if oc.isExpired() {
			return "", ErrTokenExpired
		}
		log.Printf("Reloaded valid %s token from disk", providerDisplayName(oc.provider))
	}
	return oc.accessToken, nil
}

// Provider returns the credential provider.
func (oc *OAuthCredentials) Provider() Provider {
	oc.mu.Lock()
	defer oc.mu.Unlock()
	return oc.provider
}

// AccountID returns the provider account identifier when available.
func (oc *OAuthCredentials) AccountID() string {
	oc.mu.Lock()
	defer oc.mu.Unlock()
	return oc.accountID
}

// CredentialSnapshot holds a consistent point-in-time view of credentials.
type CredentialSnapshot struct {
	Changed          bool
	AccessToken      string
	AccountID        string
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
		AccountID:        oc.accountID,
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

// jwtExpiryMillis extracts the "exp" claim from a JWT without signature
// verification. This is intentional: we only need a rough expiry check to
// decide when to reload credentials from disk — worst case is an extra read.
func jwtExpiryMillis(token string) int64 {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return 0
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0
	}
	if claims.Exp <= 0 || claims.Exp > math.MaxInt64/1000 {
		return 0
	}
	return claims.Exp * 1000
}
