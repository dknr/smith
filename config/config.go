package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds the LLM provider configuration.
type Config struct {
	BaseURL string `toml:"base_url"`
	APIKey  string `toml:"api_key"`
	Model   string `toml:"model"`
}

// configName is the name of the config file.
const configName = "smith.toml"

// configDir returns the XDG config directory for smith.
func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "smith")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".config", "smith")
}

// Load reads smith.toml from the XDG config directory and returns the parsed config.
// Returns an error if the file is not found or cannot be parsed.
func Load() (*Config, error) {
	dir := configDir()
	path := filepath.Join(dir, configName)

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config file %s not found: %w", path, err)
	}
	defer f.Close()

	var cfg Config
	if _, err := toml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("base_url is required in %s", configName)
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("model is required in %s", configName)
	}

	return &cfg, nil
}
