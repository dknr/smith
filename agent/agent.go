package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"smith/llm"
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
}

// New creates a new Agent with the given Provider and tool Registry.
func New(provider llm.Provider, executor *tools.Registry, logger *slog.Logger) *Agent {
	return &Agent{
		provider: provider,
		executor: executor,
		logger:   logger,
	}
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

		// Loop: call provider, handle tool calls or stream text.
		for {
			result, err := a.provider.Call(ctx, a.history, a.executor.Definitions())
			if err != nil {
				respCh <- &types.Response{
					Role:    "assistant",
					Content: fmt.Sprintf("Error: %v", err),
					Done:    true,
				}
				return
			}

			if len(result.ToolCalls) > 0 {
				a.handleToolCalls(ctx, result, respCh)
				continue
			}

			// Text response — stream it.
			a.streamText(ctx, result.Text, respCh)
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

func (a *Agent) streamText(ctx context.Context, text string, respCh chan<- *types.Response) {
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

	respCh <- &types.Response{
		Role:    "assistant",
		Content: accumulated.String(),
		Done:    true,
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
