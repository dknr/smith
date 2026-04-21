package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const maxBashOutput = 4096

func toolBash(ctx context.Context, argsJSON string) (string, error) {
	var p struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if p.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	parts := strings.Fields(p.Command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	ctx, cancel := context.WithTimeout(ctx, 1000000000) // 1 second
	defer cancel()

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("bash %s failed: %w%s", parts[0], err, string(output))
	}

	// Truncate output to 4kB.
	if len(output) > maxBashOutput {
		return string(output[:maxBashOutput]) + "\n… [truncated]", nil
	}
	return string(output), nil
}
