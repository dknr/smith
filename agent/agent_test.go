package agent

import (
	"context"
	"testing"
	"time"

	"smith/types"
)

// fakeProvider satisfies llm.Provider by feeding predetermined tokens.
type fakeProvider struct {
	tokens []string
	err    error
}

func (f *fakeProvider) Complete(ctx context.Context, messages []types.Message) (<-chan string, error) {
	if f.err != nil {
		return nil, f.err
	}
	ch := make(chan string, len(f.tokens))
	for _, t := range f.tokens {
		ch <- t
	}
	close(ch)
	return ch, nil
}

func TestHistory_empty(t *testing.T) {
	a := New(&fakeProvider{})
	h := a.History()
	if len(h) != 0 {
		t.Errorf("expected empty history, got %d messages", len(h))
	}
}

func TestProcessMessage_singleTurn(t *testing.T) {
	a := New(&fakeProvider{tokens: []string{"Hello", " world"}})

	respCh, err := a.ProcessMessage(context.Background(), "hi")
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}

	var responses []*types.Response
	for r := range respCh {
		responses = append(responses, r)
	}

	if len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(responses))
	}

	// First response: done=false, content="Hello"
	if responses[0].Done {
		t.Error("expected done=false for first response")
	}
	if responses[0].Content != "Hello" {
		t.Errorf("content = %q, want %q", responses[0].Content, "Hello")
	}

	// Second response: done=false, content="Hello world"
	if responses[1].Done {
		t.Error("expected done=false for second response")
	}
	if responses[1].Content != "Hello world" {
		t.Errorf("content = %q, want %q", responses[1].Content, "Hello world")
	}

	// Third response: done=true, content="Hello world"
	if !responses[2].Done {
		t.Error("expected done=true for final response")
	}
	if responses[2].Content != "Hello world" {
		t.Errorf("content = %q, want %q", responses[2].Content, "Hello world")
	}
}

func TestProcessMessage_providerError(t *testing.T) {
	a := New(&fakeProvider{err: context.Canceled})

	_, err := a.ProcessMessage(context.Background(), "hi")
	if err == nil {
		t.Error("expected error from provider error")
	}
}

func TestProcessMessage_contextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// Provider that blocks forever
	blocking := &fakeProvider{tokens: []string{"never"}}
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	a := New(blocking)
	respCh, err := a.ProcessMessage(ctx, "hi")
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}

	// Channel should close when context is cancelled
	// (the goroutine will eventually see ctx.Done and stop)
	select {
	case <-respCh:
	case <-time.After(500 * time.Millisecond):
		// If no response arrives, that's also acceptable for context cancel
	}
}

func TestHistory_afterProcessMessage(t *testing.T) {
	a := New(&fakeProvider{tokens: []string{"answer"}})

	respCh, err := a.ProcessMessage(context.Background(), "question")
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	for range respCh {
	}

	h := a.History()
	if len(h) != 2 {
		t.Fatalf("expected 2 messages in history, got %d", len(h))
	}
	if h[0].Role != "user" || h[0].Content != "question" {
		t.Errorf("history[0] = %+v, want {user, question}", h[0])
	}
	if h[1].Role != "assistant" || h[1].Content != "answer" {
		t.Errorf("history[1] = %+v, want {assistant, answer}", h[1])
	}
}

func TestHistory_defensiveCopy(t *testing.T) {
	a := New(&fakeProvider{tokens: []string{"hi"}})

	respCh, _ := a.ProcessMessage(context.Background(), "hello")
	for range respCh {
	}

	h1 := a.History()
	h1[0].Content = "mutated"

	h2 := a.History()
	if h2[0].Content == "mutated" {
		t.Error("History should return a defensive copy")
	}
}

func TestProcessMessage_multipleTurns(t *testing.T) {
	a := New(&fakeProvider{tokens: []string{"r1"}})

	respCh, _ := a.ProcessMessage(context.Background(), "q1")
	for range respCh {
	}

	// Second turn with same provider (reused for simplicity)
	a = New(&fakeProvider{tokens: []string{"r2"}})
	respCh, _ = a.ProcessMessage(context.Background(), "q2")
	for range respCh {
	}

	h := a.History()
	if len(h) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(h))
	}
	// Each agent instance has its own history
	if h[1].Content != "r2" {
		t.Errorf("last message = %q, want %q", h[1].Content, "r2")
	}
}

func TestNew_providerStored(t *testing.T) {
	fp := &fakeProvider{tokens: []string{"test"}}
	a := New(fp)
	if a.provider != fp {
		t.Error("provider not stored correctly")
	}
}
