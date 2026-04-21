package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	// Shared across all connections.
	executor := tools.NewRegistry()

	soulTool := tools.NewSoulTool(memStore)
	memoryTool := tools.NewMemoryTool(memStore)
	executor.RegisterFn("soul", soulTool.Execute, tools.SoulToolDef)
	executor.RegisterFn("memory", memoryTool.Execute, tools.MemoryToolDef)

	provider := llm.NewProvider(cfg, executor, debugLogger, executor.Definitions())
	a := agent.New(provider, executor, sess, logger, memStore)

	// Run kickoff autonomously when session is empty and kickoff is configured.
	if cfg.Kickoff != "" && len(a.History()) == 0 {
		logger.Info("autonomous kickoff", "message", cfg.Kickoff)
		respCh, err := a.ProcessMessage(context.Background(), cfg.Kickoff)
		if err != nil {
			logger.Error("autonomous kickoff error", "error", err)
		} else {
			for resp := range respCh {
				if resp.Done {
					logger.Info("kickoff response", "role", resp.Role, "content", resp.Content)
				}
			}
		}
		logger.Info("autonomous kickoff complete")
	}

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

		agentLogger := slog.New(logger.Handler()).With("component", "agent", "conn", conn.RemoteAddr().String())

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

			if req.Reset {
				logger.Info("session reset requested")
				respCh, err := a.Reset(r.Context(), cfg.Kickoff)
				if err != nil {
					logger.Error("reset error", "error", err)
					continue
				}
				for resp := range respCh {
					resp.ID = req.ID
					resp.Reset = true

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
				continue
			}

			if req.Mode != "" {
				handleModeCommand(conn, req.ID, req.Mode, a, logger)
				continue
			}

			if strings.HasPrefix(req.Content, "/") {
				handleSlashCommand(conn, req.ID, req.Content, a, logger)
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

func handleSlashCommand(conn *websocket.Conn, id string, content string, a *agent.Agent, logger *slog.Logger) {
	switch content {
	case "/safe":
		a.SetMode(types.SafeMode)
		logger.Info("mode changed", "mode", "safe")
		resp := types.Response{
			ID:      id,
			Role:    "assistant",
			Content: "Mode set to safe.",
			Done:    true,
			Command: "mode_change",
		}
		sendResponse(conn, resp, logger)
	case "/edit":
		a.SetMode(types.EditMode)
		logger.Info("mode changed", "mode", "edit")
		resp := types.Response{
			ID:      id,
			Role:    "assistant",
			Content: "Mode set to edit.",
			Done:    true,
			Command: "mode_change",
		}
		sendResponse(conn, resp, logger)
	case "/full":
		a.SetMode(types.FullMode)
		logger.Info("mode changed", "mode", "full")
		resp := types.Response{
			ID:      id,
			Role:    "assistant",
			Content: "Mode set to full.",
			Done:    true,
			Command: "mode_change",
		}
		sendResponse(conn, resp, logger)
	case "/mode":
		resp := types.Response{
			ID:      id,
			Role:    "assistant",
			Content: fmt.Sprintf("Current mode: %s.", a.Mode()),
			Done:    true,
			Command: "mode_change",
		}
		sendResponse(conn, resp, logger)
	case "/help":
		resp := types.Response{
			ID:      id,
			Role:    "assistant",
			Content: "Available commands: /safe, /edit, /full, /mode, /help, /reset, /clear, /quit",
			Done:    true,
			Command: "mode_change",
		}
		sendResponse(conn, resp, logger)
	default:
		resp := types.Response{
			ID:      id,
			Role:    "error",
			Content: fmt.Sprintf("Unknown command: %s. Type /help for available commands.", content),
			Done:    true,
		}
		sendResponse(conn, resp, logger)
	}
}

func sendResponse(conn *websocket.Conn, resp types.Response, logger *slog.Logger) {
	data, err := types.MarshalResponse(resp)
	if err != nil {
		logger.Error("failed to marshal response", "error", err)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		logger.Error("failed to write response", "error", err)
	}
}

func handleModeCommand(conn *websocket.Conn, id string, mode types.Mode, a *agent.Agent, logger *slog.Logger) {
	modeNames := map[types.Mode]string{
		types.SafeMode:  "safe",
		types.EditMode:  "edit",
		types.FullMode:  "full",
	}
	switch mode {
	case types.SafeMode, types.EditMode, types.FullMode:
		a.SetMode(mode)
		logger.Info("mode changed", "mode", modeNames[mode])
		resp := types.Response{
			ID:      id,
			Role:    "assistant",
			Content: fmt.Sprintf("Mode set to %s.", modeNames[mode]),
			Done:    true,
			Command: "mode_change",
		}
		sendResponse(conn, resp, logger)
	default:
		resp := types.Response{
			ID:      id,
			Role:    "assistant",
			Content: fmt.Sprintf("Current mode: %s.", a.Mode()),
			Done:    true,
			Command: "mode_change",
		}
		sendResponse(conn, resp, logger)
	}
}

func syncSession(conn *websocket.Conn, a *agent.Agent, id string, logger, agentLogger *slog.Logger, memStore *memory.Store, kickoff string) {
	history := a.History()

	// Stream history messages.
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
