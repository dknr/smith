package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"smith/types"
)

// HTTPProvider implements Provider by calling a chat-completions HTTP API.
type HTTPProvider struct {
	BaseURL      string
	APIKey       string
	Model        string
	SystemPrompt string
	DebugLogger  *slog.Logger
	TurnLogger   *TurnLogger
	// ProviderType indicates which LLM backend we are talking to.
	// Supported values: "llamacpp", "trtllm"
	ProviderType string
	// ReasoningEffort is the reasoning effort level (low, medium, high).
	// For llamacpp, this is mapped to a token budget (thinking_budget_tokens).
	// For trtllm, this is sent as the reasoning_effort field.
	ReasoningEffort string
}

// defaultTimeout is the HTTP client timeout for provider requests.
const defaultTimeout = 5 * 60 * time.Second

// defaultReasoningBudget is the default reasoning budget in tokens.
// llama.cpp's reasoning budget sampler limits tokens inside a reasoning block
// (e.g. between <think> and </think>). Default -1 means unlimited, which can
// cause server timeouts. 4096 tokens is a reasonable limit for reasoning.
const defaultReasoningBudget = 4096

// logDebug writes a request/response pair to the debug logger.
func (p *HTTPProvider) logDebug(ctx context.Context, method, url string, reqBody, respBody interface{}) {
	if p.DebugLogger == nil {
		return
	}
	reqJSON, _ := json.Marshal(reqBody)
	respJSON, _ := json.Marshal(respBody)
	p.DebugLogger.DebugContext(ctx, "provider request/response",
		"method", method,
		"url", url,
		"request_body", string(reqJSON),
		"response_body", string(respJSON),
	)
}

// chatRequest is the JSON request body for the chat completions endpoint.
type chatRequest struct {
	Model               string          `json:"model"`
	Messages            []msgEntry      `json:"messages"`
	Stream              bool            `json:"stream"`
	Tools               []types.ToolDef `json:"tools,omitempty"`
	ThinkingBudget      *int            `json:"thinking_budget_tokens,omitempty"`
	ReasoningEffort     *string         `json:"reasoning_effort,omitempty"`
}

type msgEntry struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []toolCallPayload `json:"tool_calls,omitempty"`
	ToolID    string           `json:"tool_call_id,omitempty"`
}

// toolCallPayload serializes tool calls in the standard nested format.
type toolCallPayload struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func msgToToolCall(tc types.ToolCall) toolCallPayload {
	return toolCallPayload{
		ID:   tc.ID,
		Type: "function",
		Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{
			Name:      tc.Name,
			Arguments: tc.Arguments,
		},
	}
}

// nonStreamChoice is a single choice from a non-streaming response.
type nonStreamChoice struct {
	Message nonStreamMessage `json:"message"`
}

type nonStreamMessage struct {
	Content   string          `json:"content"`
	ToolCalls []toolCallEntry `json:"tool_calls,omitempty"`
}

