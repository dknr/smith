package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"smith/memory"
	"smith/types"
)

// SoulTool provides read and modify operations for the agent's soul text.
type SoulTool struct {
	store *memory.Store
}

// NewSoulTool creates a SoulTool backed by the given store.
func NewSoulTool(store *memory.Store) *SoulTool {
	return &SoulTool{store: store}
}

// ToolDef returns the tool definition for the LLM.
var SoulToolDef = types.ToolDef{
	Name:        "soul",
	Description: "Read or modify the agent's soul (plain text identity file). Actions: read (returns current soul text), modify (accepts a 'text' field to replace the entire soul).",
	Parameters: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action to perform: 'read' or 'modify'",
			},
			"text": map[string]interface{}{
				"type":        "string",
				"description": "New soul text (required for 'modify' action)",
			},
		},
		"required": []string{"action"},
	},
}

// Execute handles soul tool actions.
func (t *SoulTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Action string `json:"action"`
		Text   string `json:"text"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	switch args.Action {
	case "read":
		text, err := t.store.GetSoul()
		if err != nil {
			return "", fmt.Errorf("failed to read soul: %w", err)
		}
		return text, nil

	case "modify":
		if args.Text == "" {
			return "", fmt.Errorf("text is required for modify action")
		}
		if err := t.store.SetSoul(args.Text); err != nil {
			return "", fmt.Errorf("failed to set soul: %w", err)
		}
		return fmt.Sprintf("Soul updated (%d bytes).", len(args.Text)), nil

	default:
		return "", fmt.Errorf("unknown action: %s (use 'read' or 'modify')", args.Action)
	}
}
