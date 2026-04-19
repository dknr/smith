package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"smith/llm"
	"smith/session"
	"smith/tools"
	"smith/types"
)

// Agent manages conversation history and delegates to an LLM Provider.
type Agent struct {
	mu       sync.Mutex
	history  []types.Message
	provider llm.Provider
	executor *tools.Registry
	logger   *slog.Logger
	session  *session.Session
}

// New creates a new Agent with the given Provider, tool Registry, session, and logger.
func New(provider llm.Provider, executor *tools.Registry, sess *session.Session, logger *slog.Logger) *Agent {
	a := &Agent{
		provider: provider,
		executor: executor,
		logger:   logger,
		session:  sess,
	}
	if sess != nil {
		if history, err := sess.LoadHistory(); err == nil {
			a.history = history
		}
	}
	return a
}

// ProcessMessage appends a user message to history, sends it to the provider
// (with tools), and returns a channel of responses. The agent loops on tool
// calls until the provider returns text content. Intermediate streaming
// responses have done=false, the final response has done=true.
func (a *Agent) ProcessMessage(ctx context.Context, content string) (<-chan *types.Response, error) {
	a.mu.Lock()
	a.history = append(a.history, types.Message{Role: "user", Content: content})
	a.mu.Unlock()

	respCh := make(chan *types.Response, 10)
	go func() {
		defer close(respCh)

		startLen := len(a.history)

		// Loop: call provider, handle tool calls or stream text.
		for {
			result, err := a.provider.Call(ctx, a.history, a.executor.Definitions())
			if err != nil {
				respCh <- &types.Response{
					Role:    "error",
					Content: err.Error(),
					Done:    true,
				}
				return
			}

			if len(result.ToolCalls) > 0 {
				a.handleToolCalls(ctx, result, respCh)
				continue
			}

			// Text response — stream it.
			a.streamText(ctx, result.Text, result.Usage, result.Timing, respCh)

			// Save all new messages to the session.
			if a.session != nil {
				a.mu.Lock()
				if err := a.session.Append(a.history[startLen:]...); err != nil {
					a.logger.Error("failed to save session", "error", err)
				}
				a.mu.Unlock()
			}
			return
		}
	}()

	return respCh, nil
}

func (a *Agent) handleToolCalls(ctx context.Context, result llm.CallResult, respCh chan<- *types.Response) {
	// Append tool call messages to history and notify the client.
	for _, tc := range result.ToolCalls {
		a.mu.Lock()
		a.history = append(a.history, types.Message{
			Role:      "assistant",
			ToolCalls: []types.ToolCall{tc},
		})
		a.mu.Unlock()

		respCh <- &types.Response{
			Role:    "tool_call",
			Content: formatToolCall(tc.Name, tc.Arguments),
			Done:    false,
		}

		a.logger.Debug("executing tool", "name", tc.Name, "args", tc.Arguments)
		output, err := a.executor.Execute(ctx, tc.Name, tc.Arguments)
		if err != nil {
			output = fmt.Sprintf("Error executing %s: %v", tc.Name, err)
		}

		a.mu.Lock()
		a.history = append(a.history, types.Message{
			Role:    "tool",
			Content: output,
			ToolID:  tc.ID,
		})
		a.mu.Unlock()
	}
}

func (a *Agent) streamText(ctx context.Context, text string, usage *llm.Usage, timing *llm.Timing, respCh chan<- *types.Response) {
	var accumulated strings.Builder
	// Stream the text character by character for consistency with the
	// existing streaming protocol.
	for i := 0; i < len(text); i++ {
		accumulated.WriteString(string(text[i]))
		respCh <- &types.Response{
			Role:    "assistant",
			Content: accumulated.String(),
			Done:    false,
		}
	}

	a.mu.Lock()
	a.history = append(a.history, types.Message{
		Role:    "assistant",
		Content: accumulated.String(),
	})
	a.mu.Unlock()

	var respUsage *types.ResponseUsage
	if usage != nil {
		respUsage = &types.ResponseUsage{
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
		}
	}

	var respTiming *types.ResponseTiming
	if timing != nil {
		respTiming = &types.ResponseTiming{
			PromptMs:           timing.PromptMs,
			PromptPerSecond:    timing.PromptPerSecond,
			PredictedMs:        timing.PredictedMs,
			PredictedPerSecond: timing.PredictedPerSecond,
		}
	}

	respCh <- &types.Response{
		Role:    "assistant",
		Content: accumulated.String(),
		Done:    true,
		Usage:   respUsage,
		Timing:  respTiming,
	}
}

// History returns a copy of the conversation history.
func (a *Agent) History() []types.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	h := make([]types.Message, len(a.history))
	copy(h, a.history)
	return h
}

// formatToolCall formats a tool call as "name(key='value', ...)" for display.
func formatToolCall(name, argsJSON string) string {
	if argsJSON == "" || argsJSON == "{}" {
		return fmt.Sprintf("%s()", name)
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf("%s(%s)", name, argsJSON)
	}
	parts := make([]string, 0, len(args))
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, formatArg(v)))
	}
	return fmt.Sprintf("%s(%s)", name, strings.Join(parts, ", "))
}

func formatArg(v interface{}) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", val)
	}
}
