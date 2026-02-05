package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Config holds the widget configuration.
type Config struct {
	PollIntervalSeconds int        `json:"poll_interval_seconds"`
	FontSize            float64    `json:"font_size"`
	Thresholds          Thresholds `json:"thresholds"`
}

// Thresholds defines warning/critical utilization levels.
type Thresholds struct {
	Warning  float64 `json:"warning"`
	Critical float64 `json:"critical"`
}

var configPath string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	configPath = filepath.Join(home, ".config", "claude-quota", "config.json")
}

// defaultConfig returns a Config with default values.
func defaultConfig() Config {
	return Config{
		PollIntervalSeconds: 300,
		FontSize:            18,
		Thresholds: Thresholds{
			Warning:  60,
			Critical: 85,
		},
	}
}

// loadConfig loads config from disk, creating a default if it doesn't exist.
// Missing fields keep their defaults via json.Unmarshal into a pre-populated struct.
func loadConfig() Config {
	cfg := defaultConfig()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			if writeErr := saveConfig(cfg); writeErr != nil {
				log.Printf("Failed to write default config: %v", writeErr)
			} else {
				log.Printf("Created default config at %s", configPath)
			}
			return cfg
		}
		log.Printf("Failed to read config %s: %v", configPath, err)
		return cfg
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("Failed to parse config %s: %v", configPath, err)
		return defaultConfig()
	}

	return cfg
}

// saveConfig writes config to disk with restrictive permissions (0600).
func saveConfig(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return writeFileSecure(configPath, data)
}

// writeFileSecure writes data to path with 0600 permissions, creating parent dirs.
func writeFileSecure(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
