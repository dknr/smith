package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"smith/types"
)

// GitToolDef is the tool definition for "git".
var GitToolDef = types.ToolDef{
	Name:        "git",
	Description: "Execute non-destructive git subcommands (e.g. status, diff, log, show, branch, tag, ls-files, blame, grep, remote, rev-parse, describe, for-each-ref, reflog, fsck, count-objects, shortlog).",
	Parameters: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Git subcommand to execute (e.g. 'status', 'log --oneline', 'diff --stat'). Only non-destructive commands are allowed.",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the git repository (equivalent to git -C <path>). Optional.",
			},
		},
		"required": []string{"command"},
	},
}

// allowedGitCommands is the whitelist of non-destructive git subcommands.
var allowedGitCommands = map[string]bool{
	"status":         true,
	"diff":           true,
	"log":            true,
	"show":           true,
	"branch":         true,
	"tag":            true,
	"ls-files":       true,
	"blame":          true,
	"grep":           true,
	"remote":         true,
	"rev-parse":      true,
	"describe":       true,
	"for-each-ref":   true,
	"reflog":         true,
	"fsck":           true,
	"count-objects":  true,
	"shortlog":       true,
	"cat-file":       true,
	"verify-commit":  true,
	"verify-tag":     true,
	"archive":        true,
	"daemon":         true,
	"exec-backend":   true,
	"help":           true,
	"instaweb":       true,
	"merge-base":     true,
	"rerere":         true,
	"stripspace":     true,
	"svn":            true,
	"web":            true,
}

func toolGit(ctx context.Context, argsJSON string) (string, error) {
	var p struct {
		Command string `json:"command"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if p.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	// Parse the command to extract the subcommand.
	parts := strings.Fields(p.Command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}
	subcmd := parts[0]

	// Allow "git" as the subcommand (users may include it).
	if subcmd == "git" {
		if len(parts) < 2 {
			return "", fmt.Errorf("git subcommand is required")
		}
		subcmd = parts[1]
	}

	if !allowedGitCommands[subcmd] {
		return "", fmt.Errorf("git subcommand %q is not allowed (only non-destructive commands)", subcmd)
	}

	// Build the git command with optional -C path flag.
	var gitArgs []string
	if p.Path != "" {
		gitArgs = append(gitArgs, "-C", p.Path)
	}
	gitArgs = append(gitArgs, parts...)

	cmd := exec.CommandContext(ctx, "git", gitArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w%s", subcmd, err, string(output))
	}
	return string(output), nil
}
