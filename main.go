package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"
)

// Build-time variables injected via ldflags.
var (
	Version        = "v0.0.0"
	CommitHash     = "dev"
	BuildTimestamp = "1970-01-01T00:00:00Z"
	Builder        = "unknown"
	GithubRepo     = "babs/claude-quota"
)

func versionString() string {
	return fmt.Sprintf("claude-quota %s-%s", Version, CommitHash)
}

func versionStringLong() string {
	return fmt.Sprintf("claude-quota %s-%s (built %s using %s)\nhttps://github.com/%s\n",
		Version, CommitHash, BuildTimestamp, Builder, GithubRepo)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[claude-quota] ")

	showVersion := flag.Bool("version", false, "show version and exit")
	doUpdate := flag.Bool("update", false, "check and update to latest release")
	pollInterval := flag.Int("poll-interval", 0, "poll interval in seconds (env: CLAUDE_QUOTA_POLL_INTERVAL)")
	fontSize := flag.Float64("font-size", 0, "icon font size (env: CLAUDE_QUOTA_FONT_SIZE)")
	fontName := flag.String("font-name", "", "icon font name: bold, regular, mono, monobold, bitmap (env: CLAUDE_QUOTA_FONT_NAME)")
	haloSize := flag.Float64("halo-size", -1, "text halo/outline size in pixels, 0 to disable (env: CLAUDE_QUOTA_HALO_SIZE)")
	iconSize := flag.Int("icon-size", 0, "icon size in pixels (env: CLAUDE_QUOTA_ICON_SIZE)")
	warningThreshold := flag.Float64("warning-threshold", 0, "warning utilization threshold in % (env: CLAUDE_QUOTA_WARNING_THRESHOLD)")
	criticalThreshold := flag.Float64("critical-threshold", 0, "critical utilization threshold in % (env: CLAUDE_QUOTA_CRITICAL_THRESHOLD)")
	indicator := flag.String("indicator", "", "indicator type: pie, bar, arc, bar-proj (env: CLAUDE_QUOTA_INDICATOR)")
	showText := flag.Bool("show-text", true, "show percentage text on icon (env: CLAUDE_QUOTA_SHOW_TEXT)")
	claudeHome := flag.String("claude-home", "", "home directory for Claude credentials (env: CLAUDE_QUOTA_CLAUDE_HOME)")
	flag.Usage = func() {
		fmt.Print(versionStringLong())
		fmt.Fprintf(os.Stderr, "\nUsage: %s [options]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Print(versionStringLong())
		return
	}

	if *doUpdate {
		selfUpdate()
		return
	}

	cfg := loadConfig()

	// Resolve claude-home: config < env < flag.
	if cfg.ClaudeHome != "" {
		credentialsPath = filepath.Join(cfg.ClaudeHome, ".claude", ".credentials.json")
	}
	if v := os.Getenv("CLAUDE_QUOTA_CLAUDE_HOME"); v != "" {
		credentialsPath = filepath.Join(v, ".claude", ".credentials.json")
	}
	if *claudeHome != "" {
		credentialsPath = filepath.Join(*claudeHome, ".claude", ".credentials.json")
	}

	fmt.Println("WARNING: This tool uses Claude Code's OAuth client ID to access your")
	fmt.Println("quota data via an undocumented API. This is not sanctioned by Anthropic")
	fmt.Println("and may violate the Terms of Service. Use at your own risk.")
	fmt.Println()

	credentialsPreCheck()

	fmt.Println(versionString())
	fmt.Printf("Credentials: %s\n", credentialsPath)
	fmt.Printf("Config: %s\n", configPath)

	// Only pass ShowText when the user explicitly set -show-text.
	// flag.Bool defaults to true, so we can't distinguish "not set" from
	// "-show-text=true" without flag.Visit.
	var showTextOverride *bool
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "show-text" {
			showTextOverride = showText
		}
	})

	applyOverrides(&cfg, overrides{
		PollInterval:      *pollInterval,
		FontSize:          *fontSize,
		FontName:          *fontName,
		HaloSize:          *haloSize,
		IconSize:          *iconSize,
		Indicator:         *indicator,
		ShowText:          showTextOverride,
		WarningThreshold:  *warningThreshold,
		CriticalThreshold: *criticalThreshold,
	})

	client := &http.Client{Timeout: 30 * time.Second}

	creds, err := NewOAuthCredentials()
	if err != nil {
		fmt.Printf("\nError: %v\n", err)
		os.Exit(1)
	}

	app := NewApp(cfg, creds, client)

	// Handle interrupt for clean shutdown (SIGINT on all platforms, SIGTERM on Unix).
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	notifyExtraSignals(sigCh)
	go func() {
		<-sigCh
		log.Println("Signal received, shutting down...")
		app.Shutdown()
	}()

	app.Run()
}

// overrides holds CLI flag values for config overrides.
type overrides struct {
	PollInterval      int
	FontSize          float64
	FontName          string
	HaloSize          float64
	IconSize          int
	Indicator         string
	ShowText          *bool
	WarningThreshold  float64
	CriticalThreshold float64
}

