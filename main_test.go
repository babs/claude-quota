package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
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
	t.Setenv("CLAUDE_QUOTA_PROVIDER_MARK", "true")
	t.Setenv("CLAUDE_QUOTA_PROVIDER_MARK_SIZE", "16")
	t.Setenv("CLAUDE_QUOTA_PROVIDER_MARK_POSITION", "nw")

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
	if !cfg.ProviderMark {
		t.Errorf("ProviderMark = %v, want true", cfg.ProviderMark)
	}
	if cfg.ProviderMarkSize != 16 {
		t.Errorf("ProviderMarkSize = %f, want 16", cfg.ProviderMarkSize)
	}
	if cfg.ProviderMarkPosition != "NW" {
		t.Errorf("ProviderMarkPosition = %q, want NW", cfg.ProviderMarkPosition)
	}
}

func TestApplyOverrides_FlagOverridesEnv(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_POLL_INTERVAL", "120")
	t.Setenv("CLAUDE_QUOTA_FONT_SIZE", "32")
	t.Setenv("CLAUDE_QUOTA_FONT_NAME", "bitmap")
	t.Setenv("CLAUDE_QUOTA_HALO_SIZE", "2.5")
	t.Setenv("CLAUDE_QUOTA_ICON_SIZE", "128")
	t.Setenv("CLAUDE_QUOTA_PROVIDER_MARK", "false")
	t.Setenv("CLAUDE_QUOTA_PROVIDER_MARK_SIZE", "12")
	t.Setenv("CLAUDE_QUOTA_PROVIDER_MARK_POSITION", "nw")

	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{
		PollInterval: 60, FontSize: 24, FontName: "mono",
		HaloSize: 0.5, IconSize: 256,
		ProviderMark: boolPtr(true), ProviderMarkSize: 18, ProviderMarkPosition: "se",
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
	if !cfg.ProviderMark {
		t.Errorf("ProviderMark = %v, want true (flag should override env)", cfg.ProviderMark)
	}
	if cfg.ProviderMarkSize != 18 {
		t.Errorf("ProviderMarkSize = %f, want 18", cfg.ProviderMarkSize)
	}
	if cfg.ProviderMarkPosition != "SE" {
		t.Errorf("ProviderMarkPosition = %q, want SE", cfg.ProviderMarkPosition)
	}
}

func TestApplyOverrides_InvalidEnvIgnored(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_POLL_INTERVAL", "abc")
	t.Setenv("CLAUDE_QUOTA_FONT_SIZE", "-5")
	t.Setenv("CLAUDE_QUOTA_FONT_NAME", "comic-sans")
	t.Setenv("CLAUDE_QUOTA_HALO_SIZE", "-2")
	t.Setenv("CLAUDE_QUOTA_ICON_SIZE", "-10")
	t.Setenv("CLAUDE_QUOTA_PROVIDER_MARK", "maybe")
	t.Setenv("CLAUDE_QUOTA_PROVIDER_MARK_SIZE", "-2")
	t.Setenv("CLAUDE_QUOTA_PROVIDER_MARK_POSITION", "center")

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
	if cfg.ProviderMark {
		t.Errorf("ProviderMark = %v, want false", cfg.ProviderMark)
	}
	if cfg.ProviderMarkSize != 14 {
		t.Errorf("ProviderMarkSize = %f, want 14", cfg.ProviderMarkSize)
	}
	if cfg.ProviderMarkPosition != "SE" {
		t.Errorf("ProviderMarkPosition = %q, want SE", cfg.ProviderMarkPosition)
	}
}

func TestApplyOverrides_InvalidFlagIgnored(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{FontName: "unknown-font", HaloSize: -1, ProviderMarkPosition: "middle", ProviderMarkColor: "not-a-color"})
	if cfg.FontName != "bold" {
		t.Errorf("FontName = %q, want %q (invalid flag should be ignored)", cfg.FontName, "bold")
	}
	if cfg.ProviderMarkPosition != "SE" {
		t.Errorf("ProviderMarkPosition = %q, want SE (invalid flag should be ignored)", cfg.ProviderMarkPosition)
	}
	if cfg.ProviderMarkColor != "" {
		t.Errorf("ProviderMarkColor = %q, want empty (invalid flag should be ignored)", cfg.ProviderMarkColor)
	}
}

