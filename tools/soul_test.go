package tools

import (
	"context"
	"testing"

	"smith/memory"
)

func TestSoulToolRead(t *testing.T) {
	store, err := memory.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	tool := NewSoulTool(store)

	// Should return empty string for a new store
	result, err := tool.Execute(context.Background(), `{"action":"read"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestSoulToolModify(t *testing.T) {
	store, err := memory.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	tool := NewSoulTool(store)

	result, err := tool.Execute(context.Background(), `{"action":"modify","text":"new soul"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	got, err := store.GetSoul()
	if err != nil {
		t.Fatalf("GetSoul failed: %v", err)
	}
	if got != "new soul" {
		t.Errorf("got %q, want %q", got, "new soul")
	}
}

func TestSoulToolModifyEmptyText(t *testing.T) {
	store, err := memory.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	tool := NewSoulTool(store)

	_, err = tool.Execute(context.Background(), `{"action":"modify"}`)
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestSoulToolUnknownAction(t *testing.T) {
	store, err := memory.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	tool := NewSoulTool(store)

	_, err = tool.Execute(context.Background(), `{"action":"unknown"}`)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestSoulToolInvalidArgs(t *testing.T) {
	store, err := memory.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	tool := NewSoulTool(store)

	_, err = tool.Execute(context.Background(), `not json`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
