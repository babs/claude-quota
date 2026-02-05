package main

import (
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

func TestFormatUpdatedAgo_Nil(t *testing.T) {
	if got := formatUpdatedAgo(nil); got != "Updated: --" {
		t.Errorf("formatUpdatedAgo(nil) = %q, want %q", got, "Updated: --")
	}
}

func TestFormatUpdatedAgo_Seconds(t *testing.T) {
	ts := time.Now().Add(-30 * time.Second)
	got := formatUpdatedAgo(&ts)
	// Should be ~30s, allow some slack.
	if got != "Updated: 30s ago" && got != "Updated: 31s ago" {
		t.Errorf("formatUpdatedAgo(-30s) = %q", got)
	}
}

func TestFormatUpdatedAgo_Minutes(t *testing.T) {
	ts := time.Now().Add(-3*time.Minute - 12*time.Second)
	got := formatUpdatedAgo(&ts)
	if got != "Updated: 3m 12s ago" && got != "Updated: 3m 13s ago" {
		t.Errorf("formatUpdatedAgo(-3m12s) = %q", got)
	}
}

func TestFormatUpdatedAgo_Hours(t *testing.T) {
	ts := time.Now().Add(-1*time.Hour - 5*time.Minute)
	got := formatUpdatedAgo(&ts)
	if got != "Updated: 1h 5m ago" {
		t.Errorf("formatUpdatedAgo(-1h5m) = %q", got)
	}
}

func TestFormatUpdatedAgo_Future(t *testing.T) {
	ts := time.Now().Add(10 * time.Second)
	got := formatUpdatedAgo(&ts)
	// Negative duration is clamped to 0.
	if got != "Updated: 0s ago" {
		t.Errorf("formatUpdatedAgo(future) = %q, want %q", got, "Updated: 0s ago")
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
