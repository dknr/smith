package llm

import (
	"log/slog"

	"smith/config"
	"smith/tools"
	"smith/types"
)

// NewProvider creates a Provider from the given config, tool definitions, and tool registry.
// If debugLogger is non-nil, full request/response bodies are logged at debug level.
func NewProvider(cfg *config.Config, exec tools.Executor, debugLogger *slog.Logger, defs ...[]types.ToolDef) Provider {
	var toolDefs []types.ToolDef
	if len(defs) > 0 {
		toolDefs = defs[0]
	}
	return &HTTPProvider{
		BaseURL:     cfg.BaseURL,
		APIKey:      cfg.APIKey,
		Model:       cfg.Model,
		SystemPrompt: cfg.SystemPrompt,
		Tools:       toolDefs,
		DebugLogger: debugLogger,
	}
}
