package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTruncate_Short(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate('hello', 10) = %q, want %q", got, "hello")
	}
}

func TestTruncate_Exact(t *testing.T) {
	if got := truncate("hello", 5); got != "hello" {
		t.Errorf("truncate('hello', 5) = %q, want %q", got, "hello")
	}
}

func TestTruncate_Long(t *testing.T) {
	if got := truncate("hello world", 5); got != "hello" {
		t.Errorf("truncate('hello world', 5) = %q, want %q", got, "hello")
	}
}

func TestTruncate_Empty(t *testing.T) {
	if got := truncate("", 5); got != "" {
		t.Errorf("truncate('', 5) = %q, want %q", got, "")
	}
}

func TestTruncate_MultiByteRune(t *testing.T) {
	// "ab—cd" where — is 3 bytes; truncate at 3 runes should give "ab—"
	s := "ab\u2014cd"
	got := truncate(s, 3)
	want := "ab\u2014"
	if got != want {
		t.Errorf("truncate(%q, 3) = %q, want %q", s, got, want)
	}
}

func TestParseBucket_Nil(t *testing.T) {
	var util *float64
	var resets *time.Time
	parseBucket(nil, &util, &resets)
	if util != nil {
		t.Error("parseBucket(nil) set util")
	}
	if resets != nil {
		t.Error("parseBucket(nil) set resets")
	}
}

func TestParseBucket_UtilizationOnly(t *testing.T) {
	v := 42.5
	bucket := &usageBucket{Utilization: &v}
	var util *float64
	var resets *time.Time
	parseBucket(bucket, &util, &resets)
	if util == nil || *util != 42.5 {
		t.Errorf("parseBucket utilization = %v, want 42.5", util)
	}
	if resets != nil {
		t.Error("parseBucket set resets when ResetsAt is nil")
	}
}

func TestParseBucket_Full(t *testing.T) {
	v := 73.0
	r := "2026-02-06T14:30:00Z"
	bucket := &usageBucket{Utilization: &v, ResetsAt: &r}
	var util *float64
	var resets *time.Time
	parseBucket(bucket, &util, &resets)
	if util == nil || *util != 73.0 {
		t.Errorf("parseBucket utilization = %v, want 73.0", util)
	}
	if resets == nil {
		t.Fatal("parseBucket did not set resets")
	}
	expect := time.Date(2026, 2, 6, 14, 30, 0, 0, time.UTC)
	if !resets.Equal(expect) {
		t.Errorf("parseBucket resets = %v, want %v", *resets, expect)
	}
}

func TestParseBucket_WithTimezone(t *testing.T) {
	v := 10.0
	r := "2026-02-06T14:30:00+02:00"
	bucket := &usageBucket{Utilization: &v, ResetsAt: &r}
	var util *float64
	var resets *time.Time
	parseBucket(bucket, &util, &resets)
	if resets == nil {
		t.Fatal("parseBucket did not parse timezone offset")
	}
	// 14:30+02:00 = 12:30 UTC
	expect := time.Date(2026, 2, 6, 12, 30, 0, 0, time.UTC)
	if !resets.UTC().Equal(expect) {
		t.Errorf("parseBucket resets = %v, want %v", resets.UTC(), expect)
	}
}

func TestParseBucket_NilUtilization(t *testing.T) {
	r := "2026-02-06T14:30:00Z"
	bucket := &usageBucket{ResetsAt: &r}
	var util *float64
	var resets *time.Time
	parseBucket(bucket, &util, &resets)
	if util != nil {
		t.Error("parseBucket set util when Utilization is nil")
	}
	if resets == nil {
		t.Error("parseBucket did not set resets")
	}
}

func TestParseBucket_InvalidTime(t *testing.T) {
	r := "not-a-date"
	bucket := &usageBucket{ResetsAt: &r}
	var util *float64
	var resets *time.Time
	parseBucket(bucket, &util, &resets)
	if resets != nil {
		t.Error("parseBucket should not set resets for invalid time")
	}
}

func newTestQuotaClient(token string, expiresAt int64, client *http.Client) *QuotaClient {
	creds := &OAuthCredentials{
		accessToken: token,
		expiresAt:   expiresAt,
	}
	return NewQuotaClient(creds, client)
}

func TestNewQuotaClient(t *testing.T) {
	creds := &OAuthCredentials{accessToken: "tok"}
	client := &http.Client{}
	qc := NewQuotaClient(creds, client)
	if qc.creds != creds {
		t.Error("creds not set")
	}
	if qc.client != client {
		t.Error("client not set")
	}
}

