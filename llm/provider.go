package llm

import (
	"context"

	"smith/types"
)

// Usage holds token usage information from the LLM provider.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Timing holds timing information from the LLM provider.
type Timing struct {
	PromptMs           float64
	PromptPerSecond    float64
	PredictedMs        float64
	PredictedPerSecond float64
}

// CallResult holds the result from a non-streaming provider call.
type CallResult struct {
	Text      string
	ToolCalls []types.ToolCall
	Usage     *Usage
	Timing    *Timing
}

// Provider is the interface that LLM backends must implement.
// Call sends the conversation (with optional tools) to the model and returns
// a structured result containing either text content or tool calls.
// Note: Complete is kept for potential future streaming support but is not
// currently used by the agent.
type Provider interface {
	Complete(ctx context.Context, messages []types.Message) (<-chan string, error)
	Call(ctx context.Context, messages []types.Message, tools []types.ToolDef) (CallResult, error)
}
