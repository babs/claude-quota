package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func testSnap(token, hash string) CredentialSnapshot {
	return CredentialSnapshot{
		AccessToken:      token,
		RefreshTokenHash: hash,
	}
}

func TestFetchProfile_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(200)
		w.Write([]byte(`{
			"account": {"uuid": "acct-uuid-123", "email": "user@example.com"},
			"organization": {"uuid": "org-uuid-456", "name": "My Org"}
		}`))
	}))
	defer srv.Close()

	origURL := profileURL
	defer func() { profileURL = origURL }()
	profileURL = srv.URL

	resolver := NewAccountResolver(srv.Client(), nil)
	info, err := resolver.fetchProfile("test-token")
	if err != nil {
		t.Fatalf("fetchProfile() error: %v", err)
	}
	if info.AccountUUID != "acct-uuid-123" {
		t.Errorf("AccountUUID = %q, want acct-uuid-123", info.AccountUUID)
	}
	if info.EmailAddress != "user@example.com" {
		t.Errorf("EmailAddress = %q, want user@example.com", info.EmailAddress)
	}
	if info.OrganizationUUID != "org-uuid-456" {
		t.Errorf("OrganizationUUID = %q, want org-uuid-456", info.OrganizationUUID)
	}
	if info.OrganizationName != "My Org" {
		t.Errorf("OrganizationName = %q, want 'My Org'", info.OrganizationName)
	}
}

func TestFetchProfile_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
	}))
	defer srv.Close()

	origURL := profileURL
	defer func() { profileURL = origURL }()
	profileURL = srv.URL

	resolver := NewAccountResolver(srv.Client(), nil)
	_, err := resolver.fetchProfile("tok")
	if err == nil {
		t.Error("fetchProfile() should fail on non-200")
	}
}

func TestFetchProfile_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{not json`))
	}))
	defer srv.Close()

	origURL := profileURL
	defer func() { profileURL = origURL }()
	profileURL = srv.URL

	resolver := NewAccountResolver(srv.Client(), nil)
	_, err := resolver.fetchProfile("tok")
	if err == nil {
		t.Error("fetchProfile() should fail on invalid JSON")
	}
}

func TestResolve_NilResolver(t *testing.T) {
	var r *AccountResolver
	info := r.Resolve(testSnap("token", "hash"))
	if info.AccountUUID != "" {
		t.Errorf("nil resolver should return empty AccountInfo, got %+v", info)
	}
}

func TestResolve_NilStats_CallsAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"account":{"uuid":"uuid-no-db","email":"no-db@example.com"},"organization":{"uuid":"org-1","name":"No DB Org"}}`))
	}))
	defer srv.Close()

	origURL := profileURL
	defer func() { profileURL = origURL }()
	profileURL = srv.URL

	r := NewAccountResolver(srv.Client(), nil)
	info := r.Resolve(testSnap("token", "hash"))
	if info.AccountUUID != "uuid-no-db" {
		t.Errorf("AccountUUID = %q, want uuid-no-db", info.AccountUUID)
	}
	if info.EmailAddress != "no-db@example.com" {
		t.Errorf("EmailAddress = %q, want no-db@example.com", info.EmailAddress)
	}
	if info.OrganizationName != "No DB Org" {
		t.Errorf("OrganizationName = %q, want 'No DB Org'", info.OrganizationName)
	}
}

func TestResolve_NilStats_InMemoryCache(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(200)
		w.Write([]byte(`{"account":{"uuid":"cached","email":"c@d.com"},"organization":{}}`))
	}))
	defer srv.Close()

	origURL := profileURL
	defer func() { profileURL = origURL }()
	profileURL = srv.URL

	r := NewAccountResolver(srv.Client(), nil)
	r.Resolve(testSnap("token", "hash"))
	r.Resolve(testSnap("token", "hash"))
	if c := calls.Load(); c != 1 {
		t.Errorf("API called %d times, want 1 (second call should use in-memory cache)", c)
	}
}

func TestResolve_NilStats_InMemoryCacheInvalidation(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(200)
		w.Write([]byte(`{"account":{"uuid":"u-` + r.Header.Get("Authorization") + `","email":"a@b.com"},"organization":{}}`))
	}))
	defer srv.Close()

	origURL := profileURL
	defer func() { profileURL = origURL }()
	profileURL = srv.URL

	r := NewAccountResolver(srv.Client(), nil)
	info1 := r.Resolve(testSnap("tok1", "hash-1"))
	info2 := r.Resolve(testSnap("tok2", "hash-2"))
	if c := calls.Load(); c != 2 {
		t.Errorf("API called %d times, want 2 (different hash should trigger new call)", c)
	}
	if info1.AccountUUID == info2.AccountUUID {
		t.Errorf("different hashes returned same AccountUUID %q", info1.AccountUUID)
	}
}

