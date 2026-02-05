package main

import (
	"testing"
)

func TestApplyOverrides_Defaults(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, 0, 0, "", -1, 0)
	if cfg.PollIntervalSeconds != 300 {
		t.Errorf("PollIntervalSeconds = %d, want 300", cfg.PollIntervalSeconds)
	}
	if cfg.FontSize != 34 {
		t.Errorf("FontSize = %f, want 34", cfg.FontSize)
	}
	if cfg.FontName != "bold" {
		t.Errorf("FontName = %q, want %q", cfg.FontName, "bold")
	}
	if cfg.HaloSize != 2 {
		t.Errorf("HaloSize = %f, want 2", cfg.HaloSize)
	}
	if cfg.IconSize != 64 {
		t.Errorf("IconSize = %d, want 64", cfg.IconSize)
	}
}

func TestApplyOverrides_Flags(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, 60, 24, "mono", 3.0, 128)
	if cfg.PollIntervalSeconds != 60 {
		t.Errorf("PollIntervalSeconds = %d, want 60", cfg.PollIntervalSeconds)
	}
	if cfg.FontSize != 24 {
		t.Errorf("FontSize = %f, want 24", cfg.FontSize)
	}
	if cfg.FontName != "mono" {
		t.Errorf("FontName = %q, want %q", cfg.FontName, "mono")
	}
	if cfg.HaloSize != 3.0 {
		t.Errorf("HaloSize = %f, want 3.0", cfg.HaloSize)
	}
	if cfg.IconSize != 128 {
		t.Errorf("IconSize = %d, want 128", cfg.IconSize)
	}
}

func TestApplyOverrides_EnvVars(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_POLL_INTERVAL", "120")
	t.Setenv("CLAUDE_QUOTA_FONT_SIZE", "32")
	t.Setenv("CLAUDE_QUOTA_FONT_NAME", "bitmap")
	t.Setenv("CLAUDE_QUOTA_HALO_SIZE", "2.5")
	t.Setenv("CLAUDE_QUOTA_ICON_SIZE", "128")

	cfg := defaultConfig()
	applyOverrides(&cfg, 0, 0, "", -1, 0)
	if cfg.PollIntervalSeconds != 120 {
		t.Errorf("PollIntervalSeconds = %d, want 120", cfg.PollIntervalSeconds)
	}
	if cfg.FontSize != 32 {
		t.Errorf("FontSize = %f, want 32", cfg.FontSize)
	}
	if cfg.FontName != "bitmap" {
		t.Errorf("FontName = %q, want %q", cfg.FontName, "bitmap")
	}
	if cfg.HaloSize != 2.5 {
		t.Errorf("HaloSize = %f, want 2.5", cfg.HaloSize)
	}
	if cfg.IconSize != 128 {
		t.Errorf("IconSize = %d, want 128", cfg.IconSize)
	}
}

func TestApplyOverrides_FlagOverridesEnv(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_POLL_INTERVAL", "120")
	t.Setenv("CLAUDE_QUOTA_FONT_SIZE", "32")
	t.Setenv("CLAUDE_QUOTA_FONT_NAME", "bitmap")
	t.Setenv("CLAUDE_QUOTA_HALO_SIZE", "2.5")
	t.Setenv("CLAUDE_QUOTA_ICON_SIZE", "128")

	cfg := defaultConfig()
	applyOverrides(&cfg, 60, 24, "mono", 0.5, 256)
	if cfg.PollIntervalSeconds != 60 {
		t.Errorf("PollIntervalSeconds = %d, want 60 (flag should override env)", cfg.PollIntervalSeconds)
	}
	if cfg.FontSize != 24 {
		t.Errorf("FontSize = %f, want 24 (flag should override env)", cfg.FontSize)
	}
	if cfg.FontName != "mono" {
		t.Errorf("FontName = %q, want %q (flag should override env)", cfg.FontName, "mono")
	}
	if cfg.HaloSize != 0.5 {
		t.Errorf("HaloSize = %f, want 0.5 (flag should override env)", cfg.HaloSize)
	}
	if cfg.IconSize != 256 {
		t.Errorf("IconSize = %d, want 256 (flag should override env)", cfg.IconSize)
	}
}

func TestApplyOverrides_InvalidEnvIgnored(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_POLL_INTERVAL", "abc")
	t.Setenv("CLAUDE_QUOTA_FONT_SIZE", "-5")
	t.Setenv("CLAUDE_QUOTA_FONT_NAME", "comic-sans")
	t.Setenv("CLAUDE_QUOTA_HALO_SIZE", "-2")
	t.Setenv("CLAUDE_QUOTA_ICON_SIZE", "-10")

	cfg := defaultConfig()
	applyOverrides(&cfg, 0, 0, "", -1, 0)
	if cfg.PollIntervalSeconds != 300 {
		t.Errorf("PollIntervalSeconds = %d, want 300 (invalid env should be ignored)", cfg.PollIntervalSeconds)
	}
	if cfg.FontSize != 34 {
		t.Errorf("FontSize = %f, want 34 (invalid env should be ignored)", cfg.FontSize)
	}
	if cfg.FontName != "bold" {
		t.Errorf("FontName = %q, want %q (invalid env should be ignored)", cfg.FontName, "bold")
	}
	if cfg.HaloSize != 2 {
		t.Errorf("HaloSize = %f, want 2 (invalid env should be ignored)", cfg.HaloSize)
	}
	if cfg.IconSize != 64 {
		t.Errorf("IconSize = %d, want 64 (invalid env should be ignored)", cfg.IconSize)
	}
}

func TestApplyOverrides_InvalidFlagIgnored(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, 0, 0, "unknown-font", -1, 0)
	if cfg.FontName != "bold" {
		t.Errorf("FontName = %q, want %q (invalid flag should be ignored)", cfg.FontName, "bold")
	}
}

func TestApplyOverrides_HaloZeroDisables(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, 0, 0, "", 0, 0)
	if cfg.HaloSize != 0 {
		t.Errorf("HaloSize = %f, want 0 (flag 0 should disable halo)", cfg.HaloSize)
	}
}

func TestApplyOverrides_HaloEnvZeroDisables(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_HALO_SIZE", "0")
	cfg := defaultConfig()
	applyOverrides(&cfg, 0, 0, "", -1, 0)
	if cfg.HaloSize != 0 {
		t.Errorf("HaloSize = %f, want 0 (env 0 should disable halo)", cfg.HaloSize)
	}
}
