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
	// Reset time is in the past (2026-02-06), so projection should be nil.
	if state.FiveHourProjected != nil {
		t.Errorf("FiveHourProjected should be nil for past reset, got %f", *state.FiveHourProjected)
	}
	// No resets_at for 7d, so projection should be nil.
	if state.SevenDayProjected != nil {
		t.Errorf("SevenDayProjected should be nil without resets_at, got %f", *state.SevenDayProjected)
	}
}

func TestFetch_ComputesProjection(t *testing.T) {
	// Use a reset time 1h in the future so the projection pipeline runs.
	resetsAt := time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{
			"five_hour": {"utilization": 50.0, "resets_at": "` + resetsAt + `"},
			"seven_day": {"utilization": 10.0}
		}`))
	}))
	defer srv.Close()

	origURL := usageURL
	defer func() { usageURL = origURL }()
	usageURL = srv.URL

	qc := newTestQuotaClient("tok", time.Now().UnixMilli()+300_000, srv.Client())
	if !qc.Fetch() {
		t.Fatal("Fetch() returned false")
	}

	state := qc.State()
	if state.FiveHourProjected == nil {
		t.Fatal("FiveHourProjected should be set for future reset with elapsed time")
	}
	// 50% consumed, 1h left in 5h window → elapsed=4h → projected = 50 * 5/4 = 62.5
	if *state.FiveHourProjected < 60 || *state.FiveHourProjected > 65 {
		t.Errorf("FiveHourProjected = %f, want ~62.5", *state.FiveHourProjected)
	}
}

func TestFetch_ComputesSaturation(t *testing.T) {
	// 80% consumed with 4h remaining → projected = 80*5/1 = 400% (>100)
	// → saturation = now + (100-80)/80 * 1h = now + 15min
	resetsAt := time.Now().UTC().Add(4 * time.Hour).Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{
			"five_hour": {"utilization": 80.0, "resets_at": "` + resetsAt + `"},
			"seven_day": {"utilization": 10.0}
		}`))
	}))
	defer srv.Close()

	origURL := usageURL
	defer func() { usageURL = origURL }()
	usageURL = srv.URL

	qc := newTestQuotaClient("tok", time.Now().UnixMilli()+300_000, srv.Client())
	if !qc.Fetch() {
		t.Fatal("Fetch() returned false")
	}

	state := qc.State()
	if state.FiveHourProjected == nil {
		t.Fatal("FiveHourProjected should be set")
	}
	if *state.FiveHourProjected < 350 || *state.FiveHourProjected > 450 {
		t.Errorf("FiveHourProjected = %f, want ~400", *state.FiveHourProjected)
	}
	if state.FiveHourSaturation == nil {
		t.Fatal("FiveHourSaturation should be set when projected > 100%%")
	}
	// Saturation should be ~15min from now.
	untilSat := time.Until(*state.FiveHourSaturation)
	if untilSat < 14*time.Minute || untilSat > 16*time.Minute {
		t.Errorf("FiveHourSaturation in %v, want ~15m", untilSat)
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

func TestComputeProjection_Normal(t *testing.T) {
	// 33% consumed, 23 min remaining in 5h window
	// elapsed = 5h - 23m = 4h37m = 277 min
	// projected = 33 * 300/277 ≈ 35.74
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	resetsAt := now.Add(23 * time.Minute)
	proj := computeProjection(33, resetsAt, now, fiveHourWindow)
	if proj == nil {
		t.Fatal("expected non-nil projection")
	}
	// 33 * 300 / 277 ≈ 35.74
	if *proj < 35.5 || *proj > 36.0 {
		t.Errorf("projected = %f, want ~35.74", *proj)
	}
}

func TestComputeProjection_HighUsageUncapped(t *testing.T) {
	// 80% consumed with 4h remaining in 5h window → projected way over 100
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	resetsAt := now.Add(4 * time.Hour)
	// elapsed = 1h, projected = 80 * 5/1 = 400 → no longer capped
	proj := computeProjection(80, resetsAt, now, fiveHourWindow)
	if proj == nil {
		t.Fatal("expected non-nil projection")
	}
	if *proj != 400 {
		t.Errorf("projected = %f, want 400 (uncapped)", *proj)
	}
}

func TestComputeProjection_MostWindowElapsed(t *testing.T) {
	// 50% consumed, 30min remaining in 5h window
	// elapsed = 4h30m = 270min, projected = 50 * 300/270 ≈ 55.56
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	resetsAt := now.Add(30 * time.Minute)
	proj := computeProjection(50, resetsAt, now, fiveHourWindow)
	if proj == nil {
		t.Fatal("expected non-nil projection")
	}
	if *proj < 55.0 || *proj > 56.0 {
		t.Errorf("projected = %f, want ~55.56", *proj)
	}
}

func TestComputeProjection_ZeroCurrent(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	resetsAt := now.Add(2 * time.Hour)
	proj := computeProjection(0, resetsAt, now, fiveHourWindow)
	if proj != nil {
		t.Errorf("expected nil for zero current, got %f", *proj)
	}
}

func TestComputeProjection_PastReset(t *testing.T) {
	now := time.Date(2026, 2, 10, 15, 0, 0, 0, time.UTC)
	resetsAt := time.Date(2026, 2, 10, 14, 0, 0, 0, time.UTC) // already past
	proj := computeProjection(40, resetsAt, now, fiveHourWindow)
	if proj != nil {
		t.Errorf("expected nil for past reset time, got %f", *proj)
	}
}

func TestComputeProjection_ResetEqualsNow(t *testing.T) {
	now := time.Date(2026, 2, 10, 14, 0, 0, 0, time.UTC)
	resetsAt := now
	proj := computeProjection(40, resetsAt, now, fiveHourWindow)
	if proj != nil {
		t.Errorf("expected nil when reset equals now, got %f", *proj)
	}
}

func TestComputeProjection_WindowNotStarted(t *testing.T) {
	// resetsAt is 5h+ away, meaning window hasn't started yet (timeElapsed <= 0)
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	resetsAt := now.Add(6 * time.Hour) // more than window duration away
	proj := computeProjection(10, resetsAt, now, fiveHourWindow)
	if proj != nil {
		t.Errorf("expected nil when window hasn't started, got %f", *proj)
	}
}

func TestComputeSaturationTime_Normal(t *testing.T) {
	// 80% consumed, 4h remaining in 5h window
	// elapsed = 1h, rate = 80/1h, timeToSaturation = 20/80 * 1h = 15min
	// saturation = now + 15min, reset = now + 4h → before reset ✓
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	resetsAt := now.Add(4 * time.Hour)
	sat := computeSaturationTime(80, resetsAt, now, fiveHourWindow)
	if sat == nil {
		t.Fatal("expected non-nil saturation time")
	}
	// timeToSaturation = (100-80)/80 * 1h = 0.25h = 15min
	expected := now.Add(15 * time.Minute)
	diff := sat.Sub(expected)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("saturation = %v, want ~%v (diff=%v)", sat, expected, diff)
	}
}

