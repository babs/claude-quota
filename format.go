package main

import (
	"fmt"
	"strings"
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

// formatClockLine returns "<label>: HH:MM:SS" in local time, or "<label>: --"
// when the time is unknown. Absolute (not relative) so the tray only needs to
// refresh it on a poll, not tick every second.
func formatClockLine(label string, t *time.Time) string {
	if t == nil {
		return label + ": --"
	}
	return label + ": " + t.Local().Format("15:04:05")
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
	return fmt.Sprintf("  - projected ~%.0f%% at reset", *projected)
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

func formatDryRunSummary(provider Provider, credentialsPath string, state QuotaState) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Provider: %s", provider))
	lines = append(lines, fmt.Sprintf("Credentials: %s", credentialsPath))

	if state.Error != "" {
		lines = append(lines, fmt.Sprintf("Error: %s", state.Error))
		return strings.Join(lines, "\n")
	}

	lines = append(lines, formatQuotaLine("5h", state.FiveHour, state.FiveHourResets))
	if line := formatProjectionLine(state.FiveHourProjected); line != "" {
		lines = append(lines, strings.TrimSpace(line))
	}
	if line := formatSaturationLine(state.FiveHourSaturation); line != "" {
		lines = append(lines, strings.TrimSpace(line))
	}

	lines = append(lines, formatQuotaLine("7d", state.SevenDay, state.SevenDayResets))
	if line := formatProjectionLine(state.SevenDayProjected); line != "" {
		lines = append(lines, strings.TrimSpace(line))
	}
	if line := formatSaturationLine(state.SevenDaySaturation); line != "" {
		lines = append(lines, strings.TrimSpace(line))
	}

	if state.SevenDaySonnet != nil {
		lines = append(lines, formatQuotaLine(extraQuotaLabel(state), state.SevenDaySonnet, state.SevenDaySonnetResets))
	}
	if state.LastUpdate != nil {
		lines = append(lines, fmt.Sprintf("Updated: %s", state.LastUpdate.Local().Format(time.RFC3339)))
	}

	return strings.Join(lines, "\n")
}
