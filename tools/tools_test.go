package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"smith/types"
)

func TestRegistry_Definitions(t *testing.T) {
	modeToolSets := map[string][]string{
		"safe":   {"time", "list", "view", "lua"},
		"edit":   {"time", "list", "view", "lua", "git", "edit"},
		"full":   {"time", "list", "view", "lua", "git", "edit", "bash"},
	}

	for mode, expected := range modeToolSets {
		t.Run(mode, func(t *testing.T) {
			r := NewRegistry()
			r.SetMode(types.Mode(mode))
			defs := r.Definitions()

			if len(defs) != len(expected) {
				t.Fatalf("expected %d tool definitions, got %d", len(expected), len(defs))
			}

			names := make(map[string]bool)
			for _, d := range defs {
				names[d.Name] = true
				if d.Description == "" {
					t.Errorf("tool %s has empty description", d.Name)
				}
			}
			for _, want := range expected {
				if !names[want] {
					t.Errorf("missing tool definition: %s", want)
				}
			}
		})
	}
}

func TestRegistry_Mode(t *testing.T) {
	r := NewRegistry()
	if r.Mode() != "safe" {
		t.Errorf("expected safe mode, got %q", r.Mode())
	}

	r.SetMode("edit")
	if r.Mode() != "edit" {
		t.Errorf("expected edit mode, got %q", r.Mode())
	}

	r.SetMode("full")
	if r.Mode() != "full" {
		t.Errorf("expected full mode, got %q", r.Mode())
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

func TestExecute_lua_basic(t *testing.T) {
	r := NewRegistry()
	result, err := r.Execute(context.Background(), "lua", `{"code":"smith.print('hello')"}`)
	if err != nil {
		t.Fatalf("lua: %v", err)
	}
	if result != "hello" {
		t.Errorf("result = %q, want %q", result, "hello")
	}
}

func TestExecute_lua_math(t *testing.T) {
	r := NewRegistry()
	result, err := r.Execute(context.Background(), "lua", `{"code":"smith.print(2 + 3)"}`)
	if err != nil {
		t.Fatalf("lua: %v", err)
	}
	if result != "5" {
		t.Errorf("result = %q, want %q", result, "5")
	}
}

func TestExecute_lua_missingCode(t *testing.T) {
	r := NewRegistry()
	_, err := r.Execute(context.Background(), "lua", `{}`)
	if err == nil {
		t.Error("expected error for missing code")
	}
}

func TestExecute_lua_invalidSyntax(t *testing.T) {
	r := NewRegistry()
	_, err := r.Execute(context.Background(), "lua", `{"code":"print("}`)
	if err == nil {
		t.Error("expected error for invalid syntax")
	}
	if !strings.Contains(err.Error(), "compile error") {
		t.Errorf("error should mention compile error: %v", err)
	}
}

func TestExecute_lua_view(t *testing.T) {
	r := NewRegistry()
	tmp := t.TempDir()
	testFile := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(testFile, []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}
	code := `local c=smith.view("` + testFile + `");smith.print(c)`
	args, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Execute(context.Background(), "lua", string(args))
	if err != nil {
		t.Fatalf("lua view: %v", err)
	}
	if result != "world" {
		t.Errorf("result = %q, want %q", result, "world")
	}
}

func TestExecute_lua_list(t *testing.T) {
	r := NewRegistry()
	result, err := r.Execute(context.Background(), "lua", `{"code":"local t=smith.list();smith.print(#t)"}`)
	if err != nil {
		t.Fatalf("lua list: %v", err)
	}
	n, _ := strconv.Atoi(result)
	if n < 1 {
		t.Errorf("list returned count %d, expected >= 1", n)
	}
}

func TestExecute_bash_basic(t *testing.T) {
	r := NewRegistry()
	result, err := r.Execute(context.Background(), "bash", `{"command":"echo hello"}`)
	if err != nil {
		t.Fatalf("bash: %v", err)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("result = %q, want to contain 'hello'", result)
	}
}

func TestExecute_bash_pwd(t *testing.T) {
	r := NewRegistry()
	result, err := r.Execute(context.Background(), "bash", `{"command":"pwd"}`)
	if err != nil {
		t.Fatalf("bash pwd: %v", err)
	}
	if !strings.Contains(result, "smith") {
		t.Errorf("result = %q, want to contain 'smith'", result)
	}
}

func TestExecute_bash_missingCommand(t *testing.T) {
	r := NewRegistry()
	_, err := r.Execute(context.Background(), "bash", `{}`)
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestExecute_bash_emptyCommand(t *testing.T) {
	r := NewRegistry()
	_, err := r.Execute(context.Background(), "bash", `{"command":""}`)
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestExecute_bash_truncation(t *testing.T) {
	r := NewRegistry()
	// Generate 20kB of output (above 16KB maxBashOutput threshold).
	args, err := json.Marshal(map[string]string{"command": "printf '%20000s' "})
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Execute(context.Background(), "bash", string(args))
	if err != nil {
		t.Fatalf("bash truncation: %v", err)
	}
	// Truncated output = 16384 (max) + "\n… [truncated]" (16 bytes) = 16400
	if len(result) != 16400 {
		t.Errorf("result length = %d, want 16400", len(result))
	}
	if !strings.Contains(result, "[truncated]") {
		t.Error("result should contain [truncated] marker")
	}
}

func TestExecute_bash_timeout(t *testing.T) {
	t.Skip("disabled: 30s timeout makes test suite too slow")
	r := NewRegistry()
	// Sleep 35 seconds — should be killed by 30s timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	_, err := r.Execute(ctx, "bash", `{"command":"sleep 40"}`)
	if err == nil {
		t.Error("expected timeout error for sleep 40")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "killed") {
		t.Errorf("expected timeout/killed error, got: %v", err)
	}
}
