package server

import (
	"fmt"
	"log/slog"
	"strings"

	"smith/agent"
	"smith/types"

	"github.com/gorilla/websocket"
)

// Command defines a slash command the server can handle.
type Command struct {
	Name        string
	Description string
	Handler     func(conn *websocket.Conn, id string, a *agent.Agent, logger *slog.Logger)
}

// commands is the registry of server-handled slash commands.
var commands = []Command{
	{
		Name:        "/safe",
		Description: "Set permission mode to safe (read-only operations)",
		Handler: func(conn *websocket.Conn, id string, a *agent.Agent, logger *slog.Logger) {
			a.SetMode(types.SafeMode)
			logger.Info("mode changed", "mode", "safe")
			sendResponse(conn, types.Response{
				ID:      id,
				Role:    "assistant",
				Content: "Mode set to safe.",
				Done:    true,
				Command: "mode_change",
				Mode:    "safe",
			}, logger)
		},
	},
	{
		Name:        "/edit",
		Description: "Set permission mode to edit (file read/write operations)",
		Handler: func(conn *websocket.Conn, id string, a *agent.Agent, logger *slog.Logger) {
			a.SetMode(types.EditMode)
			logger.Info("mode changed", "mode", "edit")
			sendResponse(conn, types.Response{
				ID:      id,
				Role:    "assistant",
				Content: "Mode set to edit.",
				Done:    true,
				Command: "mode_change",
				Mode:    "edit",
			}, logger)
		},
	},
	{
		Name:        "/full",
		Description: "Set permission mode to full (all operations including shell)",
		Handler: func(conn *websocket.Conn, id string, a *agent.Agent, logger *slog.Logger) {
			a.SetMode(types.FullMode)
			logger.Info("mode changed", "mode", "full")
			sendResponse(conn, types.Response{
				ID:      id,
				Role:    "assistant",
				Content: "Mode set to full.",
				Done:    true,
				Command: "mode_change",
				Mode:    "full",
			}, logger)
		},
	},
	{
		Name:        "/mode",
		Description: "Show the current permission mode",
		Handler: func(conn *websocket.Conn, id string, a *agent.Agent, logger *slog.Logger) {
			sendResponse(conn, types.Response{
				ID:      id,
				Role:    "assistant",
				Content: fmt.Sprintf("Current mode: %s.", a.Mode()),
				Done:    true,
				Command: "mode_change",
			}, logger)
		},
	},
}

// commandNames returns the sorted list of registered command names.
func commandNames() []string {
	names := make([]string, len(commands))
	for i, c := range commands {
		names[i] = c.Name
	}
	return names
}

// commandFor looks up a command by its full name (e.g. "/safe").
func commandFor(name string) *Command {
	for i := range commands {
		if commands[i].Name == name {
			return &commands[i]
		}
	}
	return nil
}

// buildHelpText returns a formatted help string from the command registry plus
// client-side commands that the server does not handle directly.
func buildHelpText(cmds []Command) string {
	var sb strings.Builder
	sb.WriteString("Available commands:\n")

	// Server-handled commands.
	for _, c := range cmds {
		sb.WriteString(fmt.Sprintf("  %s — %s\n", c.Name, c.Description))
	}

	// Client-side commands.
	sb.WriteString("  /compact — Compact the session\n")
	sb.WriteString("  /quit — Exit the chat\n")

	return strings.TrimSpace(sb.String())
}

// handleSlashCommand dispatches a slash command using the command registry.
func handleSlashCommand(conn *websocket.Conn, id string, content string, a *agent.Agent, logger *slog.Logger) {
	// /help is handled specially to avoid an init-time cycle (buildHelpText
	// reads commands, so it can't be called from within the commands slice).
	if content == "/help" {
		sendResponse(conn, types.Response{
			ID:      id,
			Role:    "assistant",
			Content: buildHelpText(commands),
			Done:    true,
		}, logger)
		return
	}

	cmd := commandFor(content)
	if cmd == nil {
		resp := types.Response{
			ID:      id,
			Role:    "error",
			Content: fmt.Sprintf("Unknown command: %s. Type /help for available commands.", content),
			Done:    true,
		}
		sendResponse(conn, resp, logger)
		return
	}
	cmd.Handler(conn, id, a, logger)
}
