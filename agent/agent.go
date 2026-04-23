package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"smith/llm"
	"smith/memory"
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
	memStore *memory.Store
	turnSeq  atomic.Int64
}

// New creates a new Agent with the given Provider, tool Registry, session, logger, and memory store.
func New(provider llm.Provider, executor *tools.Registry, sess *session.Session, logger *slog.Logger, memStore *memory.Store) *Agent {
	a := &Agent{
		provider: provider,
		executor: executor,
		logger:   logger,
		session:  sess,
		memStore: memStore,
	}
	if sess != nil {
		if history, err := sess.LoadHistory(); err == nil {
			a.history = history
			a.logger.Info("loaded history", "messages", len(a.history))
		}
	}
	return a
}

// callStats formats token usage and timing into a concise log string.
func callStats(usage *llm.Usage, timing *llm.Timing) string {
	if usage == nil {
		return ""
	}
	promptTokens := usage.PromptTokens
	completionTokens := usage.CompletionTokens
	totalTokens := usage.TotalTokens
	if timing != nil && timing.PromptPerSecond > 0 && timing.PredictedPerSecond > 0 {
		return fmt.Sprintf("%d (%.1f/s) => %d (%.1f/s) => %d (%.1fs)",
			promptTokens, timing.PromptPerSecond,
			completionTokens, timing.PredictedPerSecond,
			totalTokens, (timing.PromptMs+timing.PredictedMs)/1000)
	} else if timing != nil {
		return fmt.Sprintf("%d => %d => %d tokens (%.1fs)",
			promptTokens, completionTokens, totalTokens,
			(timing.PromptMs+timing.PredictedMs)/1000)
	}
	return fmt.Sprintf("%d => %d => %d tokens", promptTokens, completionTokens, totalTokens)
}

// toolPreview returns the first 30 characters of output for logging.
func toolPreview(output string) string {
	if len(output) <= 30 {
		return output
	}
	return output[:30] + "…"
}

// compactPrompt is the default prompt used when summarizing a session for /compact.
const compactPrompt = "Condense the following conversation into a summary. Preserve facts, decisions, and tool outcomes that may be relevant later. Discard formatting artifacts and redundant output."

// Compact prompts the LLM to summarize the current session, archives the current
// session, creates a new one with the summary as its first message, and returns
// the summary via a response channel. If the provider call fails, the current
// session is left intact.
func (a *Agent) Compact(ctx context.Context) (<-chan *types.Response, error) {
	ch := make(chan *types.Response, 1)

	// Build transcript from current history (under lock to avoid data race).
	a.mu.Lock()
	transcript := buildTranscript(a.history)
	a.mu.Unlock()

	// Call provider one-shot to summarize (no tools).
	result, err := a.provider.Call(ctx, []types.Message{
		{Role: "system", Content: compactPrompt},
		{Role: "user", Content: "Summarize the following conversation:\n\n" + transcript},
	}, nil)
	if err != nil {
		a.logger.Error("compact provider error", "error", err)
		ch <- &types.Response{
			Role:    "error",
			Content: err.Error(),
			Done:    true,
		}
		close(ch)
		return ch, nil
	}

	// Archive current session and create a new one.
	if a.session != nil {
		if _, err := a.session.ArchiveCurrent(); err != nil {
			a.logger.Error("failed to archive session during compact", "error", err)
			ch <- &types.Response{
				Role:    "error",
				Content: fmt.Sprintf("Failed to archive session: %v", err),
				Done:    true,
			}
			close(ch)
			return ch, nil
		}
	}

	// Insert summary as the first message of the new session.
	summary := result.Text
	a.mu.Lock()
	a.history = []types.Message{{Role: "assistant", Content: summary}}
	a.mu.Unlock()

	if a.session != nil {
		if err := a.session.Append(types.Message{Role: "assistant", Content: summary}); err != nil {
			a.logger.Error("failed to save compact summary", "error", err)
		}
	}

	// Save summary to long-term memory for future reference
	if a.memStore != nil {
		tags := "summary,auto-generated," + time.Now().Format("2006-01-02")
		_, err := a.memStore.Add(summary, "context", tags)
		if err != nil {
			a.logger.Error("failed to save compact summary to memory", "error", err)
		}
	}

	ch <- &types.Response{
		Role:    "assistant",
		Content: summary,
		Done:    true,
	}
	close(ch)
	return ch, nil
}

