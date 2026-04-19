package tools

import (
	"context"
	"strings"
	"testing"

	"smith/memory"
)

func TestMemoryToolAddAndRead(t *testing.T) {
	store, err := memory.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	tool := NewMemoryTool(store)

	result, err := tool.Execute(context.Background(), `{"action":"add","content":"test memory","category":"lesson","tags":"a,b"}`)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if !strings.Contains(result, "id=") {
		t.Errorf("expected id in result, got %q", result)
	}

	result, err = tool.Execute(context.Background(), `{"action":"read"}`)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !strings.Contains(result, "test memory") {
		t.Errorf("expected 'test memory' in result, got %q", result)
	}
}

func TestMemoryToolReadEmpty(t *testing.T) {
	store, err := memory.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	tool := NewMemoryTool(store)

	result, err := tool.Execute(context.Background(), `{"action":"read"}`)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !strings.Contains(result, "no memories") {
		t.Errorf("expected 'no memories' in result, got %q", result)
	}
}

func TestMemoryToolReadByCategory(t *testing.T) {
	store, err := memory.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	tool := NewMemoryTool(store)

	tool.Execute(context.Background(), `{"action":"add","content":"lesson 1","category":"lesson"}`)
	tool.Execute(context.Background(), `{"action":"add","content":"pattern 1","category":"pattern"}`)
	tool.Execute(context.Background(), `{"action":"add","content":"lesson 2","category":"lesson"}`)

	result, err := tool.Execute(context.Background(), `{"action":"read","category":"lesson"}`)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !strings.Contains(result, "lesson 2") {
		t.Errorf("expected 'lesson 2' in result, got %q", result)
	}
	if strings.Contains(result, "pattern 1") {
		t.Errorf("unexpected 'pattern 1' in category-filtered result, got %q", result)
	}
}

func TestMemoryToolUpdate(t *testing.T) {
	store, err := memory.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	tool := NewMemoryTool(store)

	tool.Execute(context.Background(), `{"action":"add","content":"old","category":"context"}`)

	result, err := tool.Execute(context.Background(), `{"action":"read"}`)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	// Extract the ID from the result
	var id int64
	for _, r := range result {
		if r >= '0' && r <= '9' {
			id = int64(r - '0')
			break
		}
	}
	if id <= 0 {
		t.Fatalf("could not extract ID from result: %q", result)
	}

	_, err = tool.Execute(context.Background(), `{"action":"update","id":`+string(rune('0'+id))+`,"content":"new"}`)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	result, err = tool.Execute(context.Background(), `{"action":"read"}`)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !strings.Contains(result, "new") {
		t.Errorf("expected 'new' in result, got %q", result)
	}
}

func TestMemoryToolDelete(t *testing.T) {
	store, err := memory.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	tool := NewMemoryTool(store)

	tool.Execute(context.Background(), `{"action":"add","content":"to delete"}`)

	result, err := tool.Execute(context.Background(), `{"action":"read"}`)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !strings.Contains(result, "to delete") {
		t.Fatalf("expected 'to delete' before delete, got %q", result)
	}

	// Extract ID
	var id int64
	for _, r := range result {
		if r >= '0' && r <= '9' {
			id = int64(r - '0')
			break
		}
	}

	_, err = tool.Execute(context.Background(), `{"action":"delete","id":`+string(rune('0'+id))+`}`)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	result, err = tool.Execute(context.Background(), `{"action":"read"}`)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !strings.Contains(result, "no memories") {
		t.Errorf("expected 'no memories' after delete, got %q", result)
	}
}

func TestMemoryToolUnknownAction(t *testing.T) {
	store, err := memory.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	tool := NewMemoryTool(store)

	_, err = tool.Execute(context.Background(), `{"action":"unknown"}`)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestMemoryToolAddMissingContent(t *testing.T) {
	store, err := memory.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	tool := NewMemoryTool(store)

	_, err = tool.Execute(context.Background(), `{"action":"add"}`)
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}

func TestMemoryToolUpdateMissingID(t *testing.T) {
	store, err := memory.New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	tool := NewMemoryTool(store)

	_, err = tool.Execute(context.Background(), `{"action":"update","content":"test"}`)
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
}
