package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"smith/types"
)

// HTTPProvider implements Provider by calling an OpenAI-compatible HTTP API.
type HTTPProvider struct {
	BaseURL string
	APIKey  string
	Model   string
	Tools   []types.ToolDef
}

// defaultTimeout is the HTTP client timeout for provider requests.
const defaultTimeout = 60 * time.Second

// chatRequest is the JSON request body for the chat completions endpoint.
type chatRequest struct {
	Model    string          `json:"model"`
	Messages []msgEntry      `json:"messages"`
	Stream   bool            `json:"stream"`
	Tools    []types.ToolDef `json:"tools,omitempty"`
}

type msgEntry struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
	ToolID    string `json:"tool_call_id,omitempty"`
}

// openAIToolCall serializes tool calls in the OpenAI nested format.
type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func msgToOpenAI(tc types.ToolCall) openAIToolCall {
	return openAIToolCall{
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

// streamChoice is a single choice entry from a streaming chunk.
type streamChoice struct {
	Delta streamDelta `json:"delta"`
}

type streamDelta struct {
	Content string `json:"content"`
}

// streamChunk is a JSON line from the SSE stream.
type streamChunk struct {
	Choices []streamChoice `json:"choices"`
}

// nonStreamChoice is a single choice from a non-streaming response.
type nonStreamChoice struct {
	Message nonStreamMessage `json:"message"`
}

type nonStreamMessage struct {
	Content   string            `json:"content"`
	ToolCalls []toolCallEntry   `json:"tool_calls,omitempty"`
}

// toolCallEntry parses the OpenAI format: {"type":"function","function":{"id":"...","name":"...","arguments":"..."}}
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
}

// processSSELine handles a complete SSE event payload and sends tokens to the channel.
func processSSELine(payload string, ch chan<- string) {
	if payload == "[DONE]" {
		return
	}
	var chunk streamChunk
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return
	}
	if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
		ch <- chunk.Choices[0].Delta.Content
	}
}

// Complete sends the conversation to the model and returns a channel of
// streaming tokens. The channel is closed when the response is complete.
func (p *HTTPProvider) Complete(ctx context.Context, messages []types.Message) (<-chan string, error) {
	msgs := make([]msgEntry, len(messages))
	for i, m := range messages {
		msgs[i] = msgEntry{Role: m.Role, Content: m.Content}
	}

	body := chatRequest{
		Model:    p.Model,
		Messages: msgs,
		Stream:   true,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	httpClient := &http.Client{Timeout: defaultTimeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("api error: status %d", resp.StatusCode)
	}

	ch := make(chan string, 10)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				payload := strings.TrimPrefix(line, "data: ")
				if payload == "[DONE]" {
					return
				}
				processSSELine(payload, ch)
			}
		}
	}()

	return ch, nil
}

// Call sends the conversation to the model (with optional tools) and returns
// a structured result containing either text content or tool calls.
func (p *HTTPProvider) Call(ctx context.Context, messages []types.Message, tools []types.ToolDef) (CallResult, error) {
	msgs := make([]msgEntry, len(messages))
	for i, m := range messages {
		me := msgEntry{
			Role:    m.Role,
			Content: m.Content,
			ToolID:  m.ToolID,
		}
		for _, tc := range m.ToolCalls {
			me.ToolCalls = append(me.ToolCalls, msgToOpenAI(tc))
		}
		msgs[i] = me
	}

	body := chatRequest{
		Model:    p.Model,
		Messages: msgs,
		Stream:   false,
		Tools:    tools,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return CallResult{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.BaseURL + "/chat/completions"
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

	if len(result.Choices) == 0 {
		return CallResult{}, fmt.Errorf("empty response from model")
	}

	msg := result.Choices[0].Message
	var toolCalls []types.ToolCall
	for _, tc := range msg.ToolCalls {
		toolCalls = append(toolCalls, tc.toToolCall())
	}
	return CallResult{
		Text:      msg.Content,
		ToolCalls: toolCalls,
	}, nil
}
