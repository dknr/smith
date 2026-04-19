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
	"smith/config"
	"smith/llm"
	"smith/memory"
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
func Serve(addr string, cfg *config.Config, protocolLogger *slog.Logger, sess *session.Session, logger *slog.Logger) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("websocket upgrade failed", "error", err)
			return
		}
		logger.Info("client connected", "remote", conn.RemoteAddr().String())

		executor := tools.NewRegistry()

		memStore, err := memory.New()
		if err != nil {
			logger.Error("memory store error", "error", err)
			return
		}
		defer memStore.Close()

		soulTool := tools.NewSoulTool(memStore)
		memoryTool := tools.NewMemoryTool(memStore)
		executor.RegisterFn("soul", soulTool.Execute, tools.SoulToolDef)
		executor.RegisterFn("memory", memoryTool.Execute, tools.MemoryToolDef)

		provider := llm.NewProvider(cfg, executor, protocolLogger, executor.Definitions())
		a := agent.New(provider, executor, sess, logger, memStore)

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
				syncSession(conn, a, req.ID, logger, memStore, cfg.Kickoff)
				continue
			}

			respCh, err := a.ProcessMessage(r.Context(), req.Content)
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

func syncSession(conn *websocket.Conn, a *agent.Agent, id string, logger *slog.Logger, memStore *memory.Store, kickoff string) {
	history := a.History()

	// If new session with kickoff, process it through the agent.
	if len(history) == 0 && kickoff != "" {
		respCh, err := a.ProcessMessage(context.Background(), kickoff)
		if err != nil {
			logger.Error("kickoff error", "error", err)
			return
		}

		// Stream all responses from the kickoff.
		for resp := range respCh {
			resp.ID = id
			data, err := types.MarshalResponse(*resp)
			if err != nil {
				logger.Error("failed to marshal kickoff response", "error", err)
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				logger.Error("failed to write kickoff response", "error", err)
				return
			}
		}

		// Send sync complete with kickoff text so client can display it.
		resp := types.Response{
			ID:           id,
			Role:         "sync",
			Content:      "",
			Done:         true,
			SyncComplete: true,
			Kickoff:      kickoff,
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
		logger.Debug("session synced with kickoff")
		return
	}

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
