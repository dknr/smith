package llm

import (
	"smith/config"
)

// NewProvider creates a Provider from the given config.
func NewProvider(cfg *config.Config) Provider {
	return &HTTPProvider{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
	}
}
