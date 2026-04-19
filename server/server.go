package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"smith/agent"
	"smith/llm"
	"smith/session"
	"smith/tools"
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

// Serve starts a WebSocket server on the given address that processes messages
// through an LLM agent and sends responses back to the client.
// It shuts down gracefully on SIGINT or SIGTERM.
func Serve(addr string, provider llm.Provider, executor *tools.Registry, sess *session.Session, logger *slog.Logger) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("websocket upgrade failed", "error", err)
			return
		}
		logger.Info("client connected", "remote", conn.RemoteAddr().String())

		agent := agent.New(provider, executor, sess, logger)

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

			if req.Sync {
				syncSession(conn, agent, req.ID, logger)
				continue
			}

			respCh, err := agent.ProcessMessage(r.Context(), req.Content)
			if err != nil {
				logger.Error("agent error", "error", err)
				continue
			}

			for resp := range respCh {
				resp.ID = req.ID

				data, err := types.MarshalResponse(*resp)
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

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Listen for shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("server shutdown error", "error", err)
		}
	}()

	logger.Info("starting websocket server", "addr", addr)
	return srv.ListenAndServe()
}

func syncSession(conn *websocket.Conn, a *agent.Agent, id string, logger *slog.Logger) {
	history := a.History()

	// Send history messages.
	if len(history) > 0 {
		resp := types.Response{
			ID:      id,
			Role:    "sync",
			Content: "",
			Done:    false,
			History: history,
		}
		data, err := types.MarshalResponse(resp)
		if err != nil {
			logger.Error("failed to marshal history", "error", err)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			logger.Error("failed to write history", "error", err)
			return
		}
	}

	// Send sync complete.
	resp := types.Response{
		ID:           id,
		Role:         "sync",
		Content:      "",
		Done:         true,
		SyncComplete: true,
	}
	data, err := types.MarshalResponse(resp)
	if err != nil {
		logger.Error("failed to marshal sync complete", "error", err)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		logger.Error("failed to write sync complete", "error", err)
		return
	}

	logger.Debug("session synced", "messages", len(history))
}
