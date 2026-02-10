package main

import (
	"fmt"
	"time"
)

// formatTimeRemaining returns a human-readable duration until resetTime.
// Returns "unknown" if resetTime is nil, "now" if already past.
func formatTimeRemaining(resetTime *time.Time) string {
	if resetTime == nil {
		return "unknown"
	}

	delta := time.Until(*resetTime)
	if delta < 0 {
		return "now"
	}

	totalSec := int(delta.Seconds())
	hours := totalSec / 3600
	minutes := (totalSec % 3600) / 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// formatResetDate formats a reset time as local "Day HH:MM".
func formatResetDate(resetTime *time.Time) string {
	if resetTime == nil {
		return ""
	}
	local := resetTime.Local()
	return local.Format("Mon 15:04")
}

// formatUpdatedAgo returns "Updated: Xs ago" / "Xm Ys ago" / "Xh Ym ago" for the given time.
func formatUpdatedAgo(t *time.Time) string {
	if t == nil {
		return "Updated: --"
	}
	ago := time.Since(*t)
	if ago < 0 {
		ago = 0
	}
	totalSec := int(ago.Seconds())
	if totalSec < 60 {
		return fmt.Sprintf("Updated: %ds ago", totalSec)
	}
	minutes := totalSec / 60
	seconds := totalSec % 60
	if minutes < 60 {
		return fmt.Sprintf("Updated: %dm %ds ago", minutes, seconds)
	}
	hours := minutes / 60
	minutes = minutes % 60
	return fmt.Sprintf("Updated: %dh %dm ago", hours, minutes)
}

// formatSaturationLine returns a formatted saturation line, or "" if nil.
func formatSaturationLine(saturation *time.Time) string {
	if saturation == nil {
		return ""
	}
	remaining := formatTimeRemaining(saturation)
	date := formatResetDate(saturation)
	return fmt.Sprintf("  - saturates in %s, %s", remaining, date)
}

// formatProjectionLine returns a formatted projection line, or "" if nil.
func formatProjectionLine(projected *float64) string {
	if projected == nil {
		return ""
	}
	return fmt.Sprintf("  - ~%.0f%% at reset", *projected)
}

// formatQuotaLine formats a single quota line with remaining time and date.
func formatQuotaLine(label string, utilization *float64, resets *time.Time) string {
	if utilization == nil {
		return fmt.Sprintf("%s: --", label)
	}
	remaining := formatTimeRemaining(resets)
	date := formatResetDate(resets)
	if date != "" {
		return fmt.Sprintf("%s: %.0f%% (resets in %s, %s)", label, *utilization, remaining, date)
	}
	return fmt.Sprintf("%s: %.0f%%", label, *utilization)
}
