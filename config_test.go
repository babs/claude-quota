package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Default(t *testing.T) {
	orig := configPath
	defer func() { configPath = orig }()

	configPath = filepath.Join(t.TempDir(), "config.json")

	cfg := loadConfig()
	if cfg.PollIntervalSeconds != 300 {
		t.Errorf("PollIntervalSeconds = %d, want 300", cfg.PollIntervalSeconds)
	}
	if cfg.FontSize != 34 {
		t.Errorf("FontSize = %f, want 34", cfg.FontSize)
	}
	if cfg.Thresholds.Warning != 60 {
		t.Errorf("Warning = %f, want 60", cfg.Thresholds.Warning)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("loadConfig should create default config file")
	}
}

func TestLoadConfig_ExistingFile(t *testing.T) {
	orig := configPath
	defer func() { configPath = orig }()

	dir := t.TempDir()
	configPath = filepath.Join(dir, "config.json")

	custom := Config{
		PollIntervalSeconds: 60,
		FontSize:            24,
		Thresholds:          Thresholds{Warning: 50, Critical: 90},
	}
	data, _ := json.MarshalIndent(custom, "", "  ")
	os.WriteFile(configPath, data, 0600)

	cfg := loadConfig()
	if cfg.PollIntervalSeconds != 60 {
		t.Errorf("PollIntervalSeconds = %d, want 60", cfg.PollIntervalSeconds)
	}
	if cfg.FontSize != 24 {
		t.Errorf("FontSize = %f, want 24", cfg.FontSize)
	}
	if cfg.Thresholds.Critical != 90 {
		t.Errorf("Critical = %f, want 90", cfg.Thresholds.Critical)
	}
}

func TestLoadConfig_PartialFile(t *testing.T) {
	orig := configPath
	defer func() { configPath = orig }()

	dir := t.TempDir()
	configPath = filepath.Join(dir, "config.json")
	os.WriteFile(configPath, []byte(`{"poll_interval_seconds": 60}`), 0600)

	cfg := loadConfig()
	if cfg.PollIntervalSeconds != 60 {
		t.Errorf("PollIntervalSeconds = %d, want 60", cfg.PollIntervalSeconds)
	}
	if cfg.FontSize != 34 {
		t.Errorf("FontSize = %f, want 34 (default)", cfg.FontSize)
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	orig := configPath
	defer func() { configPath = orig }()

	dir := t.TempDir()
	configPath = filepath.Join(dir, "config.json")
	os.WriteFile(configPath, []byte(`{broken`), 0600)

	cfg := loadConfig()
	if cfg.PollIntervalSeconds != 300 {
		t.Errorf("PollIntervalSeconds = %d, want 300 (default on parse error)", cfg.PollIntervalSeconds)
	}
}

func TestSaveConfig(t *testing.T) {
	orig := configPath
	defer func() { configPath = orig }()

	dir := t.TempDir()
	configPath = filepath.Join(dir, "config.json")

	cfg := defaultConfig()
	cfg.PollIntervalSeconds = 120
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig() error: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}

	var loaded Config
	json.Unmarshal(data, &loaded)
	if loaded.PollIntervalSeconds != 120 {
		t.Errorf("PollIntervalSeconds = %d, want 120", loaded.PollIntervalSeconds)
	}
}

func TestWriteFileSecure_Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "file.txt")

	if err := writeFileSecure(path, []byte("test")); err != nil {
		t.Fatalf("writeFileSecure() error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "test" {
		t.Errorf("content = %q, want %q", string(data), "test")
	}
}
