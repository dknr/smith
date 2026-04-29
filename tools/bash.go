package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"smith/types"
)

const maxBashOutput = 16384

// BashToolDef is the tool definition for "bash".
var BashToolDef = types.ToolDef{
	Name:        "bash",
	Description: "Execute any shell command. WARNING: This tool runs commands with full system access. Be cautious with destructive operations (rm, chmod, dd, etc.). Commands are executed in FullMode only. Timeout: 30 seconds.",
	Parameters: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Shell command to execute",
			},
		},
		"required": []string{"command"},
	},
}

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

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", p.Command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			banner := fmt.Sprintf("[Process returned non-zero exit code: %d]\n", exitErr.ExitCode())
			return banner + string(output), nil
		}
		return "", fmt.Errorf("bash failed: %s%s", err, output)
	}

	// Truncate output to 4kB.
	if len(output) > maxBashOutput {
		return string(output[:maxBashOutput]) + "\n… [truncated]", nil
	}
	return string(output), nil
}
