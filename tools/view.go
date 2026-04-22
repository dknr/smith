package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

const maxViewOutput = 40960

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
