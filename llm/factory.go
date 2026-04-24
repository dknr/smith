package llm

import (
	"log/slog"

	"smith/config"
	"smith/tools"
	"smith/types"
)

// NewProvider creates a Provider from the given config, tool definitions, and tool registry.
// If debugLogger is non-nil, full request/response bodies are logged at debug level.
// If turnLogger is non-nil, request/response bodies are also written to turn log files.
func NewProvider(cfg *config.Config, exec tools.Executor, debugLogger *slog.Logger, turnLogger *TurnLogger, defs ...[]types.ToolDef) Provider {
	var toolDefs []types.ToolDef
	if len(defs) > 0 {
		toolDefs = defs[0]
	}
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
		Tools:        toolDefs,
		DebugLogger:  debugLogger,
		TurnLogger:   turnLogger,
		ProviderType: providerType,
		ReasoningEffort: reasoningEffort,
	}
}
