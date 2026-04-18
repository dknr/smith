package server

import (
	"context"
	"log/slog"
	"net/http"

	"smith/agent"
	"smith/types"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// EchoHandler is a placeholder handler that echoes messages back.
type EchoHandler struct{}

func (h *EchoHandler) Handle(ctx context.Context, messages []agent.Message) (<-chan types.Response, error) {
	ch := make(chan types.Response, 1)
	go func() {
		defer close(ch)
		// Find the last user message
		var lastUserMsg string
		for _, m := range messages {
			if m.Role == "user" {
				lastUserMsg = m.Content
			}
		}
		ch <- types.Response{
			Content: lastUserMsg,
			Done:    true,
		}
	}()
	return ch, nil
}

// Serve starts a WebSocket server on the given address that processes messages
// through an agent and streams responses back to the client.
func Serve(addr string, logger *slog.Logger) error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("websocket upgrade failed", "error", err)
			return
		}
		logger.Info("client connected", "remote", conn.RemoteAddr().String())

		agent := agent.New(&EchoHandler{})

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				logger.Info("client disconnected", "remote", conn.RemoteAddr().String())
				break
			}

			req, err := types.UnmarshalRequest(msg)
			if err != nil {
				logger.Error("failed to parse request", "error", err)
				continue
			}

			logger.Debug("received message", "id", req.ID, "content", req.Content)

			respCh, err := agent.ProcessMessage(r.Context(), req.Content)
			if err != nil {
				logger.Error("agent error", "error", err)
				continue
			}

			for resp := range respCh {
				data, err := types.MarshalResponse(resp)
				if err != nil {
					logger.Error("failed to marshal response", "error", err)
					break
				}
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					logger.Error("failed to write response", "error", err)
					break
				}
			}
		}

		conn.Close()
	})

	logger.Info("starting websocket server", "addr", addr)
	return http.ListenAndServe(addr, nil)
}
