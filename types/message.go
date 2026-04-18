package types

import "encoding/json"

// Request represents a message from the client to the server.
type Request struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response represents a message from the server to the client.
type Response struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
	Done    bool   `json:"done"`
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
