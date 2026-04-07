package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultProvider_ClaudeOnly(t *testing.T) {
	origClaude := claudeCredentialsPath
	origCodex := codexAuthPath
	defer func() {
		claudeCredentialsPath = origClaude
		codexAuthPath = origCodex
	}()

	dir := t.TempDir()
	claudeCredentialsPath = filepath.Join(dir, "claude.json")
	codexAuthPath = filepath.Join(dir, "codex.json")

	if err := os.WriteFile(claudeCredentialsPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	if got := defaultProvider(); got != ProviderClaude {
		t.Fatalf("defaultProvider() = %q, want claude", got)
	}
}

func TestDefaultProvider_CodexOnly(t *testing.T) {
	origClaude := claudeCredentialsPath
	origCodex := codexAuthPath
	defer func() {
		claudeCredentialsPath = origClaude
		codexAuthPath = origCodex
	}()

	dir := t.TempDir()
	claudeCredentialsPath = filepath.Join(dir, "claude.json")
	codexAuthPath = filepath.Join(dir, "codex.json")

	if err := os.WriteFile(codexAuthPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	if got := defaultProvider(); got != ProviderCodex {
		t.Fatalf("defaultProvider() = %q, want codex", got)
	}
}

func TestDefaultProvider_NewestFileWins(t *testing.T) {
	origClaude := claudeCredentialsPath
	origCodex := codexAuthPath
	defer func() {
		claudeCredentialsPath = origClaude
		codexAuthPath = origCodex
	}()

	dir := t.TempDir()
	claudeCredentialsPath = filepath.Join(dir, "claude.json")
	codexAuthPath = filepath.Join(dir, "codex.json")

	if err := os.WriteFile(claudeCredentialsPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexAuthPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	claudeTime := time.Now().Add(-2 * time.Hour)
	codexTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(claudeCredentialsPath, claudeTime, claudeTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(codexAuthPath, codexTime, codexTime); err != nil {
		t.Fatal(err)
	}

	if got := defaultProvider(); got != ProviderCodex {
		t.Fatalf("defaultProvider() = %q, want codex when codex credentials are newer", got)
	}

	newClaudeTime := time.Now()
	if err := os.Chtimes(claudeCredentialsPath, newClaudeTime, newClaudeTime); err != nil {
		t.Fatal(err)
	}
	if got := defaultProvider(); got != ProviderClaude {
		t.Fatalf("defaultProvider() = %q, want claude when claude credentials are newer", got)
	}
}