// toolCallEntry parses the standard format: {"type":"function","function":{"id":"...","name":"...","arguments":"..."}}
type toolCallEntry struct {
	Type     string `json:"type"`
	Function struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (t toolCallEntry) toToolCall() types.ToolCall {
	return types.ToolCall{
		ID:        t.Function.ID,
		Name:      t.Function.Name,
		Arguments: t.Function.Arguments,
	}
}

// nonStreamResponse is the top-level non-streaming response.
type nonStreamResponse struct {
	Choices []nonStreamChoice `json:"choices"`
	Usage   nonStreamUsage    `json:"usage,omitempty"`
	Timing  nonStreamTiming   `json:"timings,omitempty"`
}

type nonStreamUsage struct {
	PromptTokens     *int `json:"prompt_tokens"`
	CompletionTokens *int `json:"completion_tokens"`
	TotalTokens      *int `json:"total_tokens"`
}

type nonStreamTiming struct {
	PromptMs           *float64 `json:"prompt_ms"`
	PromptPerSecond    *float64 `json:"prompt_per_second"`
	PredictedMs        *float64 `json:"predicted_ms"`
	PredictedPerSecond *float64 `json:"predicted_per_second"`
}

// Call sends the conversation to the model (with optional tools) and returns
// a structured result containing either text content or tool calls.
func (p *HTTPProvider) Call(ctx context.Context, messages []types.Message, tools []types.ToolDef) (CallResult, error) {
	msgs := make([]msgEntry, 0, len(messages)+1)
	if p.SystemPrompt != "" {
		msgs = append(msgs, msgEntry{Role: "system", Content: p.SystemPrompt})
	}
	for _, m := range messages {
		me := msgEntry{
			Role:    m.Role,
			Content: m.Content,
			ToolID:  m.ToolID,
		}
		for _, tc := range m.ToolCalls {
			me.ToolCalls = append(me.ToolCalls, msgToToolCall(tc))
		}
		msgs = append(msgs, me)
	}

	// Prepare reasoning parameters based on provider type
	var thinkingBudget *int
	var reasoningEffort *string
	
	if p.ProviderType == "llamacpp" {
		// Map reasoning effort to thinking budget for llama.cpp
		var budget int
		switch p.ReasoningEffort {
		case "low":
			budget = 4096
		case "medium":
			budget = 8192
		case "high":
			budget = 16384
		default:
			budget = defaultReasoningBudget // fallback to default
		}
		thinkingBudget = &budget
	} else if p.ProviderType == "trtllm" {
		// Send reasoning effort directly for TensorRT-LLM
		reasoningEffort = &p.ReasoningEffort
	}
	// For other providers or empty type, don't set either (could add defaults later)

	body := chatRequest{
		Model:          p.Model,
		Messages:       msgs,
		Stream:         false,
		Tools:          tools,
		ThinkingBudget: thinkingBudget,
		ReasoningEffort: reasoningEffort,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return CallResult{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.BaseURL + "/chat/completions"

	// Log request if turn logger is configured.
	var turn int64
	if p.TurnLogger != nil {
		turn = p.TurnLogger.Next()
		if logErr := p.TurnLogger.LogRequest(turn, data); logErr != nil {
			p.logDebug(ctx, http.MethodPost, url, nil, fmt.Sprintf("turn log error: %v", logErr))
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return CallResult{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	httpClient := &http.Client{Timeout: defaultTimeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return CallResult{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return CallResult{}, fmt.Errorf("api error: status %d", resp.StatusCode)
	}

	var result nonStreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return CallResult{}, fmt.Errorf("failed to decode response: %w", err)
	}

	// Log response if turn logger is configured.
	if p.TurnLogger != nil && turn > 0 {
		respJSON, _ := json.Marshal(result)
		if logErr := p.TurnLogger.LogResponse(turn, respJSON); logErr != nil {
			p.logDebug(ctx, http.MethodPost, url, nil, fmt.Sprintf("turn log error: %v", logErr))
		}
	}

	if len(result.Choices) == 0 {
		return CallResult{}, fmt.Errorf("empty response from model")
	}

	msg := result.Choices[0].Message
	var toolCalls []types.ToolCall
	for _, tc := range msg.ToolCalls {
		toolCalls = append(toolCalls, tc.toToolCall())
	}

	p.logDebug(ctx, http.MethodPost, url, body, result)

	var usage *Usage
	if result.Usage.PromptTokens != nil || result.Usage.CompletionTokens != nil {
		usage = &Usage{
			PromptTokens:     ptrInt(result.Usage.PromptTokens),
			CompletionTokens: ptrInt(result.Usage.CompletionTokens),
			TotalTokens:      ptrInt(result.Usage.TotalTokens),
		}
	}

	var timing *Timing
	if result.Timing.PromptMs != nil || result.Timing.PromptPerSecond != nil {
		timing = &Timing{
			PromptMs:           ptrFloat64(result.Timing.PromptMs),
			PromptPerSecond:    ptrFloat64(result.Timing.PromptPerSecond),
			PredictedMs:        ptrFloat64(result.Timing.PredictedMs),
			PredictedPerSecond: ptrFloat64(result.Timing.PredictedPerSecond),
		}
	}

	return CallResult{
		Text:      msg.Content,
		ToolCalls: toolCalls,
		Usage:     usage,
		Timing:    timing,
	}, nil
}

func ptrInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func ptrFloat64(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
