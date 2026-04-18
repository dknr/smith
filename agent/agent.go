package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"smith/llm"
	"smith/types"
)

// Agent manages conversation history and delegates to an LLM Provider.
type Agent struct {
	mu       sync.Mutex
	history  []types.Message
	provider llm.Provider
}

// New creates a new Agent with the given Provider.
func New(provider llm.Provider) *Agent {
	return &Agent{
		provider: provider,
	}
}

// ProcessMessage appends a user message to history, sends it to the provider,
// and returns a channel of responses. Intermediate responses have done=false,
// the final response has done=true.
func (a *Agent) ProcessMessage(ctx context.Context, content string) (<-chan *types.Response, error) {
	a.mu.Lock()
	a.history = append(a.history, types.Message{Role: "user", Content: content})
	a.mu.Unlock()

	tokenCh, err := a.provider.Complete(ctx, a.history)
	if err != nil {
		return nil, fmt.Errorf("provider error: %w", err)
	}

	respCh := make(chan *types.Response, 10)
	go func() {
		defer close(respCh)

		var accumulated strings.Builder
		for token := range tokenCh {
			accumulated.WriteString(token)
			respCh <- &types.Response{
				Role:    "assistant",
				Content: accumulated.String(),
				Done:    false,
			}
		}

		result := accumulated.String()

		a.mu.Lock()
		a.history = append(a.history, types.Message{Role: "assistant", Content: result})
		a.mu.Unlock()

		respCh <- &types.Response{
			Role:    "assistant",
			Content: result,
			Done:    true,
		}
	}()

	return respCh, nil
}

// History returns a copy of the conversation history.
func (a *Agent) History() []types.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	h := make([]types.Message, len(a.history))
	copy(h, a.history)
	return h
}