func TestQuotaClient_State(t *testing.T) {
	qc := newTestQuotaClient("tok", time.Now().UnixMilli()+300_000, &http.Client{})
	qc.state.Error = "test error"
	state := qc.State()
	if state.Error != "test error" {
		t.Errorf("State().Error = %q, want %q", state.Error, "test error")
	}
}

func TestQuotaClient_SetError(t *testing.T) {
	qc := newTestQuotaClient("tok", time.Now().UnixMilli()+300_000, &http.Client{})
	v := 42.0
	qc.state.FiveHour = &v

	qc.setError("something broke")

	state := qc.State()
	if state.Error != "something broke" {
		t.Errorf("Error = %q, want %q", state.Error, "something broke")
	}
	if state.FiveHour != nil {
		t.Error("setError should reset full state, but FiveHour is still set")
	}
}

func TestFetch_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(200)
		w.Write([]byte(`{
			"five_hour": {"utilization": 42.5, "resets_at": "2026-02-06T14:30:00Z"},
			"seven_day": {"utilization": 73.0},
			"seven_day_sonnet": {"utilization": 10.0}
		}`))
	}))
	defer srv.Close()

	origURL := usageURL
	defer func() { usageURL = origURL }()
	usageURL = srv.URL

	qc := newTestQuotaClient("test-token", time.Now().UnixMilli()+300_000, srv.Client())
	if !qc.Fetch() {
		t.Fatal("Fetch() returned false")
	}

	state := qc.State()
	if state.Error != "" {
		t.Errorf("Error = %q", state.Error)
	}
	if state.FiveHour == nil || *state.FiveHour != 42.5 {
		t.Errorf("FiveHour = %v, want 42.5", state.FiveHour)
	}
	if state.SevenDay == nil || *state.SevenDay != 73.0 {
		t.Errorf("SevenDay = %v, want 73.0", state.SevenDay)
	}
	if state.LastUpdate == nil {
		t.Error("LastUpdate should be set")
	}
}

func TestFetch_HTTP401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
	}))
	defer srv.Close()

	origURL := usageURL
	defer func() { usageURL = origURL }()
	usageURL = srv.URL

	qc := newTestQuotaClient("tok", time.Now().UnixMilli()+300_000, srv.Client())
	if qc.Fetch() {
		t.Error("Fetch() should return false on 401")
	}
	state := qc.State()
	if state.Error == "" {
		t.Error("Error should be set on 401")
	}
}

func TestFetch_HTTP403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
	}))
	defer srv.Close()

	origURL := usageURL
	defer func() { usageURL = origURL }()
	usageURL = srv.URL

	qc := newTestQuotaClient("tok", time.Now().UnixMilli()+300_000, srv.Client())
	qc.Fetch()
	state := qc.State()
	if state.Error != "Scope missing user:profile" {
		t.Errorf("Error = %q, want scope error", state.Error)
	}
}

func TestFetch_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{not json`))
	}))
	defer srv.Close()

	origURL := usageURL
	defer func() { usageURL = origURL }()
	usageURL = srv.URL

	qc := newTestQuotaClient("tok", time.Now().UnixMilli()+300_000, srv.Client())
	if qc.Fetch() {
		t.Error("Fetch() should return false on invalid JSON")
	}
	state := qc.State()
	if state.Error == "" {
		t.Error("Error should be set on invalid JSON")
	}
}

func TestFetch_TokenExpired(t *testing.T) {
	origCreds := credentialsPath
	defer func() { credentialsPath = origCreds }()
	credentialsPath = "/nonexistent/credentials.json"

	qc := newTestQuotaClient("tok", time.Now().UnixMilli()-1000, &http.Client{})
	if qc.Fetch() {
		t.Error("Fetch() should return false on expired token")
	}
	state := qc.State()
	if !state.TokenExpired {
		t.Error("TokenExpired should be true")
	}
	if !errors.Is(ErrTokenExpired, ErrTokenExpired) {
		t.Error("sanity check failed")
	}
}

func TestFetch_ResetsStaleState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	origURL := usageURL
	defer func() { usageURL = origURL }()
	usageURL = srv.URL

	qc := newTestQuotaClient("tok", time.Now().UnixMilli()+300_000, srv.Client())
	v := 42.0
	qc.state.FiveHour = &v
	qc.state.TokenExpired = true

	qc.Fetch()

	state := qc.State()
	if state.FiveHour != nil {
		t.Error("FiveHour should be nil after error")
	}
	if state.TokenExpired {
		t.Error("TokenExpired should be false after non-token error")
	}
}
