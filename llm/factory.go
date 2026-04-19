package llm

import (
	"log/slog"

	"smith/config"
	"smith/tools"
)

// NewProvider creates a Provider from the given config and tool definitions.
// If protocolLogger is non-nil, full request/response bodies are logged to it.
func NewProvider(cfg *config.Config, exec tools.Executor, protocolLogger *slog.Logger) Provider {
	toolsReg := tools.NewRegistry()
	return &HTTPProvider{
		BaseURL:        cfg.BaseURL,
		APIKey:         cfg.APIKey,
		Model:          cfg.Model,
		SystemPrompt:   cfg.SystemPrompt,
		Tools:          toolsReg.Definitions(),
		ProtocolLogger: protocolLogger,
	}
}
