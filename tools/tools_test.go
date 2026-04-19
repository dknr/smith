package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRegistry_Definitions(t *testing.T) {
	r := NewRegistry()
	defs := r.Definitions()
	if len(defs) != 3 {
		t.Fatalf("expected 3 tool definitions, got %d", len(defs))
	}

	names := make(map[string]bool)
	for _, d := range defs {
		names[d.Name] = true
		if d.Description == "" {
			t.Errorf("tool %s has empty description", d.Name)
		}
	}
	for _, want := range []string{"time", "list", "view"} {
		if !names[want] {
			t.Errorf("missing tool definition: %s", want)
		}
	}
}

func TestExecute_time(t *testing.T) {
	r := NewRegistry()
	result, err := r.Execute(context.Background(), "time", "{}")
	if err != nil {
		t.Fatalf("time: %v", err)
	}
	// Should be parseable as RFC3339
	_, err = time.Parse(time.RFC3339, result)
	if err != nil {
		t.Fatalf("time result not RFC3339: %v", err)
	}
}

func TestExecute_time_withArgs(t *testing.T) {
	r := NewRegistry()
	result, err := r.Execute(context.Background(), "time", `{"format":"2006-01-02"}`)
	if err != nil {
		t.Fatalf("time: %v", err)
	}
	// Still RFC3339 (format param is accepted but ignored for now)
	_, err = time.Parse(time.RFC3339, result)
	if err != nil {
		t.Fatalf("time result not RFC3339: %v", err)
	}
}

func TestExecute_list(t *testing.T) {
	r := NewRegistry()
	result, err := r.Execute(context.Background(), "list", "{}")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.HasSuffix(result, "\n") {
		t.Error("list result should end with newline")
	}
}

func TestExecute_list_hiddenFiltered(t *testing.T) {
	r := NewRegistry()
	// Default (all=false) should filter hidden files
	result, err := r.Execute(context.Background(), "list", "{}")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(result), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, ".") {
			t.Errorf("hidden file not filtered: %q", line)
		}
	}
}

func TestExecute_list_all(t *testing.T) {
	r := NewRegistry()
	// all=true should include hidden files
	result, err := r.Execute(context.Background(), "list", `{"all":true}`)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	// Just verify it succeeds and returns something
	if result == "" {
		t.Error("list all returned empty result")
	}
}

func TestExecute_list_nonexistent(t *testing.T) {
	r := NewRegistry()
	_, err := r.Execute(context.Background(), "list", `{"path":"/nonexistent/path/that/does/not/exist"}`)
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestExecute_view(t *testing.T) {
	r := NewRegistry()
	tmp := t.TempDir()
	testFile := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello tools"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := r.Execute(context.Background(), "view", `{"path":"`+testFile+`"}`)
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if result != "hello tools" {
		t.Errorf("view result = %q, want %q", result, "hello tools")
	}
}

func TestExecute_view_missingPath(t *testing.T) {
	r := NewRegistry()
	_, err := r.Execute(context.Background(), "view", `{}`)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestExecute_view_nonexistent(t *testing.T) {
	r := NewRegistry()
	_, err := r.Execute(context.Background(), "view", `{"path":"/no/such/file"}`)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestExecute_unknownTool(t *testing.T) {
	r := NewRegistry()
	_, err := r.Execute(context.Background(), "nonexistent", "{}")
	if err == nil {
		t.Error("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("error should mention unknown tool: %v", err)
	}
}

func TestExecute_view_invalidJSON(t *testing.T) {
	r := NewRegistry()
	_, err := r.Execute(context.Background(), "view", "not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
