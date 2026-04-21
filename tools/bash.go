package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
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

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", p.Command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("bash failed: %s%s", err, output)
	}

	// Truncate output to 4kB.
	if len(output) > maxBashOutput {
		return string(output[:maxBashOutput]) + "\n… [truncated]", nil
	}
	return string(output), nil
}
