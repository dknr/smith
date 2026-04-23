package tools

import (
	"context"
	"time"

	"smith/types"
)

// TimeToolDef is the tool definition for "time".
var TimeToolDef = types.ToolDef{
	Name:        "time",
	Description: "Return the current date and time in ISO 8601 format.",
	Parameters: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Time format string (default: RFC3339)",
			},
		},
		"required": []string{},
	},
}

func toolTime(ctx context.Context, argsJSON string) (string, error) {
	// Always return RFC3339 (ISO 8601). Args are accepted but ignored for now.
	_ = argsJSON
	return time.Now().UTC().Format(time.RFC3339), nil
}