func TestApplyOverrides_ProviderMarkColorFlag(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: -1, ProviderMarkColor: "#DE7356"})
	if cfg.ProviderMarkColor != "#DE7356" {
		t.Errorf("ProviderMarkColor = %q, want #DE7356", cfg.ProviderMarkColor)
	}
}

func TestApplyOverrides_ProviderMarkColorEnvVar(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_PROVIDER_MARK_COLOR", "#DE7356")
	cfg := defaultConfig()
	applyOverrides(&cfg, noOverrides)
	if cfg.ProviderMarkColor != "#DE7356" {
		t.Errorf("ProviderMarkColor = %q, want #DE7356", cfg.ProviderMarkColor)
	}
}

func TestApplyOverrides_ProviderMarkColorInvalidEnvIgnored(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_PROVIDER_MARK_COLOR", "not-a-color")
	cfg := defaultConfig()
	applyOverrides(&cfg, noOverrides)
	if cfg.ProviderMarkColor != "" {
		t.Errorf("ProviderMarkColor = %q, want empty (invalid env should be ignored)", cfg.ProviderMarkColor)
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
	if cfg.Thresholds.Warning != 80 {
		t.Errorf("Warning = %f, want 80 (invalid env should be ignored)", cfg.Thresholds.Warning)
	}
	if cfg.Thresholds.Critical != 95 {
		t.Errorf("Critical = %f, want 95 (invalid env should be ignored)", cfg.Thresholds.Critical)
	}
}

func TestApplyOverrides_ThresholdFlagOver100Ignored(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: -1, WarningThreshold: 200, CriticalThreshold: 300})
	if cfg.Thresholds.Warning != 80 {
		t.Errorf("Warning = %f, want 80 (>100 flag should be ignored)", cfg.Thresholds.Warning)
	}
	if cfg.Thresholds.Critical != 95 {
		t.Errorf("Critical = %f, want 95 (>100 flag should be ignored)", cfg.Thresholds.Critical)
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

func TestApplyOverrides_IndicatorFlag(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: -1, Indicator: "bar"})
	if cfg.Indicator != "bar" {
		t.Errorf("Indicator = %q, want %q", cfg.Indicator, "bar")
	}
}

func TestApplyOverrides_IndicatorEnvVar(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_INDICATOR", "arc")
	cfg := defaultConfig()
	applyOverrides(&cfg, noOverrides)
	if cfg.Indicator != "arc" {
		t.Errorf("Indicator = %q, want %q", cfg.Indicator, "arc")
	}
}

func TestApplyOverrides_IndicatorFlagOverridesEnv(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_INDICATOR", "arc")
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: -1, Indicator: "bar"})
	if cfg.Indicator != "bar" {
		t.Errorf("Indicator = %q, want %q (flag should override env)", cfg.Indicator, "bar")
	}
}

func TestApplyOverrides_IndicatorInvalidEnvIgnored(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_INDICATOR", "gauge")
	cfg := defaultConfig()
	applyOverrides(&cfg, noOverrides)
	if cfg.Indicator != "pie" {
		t.Errorf("Indicator = %q, want %q (invalid env should be ignored)", cfg.Indicator, "pie")
	}
}

func TestApplyOverrides_IndicatorInvalidFlagIgnored(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: -1, Indicator: "unknown"})
	if cfg.Indicator != "pie" {
		t.Errorf("Indicator = %q, want %q (invalid flag should be ignored)", cfg.Indicator, "pie")
	}
}

func boolPtr(b bool) *bool { return &b }

func TestApplyOverrides_ShowTextFlag(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: -1, ShowText: boolPtr(false)})
	if configShowText(cfg) != false {
		t.Errorf("ShowText = %v, want false", configShowText(cfg))
	}
}

func TestApplyOverrides_ShowTextEnvVar(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_SHOW_TEXT", "false")
	cfg := defaultConfig()
	applyOverrides(&cfg, noOverrides)
	if configShowText(cfg) != false {
		t.Errorf("ShowText = %v, want false (env false)", configShowText(cfg))
	}
}