func TestResolve_EmptyHash(t *testing.T) {
	origPath := statsDBPath
	statsDBPath = filepath.Join(t.TempDir(), "stats.db")
	defer func() { statsDBPath = origPath }()

	store, _ := NewStatsStore()
	defer store.Close()

	r := NewAccountResolver(&http.Client{}, store)
	info := r.Resolve(testSnap("token", ""))
	if info.AccountUUID != "" {
		t.Errorf("empty hash should return empty AccountInfo, got %+v", info)
	}
}

func TestResolve_CacheHit(t *testing.T) {
	origPath := statsDBPath
	statsDBPath = filepath.Join(t.TempDir(), "stats.db")
	defer func() { statsDBPath = origPath }()

	store, _ := NewStatsStore()
	defer store.Close()

	// Pre-populate cache.
	store.UpsertAccount("hash-123", AccountInfo{
		AccountUUID:  "cached-uuid",
		EmailAddress: "cached@example.com",
	}, "", "")

	// Server should NOT be called.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("profile API should not be called on cache hit")
		w.WriteHeader(500)
	}))
	defer srv.Close()

	origURL := profileURL
	defer func() { profileURL = origURL }()
	profileURL = srv.URL

	r := NewAccountResolver(srv.Client(), store)
	info := r.Resolve(testSnap("token", "hash-123"))
	if info.AccountUUID != "cached-uuid" {
		t.Errorf("AccountUUID = %q, want cached-uuid", info.AccountUUID)
	}
}

func TestResolve_CacheMiss(t *testing.T) {
	origPath := statsDBPath
	statsDBPath = filepath.Join(t.TempDir(), "stats.db")
	defer func() { statsDBPath = origPath }()

	store, _ := NewStatsStore()
	defer store.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"account":{"uuid":"api-uuid","email":"api@example.com"},"organization":{}}`))
	}))
	defer srv.Close()

	origURL := profileURL
	defer func() { profileURL = origURL }()
	profileURL = srv.URL

	r := NewAccountResolver(srv.Client(), store)
	info := r.Resolve(testSnap("token", "hash-new"))
	if info.AccountUUID != "api-uuid" {
		t.Errorf("AccountUUID = %q, want api-uuid", info.AccountUUID)
	}

	// Verify it was cached.
	cached, ok := store.LookupAccount("hash-new")
	if !ok {
		t.Fatal("account should have been cached after API call")
	}
	if cached.AccountUUID != "api-uuid" {
		t.Errorf("cached AccountUUID = %q, want api-uuid", cached.AccountUUID)
	}
}

func TestResolve_APIErrorFallback(t *testing.T) {
	origPath := statsDBPath
	statsDBPath = filepath.Join(t.TempDir(), "stats.db")
	defer func() { statsDBPath = origPath }()

	store, _ := NewStatsStore()
	defer store.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	origURL := profileURL
	defer func() { profileURL = origURL }()
	profileURL = srv.URL

	r := NewAccountResolver(srv.Client(), store)
	info := r.Resolve(testSnap("token", "hash-fallback"))
	// Should fall back to using the hash as UUID.
	if info.AccountUUID != "hash-fallback" {
		t.Errorf("AccountUUID = %q, want hash-fallback (fallback)", info.AccountUUID)
	}
}

func TestResolve_PassesSubscriptionInfo(t *testing.T) {
	origPath := statsDBPath
	statsDBPath = filepath.Join(t.TempDir(), "stats.db")
	defer func() { statsDBPath = origPath }()

	store, _ := NewStatsStore()
	defer store.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"account":{"uuid":"uuid-1","email":"a@b.com"},"organization":{}}`))
	}))
	defer srv.Close()

	origURL := profileURL
	defer func() { profileURL = origURL }()
	profileURL = srv.URL

	r := NewAccountResolver(srv.Client(), store)
	snap := CredentialSnapshot{
		AccessToken:      "tok",
		RefreshTokenHash: "hash-sub",
		SubscriptionType: "pro",
		RateLimitTier:    "tier4",
	}
	r.Resolve(snap)

	// Verify subscription info was persisted.
	var subType, rateLimitTier sql.NullString
	err := store.db.QueryRow(`
		SELECT subscription_type, rate_limit_tier
		FROM accounts WHERE refresh_token_hash = 'hash-sub'`,
	).Scan(&subType, &rateLimitTier)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if !subType.Valid || subType.String != "pro" {
		t.Errorf("subscription_type = %v, want pro", subType)
	}
	if !rateLimitTier.Valid || rateLimitTier.String != "tier4" {
		t.Errorf("rate_limit_tier = %v, want tier4", rateLimitTier)
	}
}
