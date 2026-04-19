package types

import "encoding/json"

// Message represents a single message in a conversation.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolID    string     `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool invocation from an LLM response.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON-encoded arguments
}

// ToolDef describes a tool available to the LLM.
type ToolDef struct {
	Name        string                 `json:"-"`
	Description string                 `json:"-"`
	Parameters  map[string]interface{} `json:"-"`
}

// MarshalJSON encodes ToolDef in the OpenAI-compatible format:
// {"type":"function","function":{"name":"...","description":"...","parameters":{...}}}
func (t ToolDef) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        t.Name,
			"description": t.Description,
			"parameters":  t.Parameters,
		},
	})
}

// Request represents a message from the client to the server.
type Request struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
	Sync    bool   `json:"sync"`
}

// Response represents a message from the server to the client.
type Response struct {
	ID           string     `json:"id"`
	Role         string     `json:"role"`
	Content      string     `json:"content"`
	Done         bool       `json:"done"`
	History      []Message  `json:"history,omitempty"`
	SyncComplete bool       `json:"sync_complete,omitempty"`
	Usage        *ResponseUsage `json:"usage,omitempty"`
	Timing       *ResponseTiming  `json:"timing,omitempty"`
}

// ResponseUsage holds token usage information.
type ResponseUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ResponseTiming holds timing information.
type ResponseTiming struct {
	PromptMs           float64 `json:"prompt_ms"`
	PromptPerSecond    float64 `json:"prompt_per_second"`
	PredictedMs        float64 `json:"predicted_ms"`
	PredictedPerSecond float64 `json:"predicted_per_second"`
}

// MarshalRequest encodes a Request as JSON bytes.
func MarshalRequest(r Request) ([]byte, error) {
	return json.Marshal(r)
}

// UnmarshalRequest decodes JSON bytes into a Request.
func UnmarshalRequest(data []byte) (*Request, error) {
	var r Request
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// MarshalResponse encodes a Response as JSON bytes.
func MarshalResponse(r Response) ([]byte, error) {
	return json.Marshal(r)
}

// UnmarshalResponse decodes JSON bytes into a Response.
func UnmarshalResponse(data []byte) (*Response, error) {
	var r Response
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
