package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
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

	client := &http.Client{Timeout: 30 * time.Second}

	creds, err := NewOAuthCredentials(client)
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
