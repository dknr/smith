package agent

import (
	"context"
	"fmt"
	"sync/atomic"

	"smith/types"
)

// Message represents a single message in the conversation history.
type Message struct {
	Role    string
	Content string
}

// Handler is the interface that agent implementations must satisfy.
// Handle receives the conversation history and returns a channel of responses.
// The handler should send intermediate responses with done=false and the final
// response with done=true, then close the channel.
type Handler interface {
	Handle(ctx context.Context, messages []Message) (<-chan types.Response, error)
}

// Agent manages conversation history and delegates to a Handler.
type Agent struct {
	history []Message
	handler Handler
	idGen   atomic.Int64
}

// New creates a new Agent with the given Handler.
func New(h Handler) *Agent {
	return &Agent{
		handler: h,
	}
}

// ProcessMessage appends a user message to history, sends it to the handler,
// and returns a channel to read the handler's responses.
func (a *Agent) ProcessMessage(ctx context.Context, content string) (<-chan types.Response, error) {
	a.history = append(a.history, Message{Role: "user", Content: content})

	id := a.idGen.Add(1)
	ch, err := a.handler.Handle(ctx, a.history)
	if err != nil {
		return nil, fmt.Errorf("handler error: %w", err)
	}

	// Wrap the channel to assign IDs and set role
	wrapped := make(chan types.Response, 10)
	go func() {
		defer close(wrapped)
		for resp := range ch {
			resp.ID = fmt.Sprintf("%d", id)
			resp.Role = "assistant"
			wrapped <- resp
		}
	}()

	return wrapped, nil
}

// History returns a copy of the conversation history.
func (a *Agent) History() []Message {
	h := make([]Message, len(a.history))
	copy(h, a.history)
	return h
}