func TestApplyOverrides_ShowTextEnvVar0(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_SHOW_TEXT", "0")
	cfg := defaultConfig()
	applyOverrides(&cfg, noOverrides)
	if configShowText(cfg) != false {
		t.Errorf("ShowText = %v, want false (env 0)", configShowText(cfg))
	}
}

func TestApplyOverrides_ShowTextEnvVarTrue(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_SHOW_TEXT", "true")
	cfg := defaultConfig()
	// Start with false to verify env sets it back to true.
	cfg.ShowText = boolPtr(false)
	applyOverrides(&cfg, noOverrides)
	if configShowText(cfg) != true {
		t.Errorf("ShowText = %v, want true (env true)", configShowText(cfg))
	}
}

func TestApplyOverrides_ShowTextFlagOverridesEnv(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_SHOW_TEXT", "false")
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: -1, ShowText: boolPtr(true)})
	if configShowText(cfg) != true {
		t.Errorf("ShowText = %v, want true (flag should override env)", configShowText(cfg))
	}
}

func TestApplyOverrides_ShowTextInvalidEnvIgnored(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_SHOW_TEXT", "maybe")
	cfg := defaultConfig()
	applyOverrides(&cfg, noOverrides)
	if configShowText(cfg) != true {
		t.Errorf("ShowText = %v, want true (invalid env should be ignored)", configShowText(cfg))
	}
}

func TestApplyOverrides_ShowAccountFlag(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: -1, ShowAccount: boolPtr(true)})
	if cfg.ShowAccount != true {
		t.Errorf("ShowAccount = %v, want true", cfg.ShowAccount)
	}
}

func TestApplyOverrides_ShowAccountEnvVar(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_SHOW_ACCOUNT", "true")
	cfg := defaultConfig()
	applyOverrides(&cfg, noOverrides)
	if cfg.ShowAccount != true {
		t.Errorf("ShowAccount = %v, want true (env true)", cfg.ShowAccount)
	}
}

func TestApplyOverrides_ShowAccountEnvVar0(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_SHOW_ACCOUNT", "0")
	cfg := defaultConfig()
	cfg.ShowAccount = true
	applyOverrides(&cfg, noOverrides)
	if cfg.ShowAccount != false {
		t.Errorf("ShowAccount = %v, want false (env 0)", cfg.ShowAccount)
	}
}

func TestApplyOverrides_ShowAccountFlagOverridesEnv(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_SHOW_ACCOUNT", "true")
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: -1, ShowAccount: boolPtr(false)})
	if cfg.ShowAccount != false {
		t.Errorf("ShowAccount = %v, want false (flag should override env)", cfg.ShowAccount)
	}
}

func TestApplyOverrides_ShowAccountInvalidEnvIgnored(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_SHOW_ACCOUNT", "maybe")
	cfg := defaultConfig()
	applyOverrides(&cfg, noOverrides)
	if cfg.ShowAccount != false {
		t.Errorf("ShowAccount = %v, want false (invalid env should be ignored)", cfg.ShowAccount)
	}
}

func TestApplyOverrides_StatsFlag(t *testing.T) {
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: -1, Stats: boolPtr(true)})
	if cfg.Stats != true {
		t.Errorf("Stats = %v, want true", cfg.Stats)
	}
}

func TestApplyOverrides_StatsEnvVar(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_STATS", "true")
	cfg := defaultConfig()
	applyOverrides(&cfg, noOverrides)
	if cfg.Stats != true {
		t.Errorf("Stats = %v, want true (env true)", cfg.Stats)
	}
}

func TestApplyOverrides_StatsEnvVar0(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_STATS", "0")
	cfg := defaultConfig()
	cfg.Stats = true
	applyOverrides(&cfg, noOverrides)
	if cfg.Stats != false {
		t.Errorf("Stats = %v, want false (env 0)", cfg.Stats)
	}
}

func TestApplyOverrides_StatsFlagOverridesEnv(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_STATS", "true")
	cfg := defaultConfig()
	applyOverrides(&cfg, overrides{HaloSize: -1, Stats: boolPtr(false)})
	if cfg.Stats != false {
		t.Errorf("Stats = %v, want false (flag should override env)", cfg.Stats)
	}
}

