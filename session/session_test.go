package session

import (
	"testing"

	"smith/types"
)

func TestNew(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()
}

func TestLoadHistory_empty(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	h, err := s.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(h) != 0 {
		t.Errorf("expected empty history, got %d messages", len(h))
	}
}

func TestAppend_and_LoadHistory(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	messages := []types.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}

	if err := s.Append(messages...); err != nil {
		t.Fatalf("Append: %v", err)
	}

	h, err := s.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(h) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(h))
	}
	if h[0].Role != "user" || h[0].Content != "hello" {
		t.Errorf("history[0] = %+v, want {user, hello}", h[0])
	}
	if h[1].Role != "assistant" || h[1].Content != "hi there" {
		t.Errorf("history[1] = %+v, want {assistant, hi there}", h[1])
	}
}

func TestAppend_multipleBatches(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	if err := s.Append(types.Message{Role: "user", Content: "q1"}); err != nil {
		t.Fatalf("Append batch 1: %v", err)
	}
	if err := s.Append(types.Message{Role: "assistant", Content: "a1"}); err != nil {
		t.Fatalf("Append batch 2: %v", err)
	}
	if err := s.Append(types.Message{Role: "user", Content: "q2"}); err != nil {
		t.Fatalf("Append batch 3: %v", err)
	}

	h, err := s.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(h) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(h))
	}
}

func TestAppend_toolCalls(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	messages := []types.Message{
		{
			Role:      "assistant",
			Content:   "",
			ToolCalls: []types.ToolCall{{ID: "call_1", Name: "time", Arguments: "{}"}},
		},
		{
			Role:   "tool",
			Content: "2026-04-18T12:00:00Z",
			ToolID: "call_1",
		},
	}

	if err := s.Append(messages...); err != nil {
		t.Fatalf("Append: %v", err)
	}

	h, err := s.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(h) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(h))
	}
	if len(h[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(h[0].ToolCalls))
	}
	if h[0].ToolCalls[0].Name != "time" {
		t.Errorf("tool call name = %q, want %q", h[0].ToolCalls[0].Name, "time")
	}
	if h[1].Role != "tool" || h[1].ToolID != "call_1" {
		t.Errorf("tool message = %+v, want {tool, call_1}", h[1])
	}
}

func TestAppend_emptyContent(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	if err := s.Append(types.Message{Role: "assistant", Content: "", ToolCalls: []types.ToolCall{{ID: "c1", Name: "time", Arguments: "{}"}}}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	h, err := s.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(h) != 1 {
		t.Fatalf("expected 1 message, got %d", len(h))
	}
	if h[0].Content != "" {
		t.Errorf("content = %q, want empty", h[0].Content)
	}
}

func TestClose(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
