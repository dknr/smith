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

	"smith/types"
)

// HTTPProvider implements Provider by calling an OpenAI-compatible HTTP API.
type HTTPProvider struct {
	BaseURL string
	APIKey  string
	Model   string
}

// chatRequest is the JSON request body for the chat completions endpoint.
type chatRequest struct {
	Model    string      `json:"model"`
	Messages []msgEntry  `json:"messages"`
	Stream   bool        `json:"stream"`
}

type msgEntry struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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

	resp, err := http.DefaultClient.Do(req)
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
				var chunk streamChunk
				if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
					continue
				}
				if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
					ch <- chunk.Choices[0].Delta.Content
				}
			}
		}
	}()

	return ch, nil
}
