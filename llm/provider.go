package llm

import (
	"context"

	"smith/types"
)

// CallResult holds the result from a non-streaming provider call.
type CallResult struct {
	Text      string
	ToolCalls []types.ToolCall
}

// Provider is the interface that LLM backends must implement.
// Complete sends the conversation history to the model and returns a channel
// of streaming tokens. The channel is closed when the response is complete.
// Call sends the conversation (with optional tools) to the model and returns
// a structured result containing either text content or tool calls.
type Provider interface {
	Complete(ctx context.Context, messages []types.Message) (<-chan string, error)
	Call(ctx context.Context, messages []types.Message, tools []types.ToolDef) (CallResult, error)
}
