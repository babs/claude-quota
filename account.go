package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
type AccountResolver struct {
	client *http.Client
	stats  *StatsStore
}

// NewAccountResolver creates a resolver. If stats is nil, the resolver degrades to no-op.
func NewAccountResolver(client *http.Client, stats *StatsStore) *AccountResolver {
	return &AccountResolver{client: client, stats: stats}
}

// Resolve returns account info for the given credential snapshot.
// DB cache is checked first; on miss, the profile API is called.
// On API error, falls back to using refreshTokenHash as AccountUUID.
func (r *AccountResolver) Resolve(snap CredentialSnapshot) AccountInfo {
	if r == nil || r.stats == nil {
		return AccountInfo{}
	}
	if snap.RefreshTokenHash == "" {
		return AccountInfo{}
	}

	// DB cache hit.
	if info, ok := r.stats.LookupAccount(snap.RefreshTokenHash); ok {
		return info
	}

	// Cache miss — call profile API.
	info, err := r.fetchProfile(snap.AccessToken)
	if err != nil {
		log.Printf("Profile API error: %v — using hash as account ID", err)
		info = AccountInfo{AccountUUID: snap.RefreshTokenHash}
	}

	r.stats.UpsertAccount(snap.RefreshTokenHash, info, snap.SubscriptionType, snap.RateLimitTier)
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
