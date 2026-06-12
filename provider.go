package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Provider string

const (
	ProviderClaude Provider = "claude"
	ProviderCodex  Provider = "codex"
)

var (
	claudeCredentialsPath string
	codexAuthPath         string
)

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	claudeCredentialsPath = filepath.Join(home, ".claude", ".credentials.json")
	codexAuthPath = filepath.Join(home, ".codex", "auth.json")
}

func normalizeProvider(v string) Provider {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case string(ProviderCodex):
		return ProviderCodex
	default:
		return ProviderClaude
	}
}

func ValidProviderName(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case string(ProviderClaude), string(ProviderCodex):
		return true
	default:
		return false
	}
}

func providerDisplayName(provider Provider) string {
	switch provider {
	case ProviderCodex:
		return "Codex"
	default:
		return "Claude"
	}
}

// providerQuotaTitle returns the user-visible title for a tray instance,
// matching the installer-emitted desktop file Name (`Agent Quota (Claude)` /
// `Agent Quota (Codex)`). Used for the systray tooltip and the multi-line
// tooltip header so the launcher entry, the systray icon hover, and the menu
// header all read consistently.
func providerQuotaTitle(provider Provider) string {
	return "Agent Quota (" + providerDisplayName(provider) + ")"
}

func loginCommand(provider Provider) string {
	if provider == ProviderCodex {
		return "codex login"
	}
	return "claude login"
}

func defaultProvider() Provider {
	claudeInfo, claudeOK := fileInfo(claudeCredentialsPath)
	codexInfo, codexOK := fileInfo(codexAuthPath)

	var p Provider
	switch {
	case claudeOK && codexOK:
		p = newerProvider(claudeInfo.ModTime(), codexInfo.ModTime())
		log.Printf("Auto-detected provider %q (both credential files exist, picked most recently modified)", p)
	case claudeOK:
		p = ProviderClaude
		log.Printf("Auto-detected provider %q (found Claude credentials only)", p)
	case codexOK:
		p = ProviderCodex
		log.Printf("Auto-detected provider %q (found Codex credentials only)", p)
	default:
		p = ProviderClaude
	}
	return p
}

func fileInfo(path string) (os.FileInfo, bool) {
	if path == "" {
		return nil, false
	}
	info, err := os.Stat(path)
	return info, err == nil
}

func newerProvider(claudeModTime, codexModTime time.Time) Provider {
	if codexModTime.After(claudeModTime) {
		return ProviderCodex
	}
	return ProviderClaude
}
