package client

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"smith/types"

	"github.com/gorilla/websocket"
)

// dial connects to a WebSocket server, adding the ws:// prefix if needed.
func dial(addr string) (*websocket.Conn, error) {
	if !strings.HasPrefix(addr, "ws://") && !strings.HasPrefix(addr, "wss://") {
		addr = "ws://" + addr
	}
	conn, _, err := websocket.DefaultDialer.Dial(addr, nil)
	return conn, err
}

// syncSession connects to the server and requests a session sync.
// It prints history messages and verifies sync complete.
// If colorize is true, tool calls are shown in yellow and errors in red.
func syncSession(conn *websocket.Conn, logger *slog.Logger, colorize bool) error {
	req := types.Request{
		ID:   "0",
		Role: "user",
		Sync: true,
	}
	data, err := types.MarshalRequest(req)
	if err != nil {
		return fmt.Errorf("failed to marshal sync request: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to send sync request: %w", err)
	}

	for {
		_, resp, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("failed to read sync response: %w", err)
		}

		r, err := types.UnmarshalResponse(resp)
		if err != nil {
			return fmt.Errorf("failed to parse sync response: %w", err)
		}

		if r.SyncComplete {
			logger.Debug("session synced")
			return nil
		}

		// Print history messages.
		for _, m := range r.History {
			if len(m.ToolCalls) > 0 {
				// Tool call messages are stored with Role=assistant but ToolCalls populated.
				for _, tc := range m.ToolCalls {
					if colorize {
						fmt.Printf("\033[33m%s(%s)\033[0m\n", tc.Name, tc.Arguments)
					}
				}
				continue
			}
			switch m.Role {
			case "user":
				fmt.Printf("> %s\n", m.Content)
			case "tool_call":
				if colorize {
					fmt.Printf("\033[33m%s\033[0m\n", m.Content)
				}
			case "tool":
				fmt.Println(m.Content)
			case "assistant":
				fmt.Println(m.Content)
			case "error":
				if colorize {
					fmt.Printf("\033[31mError: %s\033[0m\n", m.Content)
				} else {
					fmt.Fprintf(os.Stderr, "error: %s\n", m.Content)
				}
			}
		}
	}
}

// readLoop reads streaming responses from the server, printing deltas as they arrive.
// If colorize is true, tool calls are shown in yellow and errors in red.
// Returns on error or when done=true is received.
func readLoop(conn *websocket.Conn, logger *slog.Logger, colorize bool) error {
	var lastContent string
	for {
		_, resp, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		r, err := types.UnmarshalResponse(resp)
		if err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		// Tool call notifications are printed as-is, not as deltas.
		if r.Role == "tool_call" {
			if colorize {
				fmt.Printf("\033[33m%s\033[0m\n", r.Content)
			}
			continue
		}

		if r.Role == "error" {
			if colorize {
				fmt.Printf("\033[31mError: %s\033[0m\n", r.Content)
			} else {
				fmt.Fprintf(os.Stderr, "error: %s\n", r.Content)
			}
			fmt.Println()
			return nil
		}

		if len(r.Content) > len(lastContent) {
			fmt.Print(r.Content[len(lastContent):])
		}
		lastContent = r.Content
		logger.Debug("received response", "id", r.ID, "done", r.Done, "data", r.Content)

		if r.Done {
			fmt.Println()
			return nil
		}
	}
}

// Send connects to the server, sends a message, prints all responses until done, then exits.
func Send(addr, message string, logger *slog.Logger) error {
	conn, err := dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	req := types.Request{
		ID:      "1",
		Role:    "user",
		Content: message,
	}
	data, err := types.MarshalRequest(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return readLoop(conn, logger, false)
}

// Chat starts an interactive session with the server.
// Type messages to send, /quit to exit.
func Chat(addr string, logger *slog.Logger) error {
	conn, err := dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	if err := syncSession(conn, logger, true); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	msgID := 0

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			fmt.Println()
			break
		}
		input := scanner.Text()
		if strings.TrimSpace(input) == "/quit" {
			break
		}
		if strings.TrimSpace(input) == "" {
			continue
		}

		msgID++
		req := types.Request{
			ID:      fmt.Sprintf("%d", msgID),
			Role:    "user",
			Content: input,
		}
		data, err := types.MarshalRequest(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			break
		}

		if err := readLoop(conn, logger, true); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			break
		}
	}

	return nil
}
