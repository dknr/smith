package llm

import (
	"smith/config"
	"smith/tools"
)

// NewProvider creates a Provider from the given config and tool definitions.
func NewProvider(cfg *config.Config, exec tools.Executor) Provider {
	toolsReg := tools.NewRegistry()
	return &HTTPProvider{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
		Tools:   toolsReg.Definitions(),
	}
}
