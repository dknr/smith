package agent

import (
	"context"
	"fmt"

	"smith/llm"
	"smith/types"
)

// Agent manages conversation history and delegates to an LLM Provider.
type Agent struct {
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
// and returns the response wrapped as a types.Response.
func (a *Agent) ProcessMessage(ctx context.Context, content string) (*types.Response, error) {
	a.history = append(a.history, types.Message{Role: "user", Content: content})

	result, err := a.provider.Complete(ctx, a.history)
	if err != nil {
		return nil, fmt.Errorf("provider error: %w", err)
	}

	return &types.Response{
		Role:    "assistant",
		Content: result,
		Done:    true,
	}, nil
}

// History returns a copy of the conversation history.
func (a *Agent) History() []types.Message {
	h := make([]types.Message, len(a.history))
	copy(h, a.history)
	return h
}
