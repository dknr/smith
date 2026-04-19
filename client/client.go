package client

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

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
// Returns whether the session was new (no prior history).
func syncSession(conn *websocket.Conn, logger *slog.Logger, colorize bool) (bool, error) {
	req := types.Request{
		ID:   "0",
		Role: "user",
		Sync: true,
	}
	data, err := types.MarshalRequest(req)
	if err != nil {
		return false, fmt.Errorf("failed to marshal sync request: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return false, fmt.Errorf("failed to send sync request: %w", err)
	}
	logger.Debug("sent sync request")

	var bufferedKickoff []*types.Response
	var hasHistory bool
	var isNew bool
	for {
		_, resp, err := conn.ReadMessage()
		if err != nil {
			return false, fmt.Errorf("failed to read sync response: %w", err)
		}

		r, err := types.UnmarshalResponse(resp)
		if err != nil {
			return false, fmt.Errorf("failed to parse sync response: %w", err)
		}

		logger.Debug("sync response received", "role", r.Role, "done", r.Done, "syncComplete", r.SyncComplete)

		if r.SyncComplete {
			logger.Debug("session synced")
			if isNew && colorize {
				if r.Kickoff != "" {
					fmt.Printf("\033[90m  %s\033[0m\n", r.Kickoff)
				}
				// Replay buffered kickoff responses (incremental deltas).
				var lastContent string
				for _, br := range bufferedKickoff {
					if br.Role == "assistant" && br.Content != "" {
						if len(br.Content) > len(lastContent) {
							fmt.Print(br.Content[len(lastContent):])
						}
						lastContent = br.Content
						if br.Done {
							fmt.Println()
							if colorize && (br.Usage != nil || br.Timing != nil) {
								printStatsLine(br.Usage, br.Timing)
							}
						}
					} else {
						renderResponse(br, colorize)
					}
				}
			}
			return !hasHistory, nil
		}

		// Track whether we've seen history messages (indicates resumed session).
		if r.Role == "user" || r.Role == "tool" {
			hasHistory = true
		}

		// For resumed sessions, render all responses immediately.
		if hasHistory {
			renderResponse(r, colorize)
			continue
		}

		// For new sessions, buffer kickoff streaming responses.
		if r.Role == "new_session" {
			printNewSession()
			isNew = true
			logger.Debug("new session detected")
			continue
		}
		if r.Role == "tool_call" || r.Role == "assistant" || r.Role == "error" {
			bufferedKickoff = append(bufferedKickoff, r)
		}
	}
}

// printNewSession prints a grey "New session" line with the current timestamp.
func printNewSession() {
	fmt.Printf("\033[90m%s | New session\033[0m\n", time.Now().Format("15:04:05"))
}

// renderResponse prints a single Response using the standard format.
// Used by both syncSession and readLoop for consistent display.
func renderResponse(r *types.Response, colorize bool) {
	if r.Role == "tool_call" {
		if colorize {
			fmt.Printf("\033[33m%s\033[0m\n", r.Content)
		}
		return
	}
	if r.Role == "error" {
		if colorize {
			fmt.Printf("\033[31mError: %s\033[0m\n", r.Content)
		} else {
			fmt.Fprintf(os.Stderr, "error: %s\n", r.Content)
		}
		fmt.Println()
		return
	}
	if r.Role == "user" {
		fmt.Printf("> %s\n", r.Content)
		return
	}
	if r.Role == "tool" {
		fmt.Println(r.Content)
		return
	}
	// assistant: batch print (full content, no delta).
	if r.Content != "" {
		fmt.Println(r.Content)
	}
	if r.Done && colorize && (r.Usage != nil || r.Timing != nil) {
		printStatsLine(r.Usage, r.Timing)
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

		logger.Debug("received message", "role", r.Role, "done", r.Done, "id", r.ID)

		// Delegate non-assistant roles to renderResponse.
		if r.Role != "assistant" {
			renderResponse(r, colorize)
			continue
		}

		// Assistant: delta printing.
		if len(r.Content) > len(lastContent) {
			fmt.Print(r.Content[len(lastContent):])
		}
		lastContent = r.Content

		if r.Done {
			fmt.Println()
			if colorize && (r.Usage != nil || r.Timing != nil) {
				printStatsLine(r.Usage, r.Timing)
			}
			return nil
		}
	}
}

// Send connects to the server, sends a message, prints all responses until done, then exits.
// If colorize is true, tool calls are shown in yellow, errors in red, and stats are printed.
func Send(addr, message string, logger *slog.Logger, colorize bool) error {
	conn, err := dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	logger.Debug("connected to server", "addr", addr)

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
	logger.Debug("sent message", "id", req.ID, "content", req.Content)

	return readLoop(conn, logger, colorize)
}

// Chat starts an interactive session with the server.
// Type messages to send, /quit to exit.
func Chat(addr string, logger *slog.Logger) error {
	conn, err := dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	logger.Debug("connected to server", "addr", addr)

	if _, err := syncSession(conn, logger, true); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	msgID := 0

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			fmt.Println()
			logger.Debug("input stream ended, exiting")
			break
		}
		input := scanner.Text()
		if strings.TrimSpace(input) == "/quit" {
			logger.Debug("quit command received, exiting")
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
			logger.Error("failed to send message", "error", err)
			break
		}
		logger.Debug("sent message", "id", req.ID, "content", req.Content)

		if err := readLoop(conn, logger, true); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			break
		}
	}

	return nil
}

// printStatsLine prints the grey timestamp + token stats line, matching bantam's format.
func printStatsLine(usage *types.ResponseUsage, timing *types.ResponseTiming) {
	var inputTokens, outputTokens, totalTokens int
	if usage != nil {
		inputTokens = usage.PromptTokens
		outputTokens = usage.CompletionTokens
		totalTokens = usage.TotalTokens
	}

	fmt.Printf("\033[90m%s | ", time.Now().Format("15:04:05"))
	if timing != nil && timing.PromptPerSecond > 0 && timing.PredictedPerSecond > 0 {
		fmt.Printf("%d (%.1f/s) => %d (%.1f/s) => %d (%.1fs)",
			inputTokens, timing.PromptPerSecond,
			outputTokens, timing.PredictedPerSecond,
			totalTokens, (timing.PromptMs+timing.PredictedMs)/1000)
	} else if timing != nil {
		fmt.Printf("%d => %d => %d tokens (%.1fs)",
			inputTokens, outputTokens, totalTokens,
			(timing.PromptMs+timing.PredictedMs)/1000)
	} else {
		fmt.Printf("%d => %d => %d tokens", inputTokens, outputTokens, totalTokens)
	}
	fmt.Println("\033[0m")
}
