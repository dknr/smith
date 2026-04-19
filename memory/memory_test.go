package memory

import (
	"strings"
	"testing"
)

func TestAddAndLoadAll(t *testing.T) {
	store, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	id, err := store.Add("Learned Lua string ops are needed", "lesson", "lua,strings")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}

	mems, err := store.LoadAll(10)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(mems) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(mems))
	}
	if mems[0].Content != "Learned Lua string ops are needed" {
		t.Errorf("got content %q", mems[0].Content)
	}
	if mems[0].Category != "lesson" {
		t.Errorf("got category %q", mems[0].Category)
	}
	if mems[0].Tags != "lua,strings" {
		t.Errorf("got tags %q", mems[0].Tags)
	}
}

func TestLoadAllEmpty(t *testing.T) {
	store, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	mems, err := store.LoadAll(10)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(mems) != 0 {
		t.Errorf("expected 0 memories, got %d", len(mems))
	}
}

func TestRecentByCategory(t *testing.T) {
	store, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	store.Add("A lesson", "lesson", "tag1")
	store.Add("A pattern", "pattern", "tag2")
	store.Add("Another lesson", "lesson", "tag3")

	mems, err := store.RecentByCategory("lesson", 10)
	if err != nil {
		t.Fatalf("RecentByCategory failed: %v", err)
	}
	if len(mems) != 2 {
		t.Fatalf("expected 2 lessons, got %d", len(mems))
	}
	if mems[0].Content != "Another lesson" {
		t.Errorf("expected 'Another lesson', got %q", mems[0].Content)
	}
	if mems[1].Content != "A lesson" {
		t.Errorf("expected 'A lesson', got %q", mems[1].Content)
	}
}

func TestUpdate(t *testing.T) {
	store, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	id, _ := store.Add("old content", "lesson", "old")
	if err := store.Update(id, "new content", "mistake", "new"); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	mems, err := store.LoadAll(10)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if mems[0].Content != "new content" {
		t.Errorf("got content %q", mems[0].Content)
	}
	if mems[0].Category != "mistake" {
		t.Errorf("got category %q", mems[0].Category)
	}
	if mems[0].Tags != "new" {
		t.Errorf("got tags %q", mems[0].Tags)
	}
}

func TestDelete(t *testing.T) {
	store, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	id, _ := store.Add("to be deleted", "lesson", "")
	if err := store.Delete(id); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	mems, err := store.LoadAll(10)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(mems) != 0 {
		t.Errorf("expected 0 memories after delete, got %d", len(mems))
	}
}

func TestLoadLimit(t *testing.T) {
	store, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	for i := 0; i < 5; i++ {
		store.Add("memory "+string(rune('0'+i)), "context", "")
	}

	mems, err := store.LoadAll(3)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(mems) != 3 {
		t.Errorf("expected 3 memories, got %d", len(mems))
	}
}

func TestMemoryToJSON(t *testing.T) {
	m := Memory{
		ID:        1,
		Content:   "test",
		Category:  "lesson",
		Tags:      "a,b",
		CreatedAt: "2026-04-18T12:00:00Z",
	}
	out := MemoryToJSON(m)
	if !strings.Contains(out, `"content": "test"`) {
		t.Errorf("JSON missing content field: %s", out)
	}
	if !strings.Contains(out, `"category": "lesson"`) {
		t.Errorf("JSON missing category field: %s", out)
	}
}

func TestGetSoulEmpty(t *testing.T) {
	store, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	got, err := store.GetSoul()
	if err != nil {
		t.Fatalf("GetSoul failed: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestSetAndGetSoul(t *testing.T) {
	store, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	want := "I am a curious agent.\nI value precision."
	if err := store.SetSoul(want); err != nil {
		t.Fatalf("SetSoul failed: %v", err)
	}

	got, err := store.GetSoul()
	if err != nil {
		t.Fatalf("GetSoul failed: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSetSoulOverwrite(t *testing.T) {
	store, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	store.SetSoul("first")
	store.SetSoul("second")

	got, err := store.GetSoul()
	if err != nil {
		t.Fatalf("GetSoul failed: %v", err)
	}
	if got != "second" {
		t.Errorf("got %q, want %q", got, "second")
	}
}
