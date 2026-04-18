package types

import (
	"encoding/json"
	"testing"
)

func TestMarshalRequest(t *testing.T) {
	r := Request{ID: "1", Role: "user", Content: "hello"}
	data, err := MarshalRequest(r)
	if err != nil {
		t.Fatalf("MarshalRequest: %v", err)
	}

	got, err := UnmarshalRequest(data)
	if err != nil {
		t.Fatalf("UnmarshalRequest: %v", err)
	}
	if got.ID != r.ID || got.Role != r.Role || got.Content != r.Content {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, r)
	}
}

func TestMarshalRequest_noDoneField(t *testing.T) {
	data, err := MarshalRequest(Request{ID: "1", Role: "user", Content: "hi"})
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
