package client

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"smith/types"

	"github.com/chzyer/readline"
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
// Returns whether the session was new (no prior history) and the current mode.
func syncSession(conn *websocket.Conn, logger *slog.Logger, w io.Writer, colorize bool) (bool, string, error) {
	req := types.Request{
		ID:   "0",
		Role: "user",
		Sync: true,
	}
	data, err := types.MarshalRequest(req)
	if err != nil {
		return false, "", fmt.Errorf("failed to marshal sync request: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return false, "", fmt.Errorf("failed to send sync request: %w", err)
	}
	logger.Debug("sent sync request")

	var bufferedKickoff []*types.Response
	var hasHistory bool
	var isNew bool
	for {
		_, resp, err := conn.ReadMessage()
		if err != nil {
			return false, "", fmt.Errorf("failed to read sync response: %w", err)
		}

		r, err := types.UnmarshalResponse(resp)
		if err != nil {
			return false, "", fmt.Errorf("failed to parse sync response: %w", err)
		}

		logger.Debug("sync response received", "role", r.Role, "done", r.Done, "syncComplete", r.SyncComplete)

		if r.SyncComplete {
			logger.Debug("session synced")
			mode := r.Mode
			if isNew && colorize {
				if r.Kickoff != "" {
					fmt.Fprintf(w, "\033[90m  %s\033[0m\n", r.Kickoff)
				}
				// Replay buffered kickoff responses (incremental deltas).
				var lastContent string
				for _, br := range bufferedKickoff {
					if br.Role == "assistant" && br.Content != "" {
						if len(br.Content) > len(lastContent) {
							fmt.Fprint(w, br.Content[len(lastContent):])
						}
						lastContent = br.Content
						if br.Done {
							fmt.Fprintln(w)
							if colorize && (br.Usage != nil || br.Timing != nil) {
								printStatsLine(w, br.Usage, br.Timing)
							}
						}
					} else {
						renderResponse(w, br, colorize)
					}
				}
			}
			return !hasHistory, mode, nil
		}

		// Track whether we've seen history messages (indicates resumed session).
		if r.Role == "user" || r.Role == "tool" {
			hasHistory = true
		}

		// For resumed sessions, render all responses immediately.
		if hasHistory {
			renderResponse(w, r, colorize)
			continue
		}

		// For new sessions, buffer kickoff streaming responses.
		if r.Role == "new_session" {
			printNewSession(w)
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
func printNewSession(w io.Writer) {
	fmt.Fprintf(w, "\033[90m%s | New session\033[0m\n", time.Now().Format("15:04:05"))
}

// renderResponse prints a single Response using the standard format.
// Used by both syncSession and readLoop for consistent display.
// Returns the mode if it was updated, or empty string.
func renderResponse(w io.Writer, r *types.Response, colorize bool) string {
	if r.Command == "mode_change" {
		if colorize {
			fmt.Fprintf(w, "\033[90m%s\033[0m\n", r.Content)
		} else {
			fmt.Fprintln(w, r.Content)
		}
		return r.Mode
	}
	if r.Role == "tool_call" {
		if colorize {
			fmt.Fprintf(w, "\033[2;34m%s\033[0m\n", r.Content)
		}
		return ""
	}
	if r.Role == "error" {
		if colorize {
			fmt.Fprintf(w, "\033[31mError: %s\033[0m\n", r.Content)
		} else {
			fmt.Fprintf(w, "error: %s\n", r.Content)
		}
		fmt.Fprintln(w)
		return ""
	}
	if r.Role == "user" {
		fmt.Fprintf(w, "> %s\n", r.Content)
		return ""
	}
	if r.Role == "tool" {
		if colorize {
			fmt.Fprintf(w, "\033[2;36m%s\033[0m\n", r.Content)
		} else {
			fmt.Fprintln(w, r.Content)
		}
		return ""
	}
	// assistant: batch print (full content, no delta).
	if r.Content != "" {
		fmt.Fprintln(w, r.Content)
	}
	if r.Done && colorize && (r.Usage != nil || r.Timing != nil) {
		printStatsLine(w, r.Usage, r.Timing)
	}
	return ""
}

// readLoop reads streaming responses from the server, printing deltas as they arrive.
// If colorize is true, tool calls are shown in yellow and errors in red.
// Returns on error or when done=true is received.
func readLoop(conn *websocket.Conn, logger *slog.Logger, w io.Writer, colorize bool) error {
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
			renderResponse(w, r, colorize)
			continue
		}

		// Assistant: delta printing.
		if len(r.Content) > len(lastContent) {
			fmt.Fprint(w, r.Content[len(lastContent):])
		}
		lastContent = r.Content

		if r.Done {
			fmt.Fprintln(w)
			if colorize && (r.Usage != nil || r.Timing != nil) {
				printStatsLine(w, r.Usage, r.Timing)
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

	return readLoop(conn, logger, io.Discard, colorize)
}

// Chat starts an interactive session with the server.
// Type messages to send, /quit to exit.
func Chat(addr string, logger *slog.Logger, term *Terminal) error {
	conn, err := dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	logger.Debug("connected to server", "addr", addr)

	_, mode, err := syncSession(conn, logger, term.Stdout(), true)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	msgID := 0

	for {
		term.SetPrompt(modePrompt(mode))
		input, err := term.Readline()
		if err == io.EOF {
			logger.Debug("input stream ended, exiting")
			break
		}
		if err == readline.ErrInterrupt {
			if input == "" {
				break
			}
			continue
		}
		if strings.TrimSpace(input) == "/quit" {
			logger.Debug("quit command received, exiting")
			break
		}
		if strings.TrimSpace(input) == "/reset" {
			// First compact the session to preserve context before resetting.
			newMode, err := sendCommand(conn, logger, term.Stdout(), "/compact")
			if err != nil {
				fmt.Fprintf(term.Stderr(), "error: %v\n", err)
				break
			}
			newMode2, err := sendReset(conn, logger, term.Stdout(), true)
			if err != nil {
				fmt.Fprintf(term.Stderr(), "error: %v\n", err)
				break
			}
			if newMode != "" {
				mode = newMode
			}
			if newMode2 != "" {
				mode = newMode2
			}
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(input), "/") {
			newMode, err := sendCommand(conn, logger, term.Stdout(), input)
			if err != nil {
				fmt.Fprintf(term.Stderr(), "error: %v\n", err)
				break
			}
			if newMode != "" {
				mode = newMode
			}
			continue
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
			fmt.Fprintf(term.Stderr(), "error: %v\n", err)
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			fmt.Fprintf(term.Stderr(), "error: %v\n", err)
			logger.Error("failed to send message", "error", err)
			break
		}
		logger.Debug("sent message", "id", req.ID, "content", req.Content)

		if err := readLoop(conn, logger, term.Stdout(), true); err != nil {
			fmt.Fprintf(term.Stderr(), "error: %v\n", err)
			break
		}
	}

	return nil
}

// sendCommand sends a slash command to the server and renders the response.
// Returns the new mode if a mode_change command was executed.
func sendCommand(conn *websocket.Conn, logger *slog.Logger, w io.Writer, input string) (string, error) {
	req := types.Request{
		ID:      "0",
		Role:    "user",
		Content: strings.TrimSpace(input),
	}
	data, err := types.MarshalRequest(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal command: %w", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return "", fmt.Errorf("failed to send command: %w", err)
	}

	var mode string
	for {
		_, resp, err := conn.ReadMessage()
		if err != nil {
			return "", fmt.Errorf("failed to read command response: %w", err)
		}
		r, err := types.UnmarshalResponse(resp)
		if err != nil {
			return "", fmt.Errorf("failed to parse command response: %w", err)
		}
		newMode := renderResponse(w, r, true)
		if newMode != "" {
			mode = newMode
		}
		if r.Done {
			return mode, nil
		}
	}
}

// sendReset sends a reset request to the server, prints "New session",
// and streams the kickoff response. Returns the mode if available.
func sendReset(conn *websocket.Conn, logger *slog.Logger, w io.Writer, colorize bool) (string, error) {
	req := types.Request{
		ID:    "0",
		Reset: true,
	}
	data, err := types.MarshalRequest(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal reset request: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return "", fmt.Errorf("failed to send reset request: %w", err)
	}

	var bufferedKickoff []*types.Response
	var bannerPrinted bool
	var lastContent string
	var mode string
	for {
		_, resp, err := conn.ReadMessage()
		if err != nil {
			return "", fmt.Errorf("failed to read reset response: %w", err)
		}

		r, err := types.UnmarshalResponse(resp)
		if err != nil {
			return "", fmt.Errorf("failed to parse reset response: %w", err)
		}

		if r.Mode != "" {
			mode = r.Mode
		}

		// Print "New session" banner once on first assistant chunk.
		if r.Role == "assistant" && colorize && !bannerPrinted {
			printNewSession(w)
			bannerPrinted = true
		}

		// Buffer kickoff streaming responses for replay after banner.
		if r.Role == "tool_call" || r.Role == "assistant" || r.Role == "error" {
			bufferedKickoff = append(bufferedKickoff, r)
		}

		// Render non-assistant roles immediately.
		if r.Role != "assistant" {
			renderResponse(w, r, colorize)
			continue
		}

		// Assistant: delta printing (skip done response — already printed).
		if r.Done {
			// For batch responses, print any remaining content.
			if len(r.Content) > len(lastContent) {
				fmt.Fprint(w, r.Content[len(lastContent):])
			}
			fmt.Fprintln(w)
			if r.Reset && colorize && (r.Usage != nil || r.Timing != nil) {
				printStatsLine(w, r.Usage, r.Timing)
			}
			return mode, nil
		}

		// Print only the new characters since last response.
		if len(r.Content) > len(lastContent) {
			fmt.Fprint(w, r.Content[len(lastContent):])
		}
		lastContent = r.Content
	}
}

// printStatsLine prints the grey timestamp + token stats line, matching bantam's format.
func printStatsLine(w io.Writer, usage *types.ResponseUsage, timing *types.ResponseTiming) {
	var inputTokens, outputTokens, totalTokens int
	if usage != nil {
		inputTokens = usage.PromptTokens
		outputTokens = usage.CompletionTokens
		totalTokens = usage.TotalTokens
	}

	fmt.Fprintf(w, "\033[90m%s | ", time.Now().Format("15:04:05"))
	if timing != nil && timing.PromptPerSecond > 0 && timing.PredictedPerSecond > 0 {
		fmt.Fprintf(w, "%d (%.1f/s) => %d (%.1f/s) => %d (%.1fs)",
			inputTokens, timing.PromptPerSecond,
			outputTokens, timing.PredictedPerSecond,
			totalTokens, (timing.PromptMs+timing.PredictedMs)/1000)
	} else if timing != nil {
		fmt.Fprintf(w, "%d => %d => %d tokens (%.1fs)",
			inputTokens, outputTokens, totalTokens,
			(timing.PromptMs+timing.PredictedMs)/1000)
	} else {
		fmt.Fprintf(w, "%d => %d => %d tokens", inputTokens, outputTokens, totalTokens)
	}
	fmt.Fprintln(w, "\033[0m")
}
