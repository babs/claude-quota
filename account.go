package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
)

var profileURL = "https://api.anthropic.com/api/oauth/profile"

// AccountInfo holds identity information from the Anthropic profile API.
type AccountInfo struct {
	AccountUUID      string
	EmailAddress     string
	OrganizationUUID string
	OrganizationName string
}

// profileResponse matches the nested JSON returned by GET /api/oauth/profile.
type profileResponse struct {
	Account struct {
		UUID  string `json:"uuid"`
		Email string `json:"email"`
	} `json:"account"`
	Organization struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	} `json:"organization"`
}

// AccountResolver resolves account identity via the profile API with DB-backed cache.
// When stats is nil, uses an in-memory cache keyed by refresh token hash.
type AccountResolver struct {
	client   *http.Client
	stats    *StatsStore
	mu       sync.Mutex // protects lastHash/lastInfo
	lastHash string
	lastInfo AccountInfo
}

// NewAccountResolver creates a resolver.
func NewAccountResolver(client *http.Client, stats *StatsStore) *AccountResolver {
	return &AccountResolver{client: client, stats: stats}
}

// Resolve returns account info for the given credential snapshot.
// DB cache is checked first; on miss, the profile API is called.
// When stats is nil, uses an in-memory cache keyed by refresh token hash.
// On API error, falls back to using refreshTokenHash as AccountUUID.
func (r *AccountResolver) Resolve(snap CredentialSnapshot) AccountInfo {
	if r == nil {
		return AccountInfo{}
	}
	if snap.RefreshTokenHash == "" {
		return AccountInfo{}
	}

	if r.stats != nil {
		return r.resolveWithDB(snap)
	}
	return r.resolveInMemory(snap)
}

func (r *AccountResolver) resolveWithDB(snap CredentialSnapshot) AccountInfo {
	if info, ok := r.stats.LookupAccount(snap.RefreshTokenHash); ok {
		return info
	}

	info, err := r.fetchProfile(snap.AccessToken)
	if err != nil {
		log.Printf("Profile API error: %v — using hash as account ID", err)
		info = AccountInfo{AccountUUID: snap.RefreshTokenHash}
	}

	r.stats.UpsertAccount(snap.RefreshTokenHash, info, snap.SubscriptionType, snap.RateLimitTier)
	return info
}

func (r *AccountResolver) resolveInMemory(snap CredentialSnapshot) AccountInfo {
	r.mu.Lock()
	if r.lastHash == snap.RefreshTokenHash {
		info := r.lastInfo
		r.mu.Unlock()
		return info
	}
	r.mu.Unlock()

	// Fetch outside the lock to avoid holding it during HTTP I/O.
	info, err := r.fetchProfile(snap.AccessToken)
	if err != nil {
		log.Printf("Profile API error: %v — using hash as account ID", err)
		// Cached so we don't hammer a failing API on every poll cycle;
		// clears on token rotation or process restart.
		info = AccountInfo{AccountUUID: snap.RefreshTokenHash}
	}

	r.mu.Lock()
	r.lastHash = snap.RefreshTokenHash
	r.lastInfo = info
	r.mu.Unlock()
	return info
}

// fetchProfile calls GET /api/oauth/profile with the given access token.
func (r *AccountResolver) fetchProfile(accessToken string) (AccountInfo, error) {
	req, err := http.NewRequest("GET", profileURL, nil)
	if err != nil {
		return AccountInfo{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return AccountInfo{}, fmt.Errorf("fetch profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return AccountInfo{}, fmt.Errorf("profile API returned %d", resp.StatusCode)
	}

	var raw profileResponse
	// Limit body to 1MB to prevent memory exhaustion from misbehaving server.
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&raw); err != nil {
		return AccountInfo{}, fmt.Errorf("parse profile: %w", err)
	}
	if raw.Account.UUID == "" {
		log.Printf("Warning: profile API returned empty account UUID — response structure may have changed")
	}
	return AccountInfo{
		AccountUUID:      raw.Account.UUID,
		EmailAddress:     raw.Account.Email,
		OrganizationUUID: raw.Organization.UUID,
		OrganizationName: raw.Organization.Name,
	}, nil
}
