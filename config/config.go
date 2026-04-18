package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Config holds the LLM provider configuration.
type Config struct {
	BaseURL string `toml:"base_url"`
	APIKey  string `toml:"api_key"`
	Model   string `toml:"model"`
}

// Load reads smith.toml from the current working directory and returns the parsed config.
// Returns an error if the file is not found or cannot be parsed.
func Load() (*Config, error) {
	_, err := os.Stat("smith.toml")
	if err != nil {
		return nil, fmt.Errorf("config file smith.toml not found: %w", err)
	}

	var cfg Config
	if _, err := toml.DecodeFile("smith.toml", &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse smith.toml: %w", err)
	}

	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("base_url is required in smith.toml")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("model is required in smith.toml")
	}

	return &cfg, nil
}
