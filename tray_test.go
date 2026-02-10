package main

import (
	"strings"
	"testing"
	"time"
)

func TestBuildTooltip_Empty(t *testing.T) {
	state := QuotaState{}
	got := buildTooltip(state)
	if got != "Claude Quota" {
		t.Errorf("buildTooltip(empty) = %q, want %q", got, "Claude Quota")
	}
}

func TestBuildTooltip_Error(t *testing.T) {
	state := QuotaState{Error: "something broke"}
	got := buildTooltip(state)
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
		FiveHour: &v5,
		SevenDay: &v7,
	}
	got := buildTooltip(state)
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
		FiveHour:       &v5,
		SevenDay:       &v7,
		SevenDaySonnet: &vs,
	}
	got := buildTooltip(state)
	if !strings.Contains(got, "Sonnet 7d: 5%") {
		t.Errorf("buildTooltip missing Sonnet 7d line: %q", got)
	}
}

func TestBuildTooltip_WithLastUpdate(t *testing.T) {
	now := time.Now().UTC()
	state := QuotaState{LastUpdate: &now}
	got := buildTooltip(state)
	if !strings.Contains(got, "Updated:") {
		t.Errorf("buildTooltip missing Updated line: %q", got)
	}
}

func TestBuildTooltip_WithProjection(t *testing.T) {
	v5 := 33.0
	proj := 36.0
	resets := time.Now().Add(23 * time.Minute)
	state := QuotaState{
		FiveHour:          &v5,
		FiveHourResets:    &resets,
		FiveHourProjected: &proj,
	}
	got := buildTooltip(state)
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
		FiveHour:           &v5,
		FiveHourResets:     &resets,
		FiveHourProjected:  &proj,
		FiveHourSaturation: &sat,
	}
	got := buildTooltip(state)
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
		FiveHour: &v,
		Error:    "token expired",
	}
	got := buildTooltip(state)
	if !strings.Contains(got, "Error: token expired") {
		t.Errorf("buildTooltip missing error: %q", got)
	}
	// When there's an error, quota lines should be hidden.
	if strings.Contains(got, "5h:") {
		t.Errorf("buildTooltip should hide quota on error: %q", got)
	}
}