// buildTranscript formats conversation history as a markdown transcript with
// ## role headings for display during session compaction.
func buildTranscript(messages []types.Message) string {
	var sb strings.Builder
	for _, m := range messages {
		sb.WriteString(fmt.Sprintf("## %s\n", m.Role))
		if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				sb.WriteString(fmt.Sprintf("%s(%s)\n", tc.Name, tc.Arguments))
			}
		}
		if m.Content != "" {
			sb.WriteString(m.Content)
		}
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// ProcessMessage appends a user message to history, sends it to the provider
// (with tools), and returns a channel of responses. The agent loops on tool
// calls until the provider returns text content. Intermediate streaming
// responses have done=false, the final response has done=true.
func (a *Agent) ProcessMessage(ctx context.Context, content string) (<-chan *types.Response, error) {
	respCh := make(chan *types.Response, 10)
	go func() {
		defer close(respCh)

		startLen := len(a.history)
		a.mu.Lock()
		a.history = append(a.history, types.Message{Role: "user", Content: content})
		a.mu.Unlock()

		turn := a.turnSeq.Add(1)
		a.logger.Info("turn", "turn", turn, "content", content)

	// Loop: call provider, handle tool calls or stream text.
		const maxCalls = 50
		var toolCount int
		var callCount int
		var outputTokens int
		start := time.Now()
		for {
			callCount++
			if callCount > maxCalls {
				respCh <- &types.Response{
					Role:    "error",
					Content: fmt.Sprintf("Agent exceeded maximum tool calls (%d).", maxCalls),
					Done:    true,
				}
				return
			}
			result, err := a.provider.Call(ctx, a.history, a.executor.Definitions())
			if err != nil {
				a.logger.Error("provider error", "error", err)
				respCh <- &types.Response{
					Role:    "error",
					Content: err.Error(),
					Done:    true,
				}
				return
			}

			stats := callStats(result.Usage, result.Timing)
			a.logger.Info("provider call", "turn", turn, "call", callCount, "stats", stats)

			if len(result.ToolCalls) > 0 {
				if result.Usage != nil {
					outputTokens += result.Usage.CompletionTokens
				}
				toolCount = a.handleToolCalls(turn, ctx, result, respCh, toolCount)
				continue
			}

			// Text response — stream it.
			if result.Usage != nil {
				outputTokens += result.Usage.CompletionTokens
			}
			a.streamText(ctx, result.Text, result.Usage, result.Timing, respCh)

			// Save all new messages to the session.
			if a.session != nil {
				a.mu.Lock()
				if err := a.session.Append(a.history[startLen:]...); err != nil {
					a.logger.Error("failed to save session", "error", err)
				}
				a.mu.Unlock()
			}

			a.logger.Info("turn complete", "turn", turn, "calls", callCount, "tools", toolCount, "output_tokens", outputTokens, "duration", time.Since(start).Round(time.Millisecond))
			return
		}
	}()

	return respCh, nil
}

func (a *Agent) handleToolCalls(turn int64, ctx context.Context, result llm.CallResult, respCh chan<- *types.Response, toolCount int) int {
	for _, tc := range result.ToolCalls {
		a.mu.Lock()
		a.history = append(a.history, types.Message{
			Role:      "assistant",
			ToolCalls: []types.ToolCall{tc},
		})
		a.mu.Unlock()

		respCh <- &types.Response{
			Role:    "tool_call",
			Content: types.FormatToolCall(tc.Name, tc.Arguments),
			Done:    false,
		}

		argsDisplay := types.FormatToolCall(tc.Name, tc.Arguments)
		a.logger.Info("tool", "turn", turn, "name", tc.Name, "args", argsDisplay)

		output, err := a.executor.Execute(ctx, tc.Name, tc.Arguments)
		if err != nil {
			errMsg := fmt.Sprintf("Error executing %s: %v", tc.Name, err)
			a.logger.Info("tool error", "turn", turn, "name", tc.Name, "error", err)

			// Bash errors get automatic feedback: red line to client + user message to LLM.
			if tc.Name == "bash" {
				respCh <- &types.Response{
					Role:    "error",
					Content: errMsg,
					Done:    false,
				}
				a.mu.Lock()
				a.history = append(a.history, types.Message{Role: "user", Content: errMsg})
				a.mu.Unlock()
			} else {
				a.mu.Lock()
				a.history = append(a.history, types.Message{
					Role:   "tool",
					Content: errMsg,
					ToolID: tc.ID,
				})
				a.mu.Unlock()
			}
			toolCount++
			continue
		}

		a.logger.Info("tool result", "turn", turn, "name", tc.Name, "chars", len(output), "preview", toolPreview(output))

		a.mu.Lock()
		a.history = append(a.history, types.Message{
			Role:   "tool",
			Content: output,
			ToolID: tc.ID,
		})
		a.mu.Unlock()

		respCh <- &types.Response{
			Role:   "tool",
			Content: output,
			Done:    false,
		}
		toolCount++
	}
	return toolCount
}

func (a *Agent) streamText(ctx context.Context, text string, usage *llm.Usage, timing *llm.Timing, respCh chan<- *types.Response) {
	a.mu.Lock()
	a.history = append(a.history, types.Message{
		Role:    "assistant",
		Content: text,
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
		Content: text,
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

// SetMode sets the active tool mode for this agent.
func (a *Agent) SetMode(mode types.Mode) {
	a.executor.SetMode(mode)
}

// Mode returns the current tool mode.
func (a *Agent) Mode() types.Mode {
	return a.executor.Mode()
}

// Session returns the session store for this agent.
func (a *Agent) Session() *session.Session {
	return a.session
}

// Reset clears all conversation history (in-memory), resets the
// turn sequence, and optionally runs the kickoff message through the agent
// loop. Returns a response channel to stream kickoff results to the client.
func (a *Agent) Reset(ctx context.Context, kickoff string) (<-chan *types.Response, error) {
	// Clear in-memory history.
	a.mu.Lock()
	a.history = nil
	a.mu.Unlock()

	// Reset turn sequence.
	a.turnSeq.Store(0)

	// Run kickoff through the agent loop if provided.
	if kickoff != "" {
		return a.ProcessMessage(ctx, kickoff)
	}

	// No kickoff — return a single marker response.
	ch := make(chan *types.Response, 1)
	ch <- &types.Response{Role: "reset", Done: true}
	close(ch)
	return ch, nil
}
