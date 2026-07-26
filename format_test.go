package main

import (
	"strings"
	"testing"
	"time"
)

func TestFormatTimeRemaining_Nil(t *testing.T) {
	if got := formatTimeRemaining(nil); got != "unknown" {
		t.Errorf("formatTimeRemaining(nil) = %q, want %q", got, "unknown")
	}
}

func TestFormatTimeRemaining_Past(t *testing.T) {
	past := time.Now().Add(-10 * time.Minute)
	if got := formatTimeRemaining(&past); got != "now" {
		t.Errorf("formatTimeRemaining(past) = %q, want %q", got, "now")
	}
}

func TestFormatTimeRemaining_MinutesOnly(t *testing.T) {
	future := time.Now().Add(45*time.Minute + 30*time.Second)
	got := formatTimeRemaining(&future)
	if got != "45m" {
		t.Errorf("formatTimeRemaining(+45m30s) = %q, want %q", got, "45m")
	}
}

func TestFormatTimeRemaining_HoursAndMinutes(t *testing.T) {
	future := time.Now().Add(2*time.Hour + 15*time.Minute + 10*time.Second)
	got := formatTimeRemaining(&future)
	if got != "2h 15m" {
		t.Errorf("formatTimeRemaining(+2h15m) = %q, want %q", got, "2h 15m")
	}
}

func TestFormatResetDate_Nil(t *testing.T) {
	if got := formatResetDate(nil); got != "" {
		t.Errorf("formatResetDate(nil) = %q, want %q", got, "")
	}
}

func TestFormatResetDate_Format(t *testing.T) {
	// Use a fixed time in UTC, then check local formatting.
	ts := time.Date(2026, 2, 6, 14, 30, 0, 0, time.UTC)
	got := formatResetDate(&ts)
	expect := ts.Local().Format("Mon 15:04")
	if got != expect {
		t.Errorf("formatResetDate = %q, want %q", got, expect)
	}
}

func TestFormatClockLine(t *testing.T) {
	// time.Local so .Local() is a no-op and the expectation holds on any TZ.
	ts := time.Date(2026, 1, 2, 14, 30, 5, 0, time.Local)

	tests := []struct {
		name  string
		label string
		t     *time.Time
		want  string
	}{
		{"updated time", "Updated", &ts, "Updated: 14:30:05"},
		{"updated nil", "Updated", nil, "Updated: --"},
		{"next update time", "Next update", &ts, "Next update: 14:30:05"},
		{"next update nil", "Next update", nil, "Next update: --"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatClockLine(tc.label, tc.t); got != tc.want {
				t.Errorf("formatClockLine(%q) = %q, want %q", tc.label, got, tc.want)
			}
		})
	}
}

func TestFormatSaturationLine_Nil(t *testing.T) {
	got := formatSaturationLine(nil)
	if got != "" {
		t.Errorf("formatSaturationLine(nil) = %q, want %q", got, "")
	}
}

func TestFormatSaturationLine_Future(t *testing.T) {
	sat := time.Now().Add(1*time.Hour + 15*time.Minute + 30*time.Second)
	got := formatSaturationLine(&sat)
	date := formatResetDate(&sat)
	expect := "  - saturates in 1h 15m, " + date
	if got != expect {
		t.Errorf("formatSaturationLine(+1h15m) = %q, want %q", got, expect)
	}
}

func TestFormatQuotaLine_NilUtilization(t *testing.T) {
	got := formatQuotaLine("5h", nil, nil)
	if got != "5h: --" {
		t.Errorf("formatQuotaLine(nil) = %q, want %q", got, "5h: --")
	}
}

func TestFormatQuotaLine_WithUtilization_NoResets(t *testing.T) {
	v := 42.0
	got := formatQuotaLine("7d", &v, nil)
	// No reset date => no parens, but formatTimeRemaining returns "unknown".
	// Since formatResetDate(nil) == "", it uses the short format.
	if got != "7d: 42%" {
		t.Errorf("formatQuotaLine(42, nil) = %q, want %q", got, "7d: 42%")
	}
}

