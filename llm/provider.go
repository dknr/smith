package llm

import (
	"context"

	"smith/types"
)

// Provider is the interface that LLM backends must implement.
// Complete sends the conversation history to the model and returns a channel
// of streaming tokens. The channel is closed when the response is complete.
type Provider interface {
	Complete(ctx context.Context, messages []types.Message) (<-chan string, error)
}