func TestComputeSaturationTime_NoSaturation(t *testing.T) {
	// 10% consumed, 2h remaining → projected = 10 * 5/3 ≈ 16.67% → below 100%
	// saturation time would be past reset → nil
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	resetsAt := now.Add(2 * time.Hour)
	sat := computeSaturationTime(10, resetsAt, now, fiveHourWindow)
	if sat != nil {
		t.Errorf("expected nil for projection < 100%%, got %v", sat)
	}
}

func TestComputeSaturationTime_AlreadySaturated(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	resetsAt := now.Add(1 * time.Hour)
	sat := computeSaturationTime(100, resetsAt, now, fiveHourWindow)
	if sat != nil {
		t.Errorf("expected nil for current >= 100, got %v", sat)
	}
}

func TestComputeSaturationTime_PastReset(t *testing.T) {
	now := time.Date(2026, 2, 10, 15, 0, 0, 0, time.UTC)
	resetsAt := time.Date(2026, 2, 10, 14, 0, 0, 0, time.UTC)
	sat := computeSaturationTime(80, resetsAt, now, fiveHourWindow)
	if sat != nil {
		t.Errorf("expected nil for past reset, got %v", sat)
	}
}

func TestComputeSaturationTime_WindowNotStarted(t *testing.T) {
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	resetsAt := now.Add(6 * time.Hour)
	sat := computeSaturationTime(80, resetsAt, now, fiveHourWindow)
	if sat != nil {
		t.Errorf("expected nil when window hasn't started, got %v", sat)
	}
}

