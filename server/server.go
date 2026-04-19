package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
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

var (
	activeConn    *websocket.Conn
	activeConnMu  sync.Mutex
)

// Serve starts a WebSocket server on the given address that processes messages
// through an LLM agent and sends responses back to the client.
// It shuts down gracefully on SIGINT or SIGTERM.
func Serve(addr string, cfg *config.Config, debugLogger *slog.Logger, sess *session.Session, memStore *memory.Store, logger *slog.Logger) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Single-session: reject if a connection is already active.
		activeConnMu.Lock()
		if activeConn != nil {
			activeConnMu.Unlock()
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			activeConnMu.Unlock()
			logger.Error("websocket upgrade failed", "error", err)
			return
		}
		activeConn = conn
		activeConnMu.Unlock()

		logger.Info("client connected", "remote", conn.RemoteAddr().String())

		agentLogger := slog.New(logger.Handler()).With("component", "agent")

		executor := tools.NewRegistry()

		soulTool := tools.NewSoulTool(memStore)
		memoryTool := tools.NewMemoryTool(memStore)
		executor.RegisterFn("soul", soulTool.Execute, tools.SoulToolDef)
		executor.RegisterFn("memory", memoryTool.Execute, tools.MemoryToolDef)

		provider := llm.NewProvider(cfg, executor, debugLogger, executor.Definitions())
		a := agent.New(provider, executor, sess, agentLogger, memStore)

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

			if req.Sync {
				syncSession(conn, a, req.ID, logger, agentLogger, memStore, cfg.Kickoff)
				continue
			}

			logger.Info("message", "id", req.ID, "content", req.Content)

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

		// Release the session lock on disconnect.
		activeConnMu.Lock()
		activeConn = nil
		activeConnMu.Unlock()
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

func syncSession(conn *websocket.Conn, a *agent.Agent, id string, logger, agentLogger *slog.Logger, memStore *memory.Store, kickoff string) {
	history := a.History()

	// If new session with kickoff, process it through the agent.
	if len(history) == 0 && kickoff != "" {
		agentLogger.Info("sync", "kickoff", kickoff)
		// Send banner immediately so the client doesn't stall.
		banner := types.Response{
			ID:   id,
			Role: "new_session",
			Done: true,
		}
		data, err := types.MarshalResponse(banner)
		if err != nil {
			logger.Error("failed to marshal new session banner", "error", err)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			logger.Error("failed to write new session banner", "error", err)
			return
		}

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
		data, err = types.MarshalResponse(resp)
		if err != nil {
			logger.Error("failed to marshal sync complete", "error", err)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			logger.Error("failed to write sync complete", "error", err)
			return
		}
		agentLogger.Info("synced with kickoff")
		return
	}

	// Convert history messages to Response objects and stream them.
	for _, m := range history {
		if len(m.ToolCalls) > 0 {
			// Tool call from assistant — emit one Response per tool call.
			for _, tc := range m.ToolCalls {
				resp := types.Response{
					ID:      id,
					Role:    "tool_call",
					Content: types.FormatToolCall(tc.Name, tc.Arguments),
					Done:    false,
				}
				data, err := types.MarshalResponse(resp)
				if err != nil {
					logger.Error("failed to marshal tool call response", "error", err)
					return
				}
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					logger.Error("failed to write tool call response", "error", err)
					return
				}
			}
			continue
		}
		// Tool responses are internal — not sent to the client in the live path.
		if m.Role == "tool" {
			continue
		}
		// Regular message (user, assistant text, error).
		resp := types.Response{
			ID:      id,
			Role:    m.Role,
			Content: m.Content,
			Done:    false,
		}
		data, err := types.MarshalResponse(resp)
		if err != nil {
			logger.Error("failed to marshal history response", "error", err)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			logger.Error("failed to write history response", "error", err)
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

	agentLogger.Info("synced", "messages", len(history))
}
