package main

import (
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

func providerQuotaTitle(provider Provider) string {
	return providerDisplayName(provider) + " Quota"
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

	switch {
	case claudeOK && codexOK:
		return newerProvider(claudeInfo.ModTime(), codexInfo.ModTime())
	case claudeOK:
		return ProviderClaude
	case codexOK:
		return ProviderCodex
	default:
		return ProviderClaude
	}
}

func fileExists(path string) bool {
	_, ok := fileInfo(path)
	return ok
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