func TestComputeProjection_SevenDay_Normal(t *testing.T) {
	// 20% consumed, 3 days remaining in 7d window
	// elapsed = 7d - 3d = 4d, projected = 20 * 7/4 = 35
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	resetsAt := now.Add(3 * 24 * time.Hour)
	proj := computeProjection(20, resetsAt, now, sevenDayWindow)
	if proj == nil {
		t.Fatal("expected non-nil projection")
	}
	if *proj != 35 {
		t.Errorf("projected = %f, want 35", *proj)
	}
}

func TestComputeProjection_SevenDay_HighUsage(t *testing.T) {
	// 50% consumed, 6 days remaining → elapsed = 1d → projected = 50*7/1 = 350
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	resetsAt := now.Add(6 * 24 * time.Hour)
	proj := computeProjection(50, resetsAt, now, sevenDayWindow)
	if proj == nil {
		t.Fatal("expected non-nil projection")
	}
	if *proj != 350 {
		t.Errorf("projected = %f, want 350", *proj)
	}
}

func TestComputeSaturationTime_SevenDay_Normal(t *testing.T) {
	// 80% consumed, 6 days remaining in 7d window
	// elapsed = 1d, rate = 80/1d, timeToSaturation = 20/80 * 1d = 6h
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	resetsAt := now.Add(6 * 24 * time.Hour)
	sat := computeSaturationTime(80, resetsAt, now, sevenDayWindow)
	if sat == nil {
		t.Fatal("expected non-nil saturation time")
	}
	expected := now.Add(6 * time.Hour)
	diff := sat.Sub(expected)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("saturation = %v, want ~%v (diff=%v)", sat, expected, diff)
	}
}

func TestComputeSaturationTime_SevenDay_NoSaturation(t *testing.T) {
	// 5% consumed, 1 day remaining → elapsed = 6d → projected = 5*7/6 ≈ 5.83% → no saturation
	now := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	resetsAt := now.Add(1 * 24 * time.Hour)
	sat := computeSaturationTime(5, resetsAt, now, sevenDayWindow)
	if sat != nil {
		t.Errorf("expected nil for projection < 100%%, got %v", sat)
	}
}

func TestFetch_ComputesSevenDayProjection(t *testing.T) {
	// 7d reset 3 days from now → elapsed = 4d → projected = 20 * 7/4 = 35
	resetsAt := time.Now().UTC().Add(3 * 24 * time.Hour).Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{
			"five_hour": {"utilization": 10.0},
			"seven_day": {"utilization": 20.0, "resets_at": "` + resetsAt + `"}
		}`))
	}))
	defer srv.Close()

	origURL := usageURL
	defer func() { usageURL = origURL }()
	usageURL = srv.URL

	qc := newTestQuotaClient("tok", time.Now().UnixMilli()+300_000, srv.Client())
	if !qc.Fetch() {
		t.Fatal("Fetch() returned false")
	}

	state := qc.State()
	if state.SevenDayProjected == nil {
		t.Fatal("SevenDayProjected should be set")
	}
	if *state.SevenDayProjected < 34 || *state.SevenDayProjected > 36 {
		t.Errorf("SevenDayProjected = %f, want ~35", *state.SevenDayProjected)
	}
}

func TestFetch_ComputesSevenDaySaturation(t *testing.T) {
	// 80% consumed with 6 days remaining → projected = 80*7/1 = 560% (>100)
	// → saturation = now + (100-80)/80 * 1d = now + 6h
	resetsAt := time.Now().UTC().Add(6 * 24 * time.Hour).Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{
			"five_hour": {"utilization": 10.0},
			"seven_day": {"utilization": 80.0, "resets_at": "` + resetsAt + `"}
		}`))
	}))
	defer srv.Close()

	origURL := usageURL
	defer func() { usageURL = origURL }()
	usageURL = srv.URL

	qc := newTestQuotaClient("tok", time.Now().UnixMilli()+300_000, srv.Client())
	if !qc.Fetch() {
		t.Fatal("Fetch() returned false")
	}

	state := qc.State()
	if state.SevenDayProjected == nil {
		t.Fatal("SevenDayProjected should be set")
	}
	if *state.SevenDayProjected < 500 || *state.SevenDayProjected > 600 {
		t.Errorf("SevenDayProjected = %f, want ~560", *state.SevenDayProjected)
	}
	if state.SevenDaySaturation == nil {
		t.Fatal("SevenDaySaturation should be set when projected > 100%%")
	}
	untilSat := time.Until(*state.SevenDaySaturation)
	if untilSat < 5*time.Hour || untilSat > 7*time.Hour {
		t.Errorf("SevenDaySaturation in %v, want ~6h", untilSat)
	}
}
