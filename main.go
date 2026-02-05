package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
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

	fmt.Println("WARNING: This tool uses Claude Code's OAuth client ID to access your")
	fmt.Println("quota data via an undocumented API. This is not sanctioned by Anthropic")
	fmt.Println("and may violate the Terms of Service. Use at your own risk.")
	fmt.Println()

	// Check credentials exist.
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		fmt.Println("Claude Code credentials not found.")
		fmt.Printf("Expected: %s\n", credentialsPath)
		fmt.Println("\nRun 'claude login' to authenticate Claude Code first.")
		os.Exit(1)
	}

	fmt.Println(versionString())
	fmt.Printf("Credentials: %s\n", credentialsPath)
	fmt.Printf("Config: %s\n", configPath)

	cfg := loadConfig()
	applyOverrides(&cfg, *pollInterval, *fontSize)

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

// applyOverrides applies env vars and flags to config. Priority: flag > env > config file.
func applyOverrides(cfg *Config, flagPollInterval int, flagFontSize float64) {
	if v := os.Getenv("CLAUDE_QUOTA_POLL_INTERVAL"); v != "" {
		if i, err := strconv.Atoi(v); err != nil || i <= 0 {
			log.Printf("Ignoring invalid CLAUDE_QUOTA_POLL_INTERVAL=%q", v)
		} else {
			cfg.PollIntervalSeconds = i
		}
	}
	if flagPollInterval > 0 {
		cfg.PollIntervalSeconds = flagPollInterval
	}

	if v := os.Getenv("CLAUDE_QUOTA_FONT_SIZE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err != nil || f <= 0 {
			log.Printf("Ignoring invalid CLAUDE_QUOTA_FONT_SIZE=%q", v)
		} else {
			cfg.FontSize = f
		}
	}
	if flagFontSize > 0 {
		cfg.FontSize = flagFontSize
	}
}
