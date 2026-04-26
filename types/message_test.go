package types

import (
	"encoding/json"
	"testing"
)

func TestMarshalRequest(t *testing.T) {
	r := Request{ID: "1", Content: "hello"}
	data, err := MarshalRequest(r)
	if err != nil {
		t.Fatalf("MarshalRequest: %v", err)
	}

	got, err := UnmarshalRequest(data)
	if err != nil {
		t.Fatalf("UnmarshalRequest: %v", err)
	}
	if got.ID != r.ID || got.Content != r.Content {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, r)
	}
}

func TestMarshalRequest_noDoneField(t *testing.T) {
	data, err := MarshalRequest(Request{ID: "1", Content: "hi"})
	if err != nil {
		t.Fatalf("MarshalRequest: %v", err)
	}
	if bytes := []byte(data); len(bytes) > 0 && bytes[0] == '{' {
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("parse raw: %v", err)
		}
		if _, ok := raw["done"]; ok {
			t.Error("Request JSON should not contain 'done' field")
		}
	}
}

func TestUnmarshalRequest_malformed(t *testing.T) {
	_, err := UnmarshalRequest([]byte("not json"))
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestUnmarshalRequest_extraFields(t *testing.T) {
	data := []byte(`{"id":"1","role":"user","content":"hi","unknown":"value"}`)
	r, err := UnmarshalRequest(data)
	if err != nil {
		t.Fatalf("UnmarshalRequest: %v", err)
	}
	if r.ID != "1" || r.Content != "hi" {
		t.Errorf("unexpected result: %+v", r)
	}
}

func TestMarshalResponse(t *testing.T) {
	r := Response{ID: "1", Role: "assistant", Content: "hello", Done: true}
	data, err := MarshalResponse(r)
	if err != nil {
		t.Fatalf("MarshalResponse: %v", err)
	}

	got, err := UnmarshalResponse(data)
	if err != nil {
		t.Fatalf("UnmarshalResponse: %v", err)
	}
	if got.ID != r.ID || got.Role != r.Role || got.Content != r.Content || got.Done != r.Done {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, r)
	}
}

func TestMarshalResponse_doneFalse(t *testing.T) {
	r := Response{ID: "1", Role: "assistant", Content: "partial", Done: false}
	data, err := MarshalResponse(r)
	if err != nil {
		t.Fatalf("MarshalResponse: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse raw: %v", err)
	}
	if v, ok := raw["done"].(bool); !ok || v {
		t.Errorf("expected done=false, got %v", v)
	}
}

func TestUnmarshalResponse_malformed(t *testing.T) {
	_, err := UnmarshalResponse([]byte("not json"))
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestUnmarshalResponse_missingDone(t *testing.T) {
	data := []byte(`{"id":"1","role":"assistant","content":"hi"}`)
	r, err := UnmarshalResponse(data)
	if err != nil {
		t.Fatalf("UnmarshalResponse: %v", err)
	}
	if r.Done {
		t.Error("expected Done to default to false when missing")
	}
}

func TestMessage_toolCalls(t *testing.T) {
	m := Message{
		Role:    "assistant",
		Content: "",
		ToolCalls: []ToolCall{
			{ID: "call_1", Name: "time", Arguments: "{}"},
		},
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal Message with tool_calls: %v", err)
	}

	var got Message
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal Message with tool_calls: %v", err)
	}
	if len(got.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(got.ToolCalls))
	}
	if got.ToolCalls[0].ID != "call_1" || got.ToolCalls[0].Name != "time" {
		t.Errorf("unexpected tool call: %+v", got.ToolCalls[0])
	}
}

func TestMessage_toolResult(t *testing.T) {
	m := Message{
		Role:   "tool",
		Content: "2026-04-18T12:00:00Z",
		ToolID: "call_1",
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal Message with tool result: %v", err)
	}

	var got Message
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal Message with tool result: %v", err)
	}
	if got.Role != "tool" || got.ToolID != "call_1" {
		t.Errorf("unexpected tool message: %+v", got)
	}
}

func TestMessage_emptyToolCallsNotOmitted(t *testing.T) {
	m := Message{Role: "assistant", Content: "hello", ToolCalls: nil}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal Message without tool_calls: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse raw: %v", err)
	}
	if _, ok := raw["tool_calls"]; ok {
		t.Error("Message without tool_calls should not include 'tool_calls' field")
	}
}

func TestToolDef_marshal(t *testing.T) {
	def := ToolDef{
		Name:        "time",
		Description: "Get the current time",
		Parameters: map[string]interface{}{
			"type": "object",
		},
	}
	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("Marshal ToolDef: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse raw: %v", err)
	}
	if raw["type"] != "function" {
		t.Errorf("expected type=function, got %v", raw["type"])
	}
	fn, ok := raw["function"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected function key to be an object, got %T", raw["function"])
	}
	if fn["name"] != "time" {
		t.Errorf("function.name = %v, want %q", fn["name"], "time")
	}
	if fn["description"] != "Get the current time" {
		t.Errorf("function.description = %v", fn["description"])
	}
}
