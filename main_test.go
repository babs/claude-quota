package main

import (
	"testing"
)

func TestApplyOverrides_Defaults(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, 0, 0)
	if cfg.PollIntervalSeconds != 300 {
		t.Errorf("PollIntervalSeconds = %d, want 300", cfg.PollIntervalSeconds)
	}
	if cfg.FontSize != 18 {
		t.Errorf("FontSize = %f, want 18", cfg.FontSize)
	}
}

func TestApplyOverrides_Flags(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, 60, 24)
	if cfg.PollIntervalSeconds != 60 {
		t.Errorf("PollIntervalSeconds = %d, want 60", cfg.PollIntervalSeconds)
	}
	if cfg.FontSize != 24 {
		t.Errorf("FontSize = %f, want 24", cfg.FontSize)
	}
}

func TestApplyOverrides_EnvVars(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_POLL_INTERVAL", "120")
	t.Setenv("CLAUDE_QUOTA_FONT_SIZE", "32")

	cfg := defaultConfig()
	applyOverrides(&cfg, 0, 0)
	if cfg.PollIntervalSeconds != 120 {
		t.Errorf("PollIntervalSeconds = %d, want 120", cfg.PollIntervalSeconds)
	}
	if cfg.FontSize != 32 {
		t.Errorf("FontSize = %f, want 32", cfg.FontSize)
	}
}

func TestApplyOverrides_FlagOverridesEnv(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_POLL_INTERVAL", "120")
	t.Setenv("CLAUDE_QUOTA_FONT_SIZE", "32")

	cfg := defaultConfig()
	applyOverrides(&cfg, 60, 24)
	if cfg.PollIntervalSeconds != 60 {
		t.Errorf("PollIntervalSeconds = %d, want 60 (flag should override env)", cfg.PollIntervalSeconds)
	}
	if cfg.FontSize != 24 {
		t.Errorf("FontSize = %f, want 24 (flag should override env)", cfg.FontSize)
	}
}

func TestApplyOverrides_InvalidEnvIgnored(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_POLL_INTERVAL", "abc")
	t.Setenv("CLAUDE_QUOTA_FONT_SIZE", "-5")

	cfg := defaultConfig()
	applyOverrides(&cfg, 0, 0)
	if cfg.PollIntervalSeconds != 300 {
		t.Errorf("PollIntervalSeconds = %d, want 300 (invalid env should be ignored)", cfg.PollIntervalSeconds)
	}
	if cfg.FontSize != 18 {
		t.Errorf("FontSize = %f, want 18 (invalid env should be ignored)", cfg.FontSize)
	}
}
