package main

import (
	"strings"
	"testing"
	"time"
)

func TestBuildTooltip_Empty(t *testing.T) {
	state := QuotaState{}
	got := buildTooltip(state, ProviderClaude)
	if got != "Agent Quota (Claude)" {
		t.Errorf("buildTooltip(empty) = %q, want %q", got, "Agent Quota (Claude)")
	}
}

func TestBuildTooltip_Error(t *testing.T) {
	state := QuotaState{Error: "something broke"}
	got := buildTooltip(state, ProviderClaude)
	if !strings.Contains(got, "Error: something broke") {
		t.Errorf("buildTooltip(error) = %q, missing error line", got)
	}
	// Error state should not include quota lines.
	if strings.Contains(got, "5h:") {
		t.Errorf("buildTooltip(error) should not contain quota lines")
	}
}

func TestBuildTooltip_WithQuota(t *testing.T) {
	v5 := 42.0
	v7 := 10.0
	state := QuotaState{
		Provider: ProviderClaude,
		FiveHour: &v5,
		SevenDay: &v7,
	}
	got := buildTooltip(state, ProviderClaude)
	if !strings.Contains(got, "5h: 42%") {
		t.Errorf("buildTooltip missing 5h line: %q", got)
	}
	if !strings.Contains(got, "7d: 10%") {
		t.Errorf("buildTooltip missing 7d line: %q", got)
	}
}

func TestBuildTooltip_WithAllQuotas(t *testing.T) {
	v5 := 42.0
	v7 := 10.0
	vs := 5.0
	state := QuotaState{
		Provider:       ProviderClaude,
		FiveHour:       &v5,
		SevenDay:       &v7,
		SevenDaySonnet: &vs,
	}
	got := buildTooltip(state, ProviderClaude)
	if !strings.Contains(got, "Sonnet 7d: 5%") {
		t.Errorf("buildTooltip missing Sonnet 7d line: %q", got)
	}
}

func TestBuildTooltip_WithLastUpdate(t *testing.T) {
	now := time.Now().UTC()
	state := QuotaState{LastUpdate: &now}
	got := buildTooltip(state, ProviderClaude)
	if !strings.Contains(got, "Updated:") {
		t.Errorf("buildTooltip missing Updated line: %q", got)
	}
}

func TestBuildTooltip_WithProjection(t *testing.T) {
	v5 := 33.0
	proj := 36.0
	resets := time.Now().Add(23 * time.Minute)
	state := QuotaState{
		Provider:          ProviderClaude,
		FiveHour:          &v5,
		FiveHourResets:    &resets,
		FiveHourProjected: &proj,
	}
	got := buildTooltip(state, ProviderClaude)
	if !strings.Contains(got, "5h: 33%") {
		t.Errorf("buildTooltip missing 5h line: %q", got)
	}
	if !strings.Contains(got, "\n  - projected ~36% at reset") {
		t.Errorf("buildTooltip missing projection on separate line: %q", got)
	}
}

func TestBuildTooltip_WithSaturation(t *testing.T) {
	v5 := 80.0
	proj := 400.0
	resets := time.Now().Add(4 * time.Hour)
	sat := time.Now().Add(15 * time.Minute)
	state := QuotaState{
		Provider:           ProviderClaude,
		FiveHour:           &v5,
		FiveHourResets:     &resets,
		FiveHourProjected:  &proj,
		FiveHourSaturation: &sat,
	}
	got := buildTooltip(state, ProviderClaude)
	if !strings.Contains(got, "projected ~400% at reset") {
		t.Errorf("buildTooltip missing uncapped projection: %q", got)
	}
	if !strings.Contains(got, "saturates in") {
		t.Errorf("buildTooltip missing saturation line: %q", got)
	}
}

func TestBuildTooltip_ErrorHidesQuota(t *testing.T) {
	v := 42.0
	state := QuotaState{
		Provider: ProviderClaude,
		FiveHour: &v,
		Error:    "token expired",
	}
	got := buildTooltip(state, ProviderClaude)
	if !strings.Contains(got, "Error: token expired") {
		t.Errorf("buildTooltip missing error: %q", got)
	}
	// When there's an error, quota lines should be hidden.
	if strings.Contains(got, "5h:") {
		t.Errorf("buildTooltip should hide quota on error: %q", got)
	}
}

func TestExtraQuotaLabel_CustomLabel(t *testing.T) {
	state := QuotaState{Provider: ProviderCodex, SevenDaySonnetLabel: "Code review"}
	if got := extraQuotaLabel(state); got != "Code review" {
		t.Fatalf("extraQuotaLabel = %q, want 'Code review'", got)
	}
}

func TestExtraQuotaLabel_CodexDefault(t *testing.T) {
	state := QuotaState{Provider: ProviderCodex}
	if got := extraQuotaLabel(state); got != "Additional" {
		t.Fatalf("extraQuotaLabel(codex, no label) = %q, want 'Additional'", got)
	}
}

func TestExtraQuotaLabel_ClaudeDefault(t *testing.T) {
	state := QuotaState{Provider: ProviderClaude}
	if got := extraQuotaLabel(state); got != "Sonnet 7d" {
		t.Fatalf("extraQuotaLabel(claude) = %q, want 'Sonnet 7d'", got)
	}
}

func TestBuildTooltip_CodexUsesProviderTitle(t *testing.T) {
	v5 := 12.0
	state := QuotaState{
		Provider: ProviderCodex,
		FiveHour: &v5,
		SevenDay: &v5,
	}
	got := buildTooltip(state, ProviderCodex)
	if !strings.HasPrefix(got, "Agent Quota (Codex)") {
		t.Fatalf("buildTooltip() = %q, want Codex title", got)
	}
}
