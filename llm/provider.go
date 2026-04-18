package llm

import (
	"context"

	"smith/types"
)

// Provider is the interface that LLM backends must implement.
// Complete sends the conversation history to the model and returns the response text.
type Provider interface {
	Complete(ctx context.Context, messages []types.Message) (string, error)
}