// applyIntOverride applies an int override from env var and flag.
// The env value is parsed with Atoi; both env and flag values are accepted only if valid returns true.
func applyIntOverride(target *int, envKey string, flagVal int, valid func(int) bool) {
	if v := os.Getenv(envKey); v != "" {
		if i, err := strconv.Atoi(v); err != nil || !valid(i) {
			log.Printf("Ignoring invalid %s=%q", envKey, v)
		} else {
			*target = i
		}
	}
	if valid(flagVal) {
		*target = flagVal
	}
}

// applyFloatOverride applies a float64 override from env var and flag.
// flagIsSet indicates whether the flag was explicitly provided (since zero may be a valid value).
func applyFloatOverride(target *float64, envKey string, flagVal float64, flagIsSet bool, valid func(float64) bool) {
	if v := os.Getenv(envKey); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err != nil || !valid(f) {
			log.Printf("Ignoring invalid %s=%q", envKey, v)
		} else {
			*target = f
		}
	}
	if flagIsSet && valid(flagVal) {
		*target = flagVal
	}
}

// applyStringOverride applies a string override from env var and flag.
// Non-empty values are accepted only if valid returns true.
func applyStringOverride(target *string, envKey, flagName, flagVal string, valid func(string) bool) {
	if v := os.Getenv(envKey); v != "" {
		if !valid(v) {
			log.Printf("Ignoring invalid %s=%q", envKey, v)
		} else {
			*target = v
		}
	}
	if flagVal != "" {
		if !valid(flagVal) {
			log.Printf("Ignoring invalid -%s=%q", flagName, flagVal)
		} else {
			*target = flagVal
		}
	}
}

// applyOverrides applies env vars and flags to config. Priority: flag > env > config file.
func applyOverrides(cfg *Config, o overrides) {
	applyIntOverride(&cfg.PollIntervalSeconds, "CLAUDE_QUOTA_POLL_INTERVAL", o.PollInterval,
		func(i int) bool { return i > 0 })
	applyFloatOverride(&cfg.FontSize, "CLAUDE_QUOTA_FONT_SIZE", o.FontSize, o.FontSize > 0,
		func(f float64) bool { return f > 0 })
	applyStringOverride(&cfg.FontName, "CLAUDE_QUOTA_FONT_NAME", "font-name", o.FontName, ValidFontName)
	applyFloatOverride(&cfg.HaloSize, "CLAUDE_QUOTA_HALO_SIZE", o.HaloSize, o.HaloSize >= 0,
		func(f float64) bool { return f >= 0 })
	applyIntOverride(&cfg.IconSize, "CLAUDE_QUOTA_ICON_SIZE", o.IconSize,
		func(i int) bool { return i > 0 })
	applyStringOverride(&cfg.Indicator, "CLAUDE_QUOTA_INDICATOR", "indicator", o.Indicator, ValidIndicatorName)

	// ShowText: unique tri-state parsing (true/1, false/0).
	if v := os.Getenv("CLAUDE_QUOTA_SHOW_TEXT"); v != "" {
		switch v {
		case "true", "1":
			b := true
			cfg.ShowText = &b
		case "false", "0":
			b := false
			cfg.ShowText = &b
		default:
			log.Printf("Ignoring invalid CLAUDE_QUOTA_SHOW_TEXT=%q", v)
		}
	}
	if o.ShowText != nil {
		cfg.ShowText = o.ShowText
	}

	// Thresholds: cross-field validation, kept inline.
	if v := os.Getenv("CLAUDE_QUOTA_WARNING_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err != nil || f <= 0 || f > 100 {
			log.Printf("Ignoring invalid CLAUDE_QUOTA_WARNING_THRESHOLD=%q", v)
		} else {
			cfg.Thresholds.Warning = f
		}
	}
	if o.WarningThreshold > 0 && o.WarningThreshold <= 100 {
		cfg.Thresholds.Warning = o.WarningThreshold
	} else if o.WarningThreshold > 100 {
		log.Printf("Ignoring invalid -warning-threshold=%.0f (must be 1-100)", o.WarningThreshold)
	}

	if v := os.Getenv("CLAUDE_QUOTA_CRITICAL_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err != nil || f <= 0 || f > 100 {
			log.Printf("Ignoring invalid CLAUDE_QUOTA_CRITICAL_THRESHOLD=%q", v)
		} else {
			cfg.Thresholds.Critical = f
		}
	}
	if o.CriticalThreshold > 0 && o.CriticalThreshold <= 100 {
		cfg.Thresholds.Critical = o.CriticalThreshold
	} else if o.CriticalThreshold > 100 {
		log.Printf("Ignoring invalid -critical-threshold=%.0f (must be 1-100)", o.CriticalThreshold)
	}

	if cfg.Thresholds.Warning >= cfg.Thresholds.Critical {
		log.Printf("Warning threshold (%.0f) >= critical threshold (%.0f), swapping", cfg.Thresholds.Warning, cfg.Thresholds.Critical)
		cfg.Thresholds.Warning, cfg.Thresholds.Critical = cfg.Thresholds.Critical, cfg.Thresholds.Warning
	}
}
