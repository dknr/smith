package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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
	Stream   bool        `json:"stream,omitempty"`
}

type msgEntry struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse is the JSON response body from the chat completions endpoint.
type chatResponse struct {
	Choices []choiceEntry `json:"choices"`
}

type choiceEntry struct {
	Message msgEntry `json:"message"`
}

// Complete sends the conversation to the model and returns the response text.
func (p *HTTPProvider) Complete(ctx context.Context, messages []types.Message) (string, error) {
	msgs := make([]msgEntry, len(messages))
	for i, m := range messages {
		msgs[i] = msgEntry{Role: m.Role, Content: m.Content}
	}

	body := chatRequest{
		Model:    p.Model,
		Messages: msgs,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("api error: status %d", resp.StatusCode)
	}

	var apiResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return "", nil
	}

	return apiResp.Choices[0].Message.Content, nil
}
