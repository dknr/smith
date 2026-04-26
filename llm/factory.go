package llm

import (
	"log/slog"

	"smith/config"
	"smith/types"
)

// NewProvider creates a Provider from the given config and tool definitions.
// If debugLogger is non-nil, full request/response bodies are logged at debug level.
// If turnLogger is non-nil, request/response bodies are also written to turn log files.
func NewProvider(cfg *config.Config, debugLogger *slog.Logger, turnLogger *TurnLogger, defs ...[]types.ToolDef) Provider {
	// Set default reasoning effort to "low" if not provided
	reasoningEffort := cfg.ReasoningEffort
	if reasoningEffort == "" {
		reasoningEffort = "low"
	}
	// Set default provider type to "llamacpp" if not provided
	providerType := cfg.ProviderType
	if providerType == "" {
		providerType = "llamacpp"
	}
	return &HTTPProvider{
		BaseURL:      cfg.BaseURL,
		APIKey:       cfg.APIKey,
		Model:        cfg.Model,
		SystemPrompt: cfg.SystemPrompt,
		DebugLogger:  debugLogger,
		TurnLogger:   turnLogger,
		ProviderType: providerType,
		ReasoningEffort: reasoningEffort,
	}
}
