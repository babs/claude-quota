package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsExpired_ZeroExpiresAt(t *testing.T) {
	oc := &OAuthCredentials{expiresAt: 0}
	if oc.isExpired() {
		t.Error("isExpired() should return false when expiresAt is 0")
	}
}

func TestIsExpired_Future(t *testing.T) {
	oc := &OAuthCredentials{expiresAt: time.Now().UnixMilli() + 120_000}
	if oc.isExpired() {
		t.Error("isExpired() should return false for token expiring in 2 minutes")
	}
}

func TestIsExpired_Past(t *testing.T) {
	oc := &OAuthCredentials{expiresAt: time.Now().UnixMilli() - 1000}
	if !oc.isExpired() {
		t.Error("isExpired() should return true for token expired 1s ago")
	}
}

func TestIsExpired_WithinMargin(t *testing.T) {
	oc := &OAuthCredentials{expiresAt: time.Now().UnixMilli() + 30_000}
	if !oc.isExpired() {
		t.Error("isExpired() should return true when within 60s margin")
	}
}

func writeTestCredentials(t *testing.T, token string, expiresAt int64) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	creds := map[string]any{
		"claudeAiOauth": map[string]any{
			"accessToken": token,
			"expiresAt":   expiresAt,
		},
	}
	data, err := json.Marshal(creds)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_Valid(t *testing.T) {
	orig := credentialsPath
	defer func() { credentialsPath = orig }()

	credentialsPath = writeTestCredentials(t, "test-token", time.Now().UnixMilli()+300_000)

	oc := &OAuthCredentials{}
	if err := oc.load(); err != nil {
		t.Fatalf("load() error: %v", err)
	}
	if oc.accessToken != "test-token" {
		t.Errorf("accessToken = %q, want %q", oc.accessToken, "test-token")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	orig := credentialsPath
	defer func() { credentialsPath = orig }()

	credentialsPath = "/nonexistent/path/credentials.json"

	oc := &OAuthCredentials{}
	if err := oc.load(); err == nil {
		t.Error("load() should fail for missing file")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	orig := credentialsPath
	defer func() { credentialsPath = orig }()

	dir := t.TempDir()
	credentialsPath = filepath.Join(dir, "credentials.json")
	os.WriteFile(credentialsPath, []byte("{invalid"), 0600)

	oc := &OAuthCredentials{}
	if err := oc.load(); err == nil {
		t.Error("load() should fail for invalid JSON")
	}
}

func TestLoad_EmptyToken(t *testing.T) {
	orig := credentialsPath
	defer func() { credentialsPath = orig }()

	credentialsPath = writeTestCredentials(t, "", 0)

	oc := &OAuthCredentials{}
	if err := oc.load(); err == nil {
		t.Error("load() should fail when accessToken is empty")
	}
}

func TestNewOAuthCredentials_Valid(t *testing.T) {
	orig := credentialsPath
	defer func() { credentialsPath = orig }()

	credentialsPath = writeTestCredentials(t, "tok", time.Now().UnixMilli()+300_000)

	oc, err := NewOAuthCredentials()
	if err != nil {
		t.Fatalf("NewOAuthCredentials() error: %v", err)
	}
	if oc.accessToken != "tok" {
		t.Errorf("accessToken = %q, want %q", oc.accessToken, "tok")
	}
}

func TestNewOAuthCredentials_MissingFile(t *testing.T) {
	orig := credentialsPath
	defer func() { credentialsPath = orig }()

	credentialsPath = "/nonexistent/credentials.json"

	_, err := NewOAuthCredentials()
	if err == nil {
		t.Error("NewOAuthCredentials() should fail for missing file")
	}
}

func TestGetAccessToken_Valid(t *testing.T) {
	oc := &OAuthCredentials{
		accessToken: "valid-token",
		expiresAt:   time.Now().UnixMilli() + 300_000,
	}
	tok, err := oc.GetAccessToken()
	if err != nil {
		t.Fatalf("GetAccessToken() error: %v", err)
	}
	if tok != "valid-token" {
		t.Errorf("token = %q, want %q", tok, "valid-token")
	}
}

func TestGetAccessToken_ExpiredReloadSuccess(t *testing.T) {
	orig := credentialsPath
	defer func() { credentialsPath = orig }()

	futureMs := time.Now().UnixMilli() + 300_000
	credentialsPath = writeTestCredentials(t, "refreshed-token", futureMs)

	oc := &OAuthCredentials{
		accessToken: "old-token",
		expiresAt:   time.Now().UnixMilli() - 1000,
	}
	tok, err := oc.GetAccessToken()
	if err != nil {
		t.Fatalf("GetAccessToken() error: %v", err)
	}
	if tok != "refreshed-token" {
		t.Errorf("token = %q, want %q", tok, "refreshed-token")
	}
}

func TestGetAccessToken_ExpiredReloadStillExpired(t *testing.T) {
	orig := credentialsPath
	defer func() { credentialsPath = orig }()

	pastMs := time.Now().UnixMilli() - 1000
	credentialsPath = writeTestCredentials(t, "still-expired", pastMs)

	oc := &OAuthCredentials{
		accessToken: "old-token",
		expiresAt:   time.Now().UnixMilli() - 2000,
	}
	_, err := oc.GetAccessToken()
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got: %v", err)
	}
}

func TestGetAccessToken_ExpiredReloadFails(t *testing.T) {
	orig := credentialsPath
	defer func() { credentialsPath = orig }()

	credentialsPath = "/nonexistent/credentials.json"

	oc := &OAuthCredentials{
		accessToken: "old-token",
		expiresAt:   time.Now().UnixMilli() - 1000,
	}
	_, err := oc.GetAccessToken()
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired wrapped, got: %v", err)
	}
}