func TestFormatQuotaLine_WithUtilization_WithResets(t *testing.T) {
	v := 73.0
	// Add extra seconds to avoid rounding down across the minute boundary.
	resets := time.Now().Add(2*time.Hour + 30*time.Minute + 30*time.Second)
	got := formatQuotaLine("5h", &v, &resets)
	date := formatResetDate(&resets)
	expect := "5h: 73% (resets in 2h 30m, " + date + ")"
	if got != expect {
		t.Errorf("formatQuotaLine(73, +2h30m) = %q, want %q", got, expect)
	}
}

func TestFormatProjectionLine_Nil(t *testing.T) {
	got := formatProjectionLine(nil)
	if got != "" {
		t.Errorf("formatProjectionLine(nil) = %q, want %q", got, "")
	}
}

func TestFormatProjectionLine_Value(t *testing.T) {
	proj := 35.7
	got := formatProjectionLine(&proj)
	if got != "  - projected ~36% at reset" {
		t.Errorf("formatProjectionLine(35.7) = %q, want %q", got, "  - projected ~36% at reset")
	}
}

func TestFormatDryRunSummary(t *testing.T) {
	v5 := 12.0
	v7 := 34.0
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	reset := now.Add(2 * time.Hour)
	state := QuotaState{
		Provider:       ProviderCodex,
		FiveHour:       &v5,
		FiveHourResets: &reset,
		SevenDay:       &v7,
		LastUpdate:     &now,
	}

	got := formatDryRunSummary(ProviderCodex, "/tmp/auth.json", state)
	if !strings.Contains(got, "Provider: codex") {
		t.Fatalf("missing provider line: %q", got)
	}
	if !strings.Contains(got, "Credentials: /tmp/auth.json") {
		t.Fatalf("missing credentials line: %q", got)
	}
	if !strings.Contains(got, "5h: 12%") {
		t.Fatalf("missing 5h line: %q", got)
	}
	if !strings.Contains(got, "7d: 34%") {
		t.Fatalf("missing 7d line: %q", got)
	}
	if !strings.Contains(got, "Updated: "+now.Local().Format(time.RFC3339)) {
		t.Fatalf("missing updated line: %q", got)
	}
}

func TestFormatDryRunSummary_Error(t *testing.T) {
	state := QuotaState{
		Error: "token expired",
	}
	got := formatDryRunSummary(ProviderClaude, "/tmp/creds.json", state)
	if !strings.Contains(got, "Error: token expired") {
		t.Fatalf("missing error line: %q", got)
	}
	if strings.Contains(got, "5h:") {
		t.Fatalf("error summary should not contain quota lines: %q", got)
	}
}

func TestFormatDryRunSummary_WithProjectionAndSonnet(t *testing.T) {
	v5 := 80.0
	v7 := 30.0
	proj := 120.0
	sonnet := 15.0
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	reset := now.Add(1 * time.Hour)
	sat := now.Add(30 * time.Minute)
	state := QuotaState{
		Provider:           ProviderClaude,
		FiveHour:           &v5,
		FiveHourResets:     &reset,
		FiveHourProjected:  &proj,
		FiveHourSaturation: &sat,
		SevenDay:           &v7,
		SevenDaySonnet:     &sonnet,
		LastUpdate:         &now,
	}

	got := formatDryRunSummary(ProviderClaude, "/tmp/creds.json", state)
	if !strings.Contains(got, "5h: 80%") {
		t.Fatalf("missing 5h line: %q", got)
	}
	if !strings.Contains(got, "projected ~120% at reset") {
		t.Fatalf("missing projection line: %q", got)
	}
	if !strings.Contains(got, "saturates") {
		t.Fatalf("missing saturation line: %q", got)
	}
	if !strings.Contains(got, "Sonnet 7d: 15%") {
		t.Fatalf("missing sonnet line: %q", got)
	}
}
