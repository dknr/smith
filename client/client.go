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

// readLoop reads streaming responses from the server, printing deltas as they arrive.
// Returns on error or when done=true is received.
func readLoop(conn *websocket.Conn, logger *slog.Logger) error {
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
			fmt.Println(r.Content)
			continue
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

	return readLoop(conn, logger)
}

// Chat starts an interactive session with the server.
// Type messages to send, /quit to exit.
func Chat(addr string, logger *slog.Logger) error {
	conn, err := dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

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

		if err := readLoop(conn, logger); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			break
		}
	}

	return nil
}
