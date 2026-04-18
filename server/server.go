package server

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Serve starts a WebSocket server on the given address that echoes all received messages back to the sender.
func Serve(addr string, logger *slog.Logger) error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("websocket upgrade failed", "error", err)
			return
		}
		logger.Info("client connected", "remote", conn.RemoteAddr().String())

		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				logger.Info("client disconnected", "remote", conn.RemoteAddr().String())
				break
			}

			logger.Debug("received message", "data", string(msg))

			if err := conn.WriteMessage(msgType, msg); err != nil {
				logger.Error("failed to write message", "error", err)
				break
			}
		}

		conn.Close()
	})

	logger.Info("starting websocket server", "addr", addr)
	return http.ListenAndServe(addr, nil)
}
