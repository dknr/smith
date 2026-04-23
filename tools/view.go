package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"smith/types"
)

const maxViewOutput = 40960

// ViewToolDef is the tool definition for "view".
var ViewToolDef = types.ToolDef{
	Name:        "view",
	Description: "Read the contents of a file. Output is truncated to 4kB with [truncated] marker if exceeded.",
	Parameters: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to read",
			},
		},
		"required": []string{"path"},
	},
}

func toolView(ctx context.Context, argsJSON string) (string, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if p.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	data, err := os.ReadFile(p.Path)
	if err != nil {
		return "", err
	}

	if len(data) > maxViewOutput {
		return string(data[:maxViewOutput]) + "\n… [truncated]", nil
	}
	return string(data), nil
}
