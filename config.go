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
	ClaudeHome          string     `json:"claude_home,omitempty"`
	PollIntervalSeconds int        `json:"poll_interval_seconds"`
	FontSize            float64    `json:"font_size"`
	FontName            string     `json:"font_name"`
	HaloSize            float64    `json:"halo_size"`
	IconSize            int        `json:"icon_size"`
	Indicator           string     `json:"indicator"`
	ShowText            *bool      `json:"show_text"`
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
	showText := true
	return Config{
		PollIntervalSeconds: 300,
		FontSize:            34,
		FontName:            "bold",
		HaloSize:            2,
		IconSize:            64,
		Indicator:           "pie",
		ShowText:            &showText,
		Thresholds: Thresholds{
			Warning:  60,
			Critical: 85,
		},
	}
}

// configShowText dereferences ShowText with a default of true.
func configShowText(cfg Config) bool {
	if cfg.ShowText == nil {
		return true
	}
	return *cfg.ShowText
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

	defaults := defaultConfig()
	if cfg.IconSize <= 0 {
		log.Printf("Invalid icon_size %d in config, using default %d", cfg.IconSize, defaults.IconSize)
		cfg.IconSize = defaults.IconSize
	}
	if cfg.FontSize <= 0 {
		log.Printf("Invalid font_size %v in config, using default %v", cfg.FontSize, defaults.FontSize)
		cfg.FontSize = defaults.FontSize
	}
	if cfg.PollIntervalSeconds <= 0 {
		log.Printf("Invalid poll_interval_seconds %d in config, using default %d", cfg.PollIntervalSeconds, defaults.PollIntervalSeconds)
		cfg.PollIntervalSeconds = defaults.PollIntervalSeconds
	}
	if cfg.HaloSize < 0 {
		log.Printf("Invalid halo_size %v in config, using default %v", cfg.HaloSize, defaults.HaloSize)
		cfg.HaloSize = defaults.HaloSize
	}
	if cfg.FontName == "" || !ValidFontName(cfg.FontName) {
		if cfg.FontName != "" {
			log.Printf("Unknown font_name %q in config, using default %q", cfg.FontName, defaults.FontName)
		}
		cfg.FontName = defaults.FontName
	}
	if cfg.Indicator == "" || !ValidIndicatorName(cfg.Indicator) {
		if cfg.Indicator != "" {
			log.Printf("Unknown indicator %q in config, using default %q", cfg.Indicator, defaults.Indicator)
		}
		cfg.Indicator = defaults.Indicator
	}
	if cfg.ShowText == nil {
		cfg.ShowText = defaults.ShowText
	}
	if cfg.Thresholds.Warning <= 0 || cfg.Thresholds.Warning > 100 {
		log.Printf("Invalid thresholds.warning %v in config, using default %v", cfg.Thresholds.Warning, defaults.Thresholds.Warning)
		cfg.Thresholds.Warning = defaults.Thresholds.Warning
	}
	if cfg.Thresholds.Critical <= 0 || cfg.Thresholds.Critical > 100 {
		log.Printf("Invalid thresholds.critical %v in config, using default %v", cfg.Thresholds.Critical, defaults.Thresholds.Critical)
		cfg.Thresholds.Critical = defaults.Thresholds.Critical
	}
	if cfg.Thresholds.Warning >= cfg.Thresholds.Critical {
		log.Printf("Config thresholds.warning (%.0f) >= thresholds.critical (%.0f), swapping", cfg.Thresholds.Warning, cfg.Thresholds.Critical)
		cfg.Thresholds.Warning, cfg.Thresholds.Critical = cfg.Thresholds.Critical, cfg.Thresholds.Warning
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
