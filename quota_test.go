package main

import (
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