func TestApplyOverrides_StatsInvalidEnvIgnored(t *testing.T) {
	t.Setenv("CLAUDE_QUOTA_STATS", "maybe")
	cfg := defaultConfig()
	applyOverrides(&cfg, noOverrides)
	if cfg.Stats != false {
		t.Errorf("Stats = %v, want false (invalid env should be ignored)", cfg.Stats)
	}
}

func TestConfigShowText_NilDefault(t *testing.T) {
	cfg := Config{}
	if configShowText(cfg) != true {
		t.Errorf("configShowText(nil) = %v, want true", configShowText(cfg))
	}
}

func TestConfigShowText_False(t *testing.T) {
	cfg := Config{ShowText: boolPtr(false)}
	if configShowText(cfg) != false {
		t.Errorf("configShowText(false) = %v, want false", configShowText(cfg))
	}
}

// Note: dedicated tests for the provider flag/env precedence chain live in
// TestResolveProvider below, since applyOverrides no longer handles provider
// resolution.

// withIsolatedCredentials points the credential-path globals at files inside
// t.TempDir(), optionally creating them, and restores the originals on
// cleanup. Used to make resolveProvider's defaultProvider() fallback
// deterministic without touching the host filesystem.
func withIsolatedCredentials(t *testing.T, createClaude, createCodex bool) {
	t.Helper()
	origClaude := claudeCredentialsPath
	origCodex := codexAuthPath
	dir := t.TempDir()
	claudeCredentialsPath = filepath.Join(dir, ".claude", ".credentials.json")
	codexAuthPath = filepath.Join(dir, ".codex", "auth.json")
	if createClaude {
		if err := os.MkdirAll(filepath.Dir(claudeCredentialsPath), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(claudeCredentialsPath, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if createCodex {
		if err := os.MkdirAll(filepath.Dir(codexAuthPath), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(codexAuthPath, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		claudeCredentialsPath = origClaude
		codexAuthPath = origCodex
	})
}

func TestResolveCredentialPath(t *testing.T) {
	const subdir, filename = ".claude", ".credentials.json"
	join := func(home string) string { return filepath.Join(home, subdir, filename) }

	cases := []struct {
		name       string
		cfgHome    string
		cfgDirect  string
		envHome    string
		envDirect  string
		flagHome   string
		flagDirect string
		want       string
	}{
		{
			name: "all empty returns empty",
			want: "",
		},
		// Home chain: config < env < flag
		{
			name:    "config home sets path",
			cfgHome: "/cfg",
			want:    join("/cfg"),
		},
		{
			name:    "env home overrides config home",
			cfgHome: "/cfg",
			envHome: "/env",
			want:    join("/env"),
		},
		{
			name:     "flag home overrides env home",
			cfgHome:  "/cfg",
			envHome:  "/env",
			flagHome: "/flag",
			want:     join("/flag"),
		},
		// Direct chain: config < env < flag
		{
			name:      "config direct sets path",
			cfgDirect: "/cfg/creds.json",
			want:      "/cfg/creds.json",
		},
		{
			name:      "env direct overrides config direct",
			cfgDirect: "/cfg/creds.json",
			envDirect: "/env/creds.json",
			want:      "/env/creds.json",
		},
		{
			name:       "flag direct overrides env direct",
			cfgDirect:  "/cfg/creds.json",
			envDirect:  "/env/creds.json",
			flagDirect: "/flag/creds.json",
			want:       "/flag/creds.json",
		},
		// Direct always beats home regardless of source level (M1 fix)
		{
			name:      "config direct beats config home",
			cfgHome:   "/home",
			cfgDirect: "/direct/creds.json",
			want:      "/direct/creds.json",
		},
		{
			name:      "config direct beats env home",
			cfgDirect: "/direct/creds.json",
			envHome:   "/env",
			want:      "/direct/creds.json",
		},
		{
			name:      "config direct beats flag home",
			cfgDirect: "/direct/creds.json",
			flagHome:  "/flag",
			want:      "/direct/creds.json",
		},
		{
			name:      "env direct beats config home",
			cfgHome:   "/cfg",
			envDirect: "/env/creds.json",
			want:      "/env/creds.json",
		},
		{
			name:      "env direct beats env home",
			envHome:   "/env",
			envDirect: "/env/creds.json",
			want:      "/env/creds.json",
		},
		{
			name:      "env direct beats flag home",
			flagHome:  "/flag",
			envDirect: "/env/creds.json",
			want:      "/env/creds.json",
		},
		{
			name:       "flag direct beats config home",
			cfgHome:    "/cfg",
			flagDirect: "/flag/creds.json",
			want:       "/flag/creds.json",
		},
		{
			name:       "flag direct beats env home",
			envHome:    "/env",
			flagDirect: "/flag/creds.json",
			want:       "/flag/creds.json",
		},
		{
			name:       "flag direct beats flag home",
			flagHome:   "/flag",
			flagDirect: "/flag/creds.json",
			want:       "/flag/creds.json",
		},
		// All sources set: flag direct wins
		{
			name:       "flag direct wins all",
			cfgHome:    "/cfg",
			cfgDirect:  "/cfg/creds.json",
			envHome:    "/env",
			envDirect:  "/env/creds.json",
			flagHome:   "/flag",
			flagDirect: "/flag/creds.json",
			want:       "/flag/creds.json",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveCredentialPath(
				credentialSources{tc.cfgHome, tc.cfgDirect},
				credentialSources{tc.envHome, tc.envDirect},
				credentialSources{tc.flagHome, tc.flagDirect},
				subdir, filename,
			)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveProvider(t *testing.T) {
	cases := []struct {
		name      string
		flag      string
		env       string
		cfg       string
		want      Provider
		wantLog   string // substring; empty means no log expected
		createCl  bool
		createCdx bool
	}{
		{
			name: "flag wins over env and config",
			flag: "codex", env: "claude", cfg: "claude",
			want: ProviderCodex,
		},
		{
			name: "env wins over config when flag empty",
			env:  "codex", cfg: "claude",
			want: ProviderCodex,
		},
		{
			name: "config used when flag and env empty",
			cfg:  "codex",
			want: ProviderCodex,
		},
		{
			name:    "invalid flag falls through to env",
			flag:    "bogus",
			env:     "codex",
			want:    ProviderCodex,
			wantLog: `Ignoring invalid -provider="bogus"`,
		},
		{
			name:    "invalid env falls through to config",
			env:     "bogus",
			cfg:     "claude",
			want:    ProviderClaude,
			wantLog: `Ignoring invalid CLAUDE_QUOTA_PROVIDER="bogus"`,
		},
		{
			name:     "auto-detect fallback picks up claude creds",
			createCl: true,
			want:     ProviderClaude,
			wantLog:  "Auto-detected provider",
		},
		{
			name:      "auto-detect fallback picks up codex creds",
			createCdx: true,
			want:      ProviderCodex,
			wantLog:   "Auto-detected provider",
		},
		{
			// Locks in the invariant that defaultProvider() does NOT log
			// "Auto-detected provider" in the neither-credentials branch
			// (provider.go defaultProvider default case). If that branch
			// ever starts logging, this test will flag it.
			name: "auto-detect with no creds defaults to claude",
			want: ProviderClaude,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withIsolatedCredentials(t, tc.createCl, tc.createCdx)
			var buf bytes.Buffer
			origOut := log.Writer()
			log.SetOutput(&buf)
			t.Cleanup(func() { log.SetOutput(origOut) })

			got := resolveProvider(tc.flag, tc.env, tc.cfg)
			if got != tc.want {
				t.Errorf("resolveProvider(%q,%q,%q) = %q, want %q", tc.flag, tc.env, tc.cfg, got, tc.want)
			}
			if tc.wantLog == "" {
				if buf.Len() > 0 {
					t.Errorf("unexpected log output: %q", buf.String())
				}
			} else if !strings.Contains(buf.String(), tc.wantLog) {
				t.Errorf("log %q does not contain %q", buf.String(), tc.wantLog)
			}
		})
	}
}
