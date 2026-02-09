package main

import (
	"testing"
)

// noOverrides is the zero-value overrides struct that changes nothing.
// HaloSize -1 means "not set" (0 is a valid value that disables halo).
var noOverrides = overrides{HaloSize: -1}

func TestApplyOverrides_Defaults(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, noOverrides)
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
	applyOverrides(&cfg, overrides{
		PollInterval: 60, FontSize: 24, FontName: "mono",
		HaloSize: 3.0, IconSize: 128,
	})
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
	applyOverrides(&cfg, noOverrides)
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
	applyOverrides(&cfg, overrides{
		PollInterval: 60, FontSize: 24, FontName: "mono",
		HaloSize: 0.5, IconSize: 256,
	})
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
	applyOverrides(&cfg, noOverrides)
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
	applyOverrides(&cfg, overrides{FontName: "unknown-font", HaloSize: -1})
	if cfg.FontName != "bold" {
		t.Errorf("FontName = %q, want %q (invalid flag should be ignored)", cfg.FontName, "bold")
	}
}

func TestApplyOverrides_HaloZeroDisables(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: 0})
	if cfg.HaloSize != 0 {
		t.Errorf("HaloSize = %f, want 0 (flag 0 should disable halo)", cfg.HaloSize)
	}
}

func TestApplyOverrides_HaloEnvZeroDisables(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_HALO_SIZE", "0")
	cfg := defaultConfig()
	applyOverrides(&cfg, noOverrides)
	if cfg.HaloSize != 0 {
		t.Errorf("HaloSize = %f, want 0 (env 0 should disable halo)", cfg.HaloSize)
	}
}

func TestApplyOverrides_ThresholdFlags(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: -1, WarningThreshold: 50, CriticalThreshold: 90})
	if cfg.Thresholds.Warning != 50 {
		t.Errorf("Warning = %f, want 50", cfg.Thresholds.Warning)
	}
	if cfg.Thresholds.Critical != 90 {
		t.Errorf("Critical = %f, want 90", cfg.Thresholds.Critical)
	}
}

func TestApplyOverrides_ThresholdEnvVars(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_WARNING_THRESHOLD", "40")
	t.Setenv("CLAUDE_QUOTA_CRITICAL_THRESHOLD", "70")
	cfg := defaultConfig()
	applyOverrides(&cfg, noOverrides)
	if cfg.Thresholds.Warning != 40 {
		t.Errorf("Warning = %f, want 40", cfg.Thresholds.Warning)
	}
	if cfg.Thresholds.Critical != 70 {
		t.Errorf("Critical = %f, want 70", cfg.Thresholds.Critical)
	}
}

func TestApplyOverrides_ThresholdFlagOverridesEnv(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_WARNING_THRESHOLD", "40")
	t.Setenv("CLAUDE_QUOTA_CRITICAL_THRESHOLD", "70")
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: -1, WarningThreshold: 50, CriticalThreshold: 90})
	if cfg.Thresholds.Warning != 50 {
		t.Errorf("Warning = %f, want 50 (flag should override env)", cfg.Thresholds.Warning)
	}
	if cfg.Thresholds.Critical != 90 {
		t.Errorf("Critical = %f, want 90 (flag should override env)", cfg.Thresholds.Critical)
	}
}

func TestApplyOverrides_ThresholdInvalidEnvIgnored(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_WARNING_THRESHOLD", "abc")
	t.Setenv("CLAUDE_QUOTA_CRITICAL_THRESHOLD", "150")
	cfg := defaultConfig()
	applyOverrides(&cfg, noOverrides)
	if cfg.Thresholds.Warning != 60 {
		t.Errorf("Warning = %f, want 60 (invalid env should be ignored)", cfg.Thresholds.Warning)
	}
	if cfg.Thresholds.Critical != 85 {
		t.Errorf("Critical = %f, want 85 (invalid env should be ignored)", cfg.Thresholds.Critical)
	}
}

func TestApplyOverrides_ThresholdFlagOver100Ignored(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: -1, WarningThreshold: 200, CriticalThreshold: 300})
	if cfg.Thresholds.Warning != 60 {
		t.Errorf("Warning = %f, want 60 (>100 flag should be ignored)", cfg.Thresholds.Warning)
	}
	if cfg.Thresholds.Critical != 85 {
		t.Errorf("Critical = %f, want 85 (>100 flag should be ignored)", cfg.Thresholds.Critical)
	}
}

func TestApplyOverrides_ThresholdWarningGeCriticalSwaps(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: -1, WarningThreshold: 90, CriticalThreshold: 50})
	if cfg.Thresholds.Warning != 50 {
		t.Errorf("Warning = %f, want 50 (should have been swapped)", cfg.Thresholds.Warning)
	}
	if cfg.Thresholds.Critical != 90 {
		t.Errorf("Critical = %f, want 90 (should have been swapped)", cfg.Thresholds.Critical)
	}
}

func TestApplyOverrides_ThresholdEqualSwaps(t *testing.T) {
	cfg := defaultConfig()
	cfg.Thresholds.Warning = 70
	cfg.Thresholds.Critical = 70
	applyOverrides(&cfg, noOverrides)
	// Equal values: swapped, so warning < critical won't hold — but at least it's logged.
	// After swap both are 70, which is still equal. The swap is a no-op but the log fires.
	if cfg.Thresholds.Warning != 70 || cfg.Thresholds.Critical != 70 {
		t.Errorf("Equal thresholds should remain 70/70 after swap, got %f/%f", cfg.Thresholds.Warning, cfg.Thresholds.Critical)
	}
}
