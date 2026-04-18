package client

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/gorilla/websocket"
)

// ConnectAndSend connects to a WebSocket server at the given address, sends the message,
// prints the response, and exits.
func ConnectAndSend(addr, message string, logger *slog.Logger) error {
	if !strings.HasPrefix(addr, "ws://") && !strings.HasPrefix(addr, "wss://") {
		addr = "ws://" + addr
	}
	conn, _, err := websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Println(string(msg))
	logger.Debug("received response", "data", string(msg))

	return nil
}
