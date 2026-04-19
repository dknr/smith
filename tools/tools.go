package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"smith/types"
)

// Executor handles tool execution by name and arguments.
type Executor interface {
	Execute(ctx context.Context, name string, argsJSON string) (string, error)
}

// Registry is a collection of named tools with execution and definition support.
type Registry struct {
	mu      sync.Mutex
	tools   map[string]toolFunc
	defs    []types.ToolDef
	defMap  map[string]types.ToolDef
}

type toolFunc func(ctx context.Context, argsJSON string) (string, error)

// Definitions returns the tool definitions for the LLM.
func (r *Registry) Definitions() []types.ToolDef {
	r.mu.Lock()
	defer r.mu.Unlock()
	defs := make([]types.ToolDef, 0, len(r.tools))
	for _, def := range toolDefs {
		defs = append(defs, def)
	}
	for _, def := range r.defs {
		defs = append(defs, def)
	}
	return defs
}

// Execute dispatches to the registered tool by name.
func (r *Registry) Execute(ctx context.Context, name, argsJSON string) (string, error) {
	fn, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return fn(ctx, argsJSON)
}

// NewRegistry creates a Registry with the built-in tools registered.
func NewRegistry() *Registry {
	r := &Registry{
		tools:  make(map[string]toolFunc),
		defMap: make(map[string]types.ToolDef),
	}
	for name, def := range toolDefs {
		r.defMap[name] = def
	}
	r.register("time", toolTime)
	r.register("list", toolList)
	r.register("view", toolView)
	r.register("lua", toolLua)
	r.register("edit", toolEdit)
	return r
}

func (r *Registry) register(name string, fn toolFunc) {
	r.tools[name] = fn
	if def, ok := toolDefs[name]; ok {
		r.defMap[name] = def
	}
}

// RegisterFn registers a stateful tool function and its definition.
func (r *Registry) RegisterFn(name string, fn toolFunc, def types.ToolDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[name] = fn
	r.defMap[name] = def
	r.defs = append(r.defs, def)
}

// toolDefs holds the JSON schema definitions for built-in tools.
var toolDefs = map[string]types.ToolDef{
	"time": {
		Name:        "time",
		Description: "Return the current date and time in ISO 8601 format.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"format": map[string]interface{}{
					"type":        "string",
					"description": "Time format string (default: RFC3339)",
				},
			},
			"required": []string{},
		},
	},
	"list": {
		Name:        "list",
		Description: "List files and directories in a path.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to list (default: current directory)",
				},
				"all": map[string]interface{}{
					"type":        "boolean",
					"description": "Include hidden files (default: false)",
				},
			},
			"required": []string{},
		},
	},
	"view": {
		Name:        "view",
		Description: "Read the contents of a file.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to read",
				},
			},
			"required": []string{"path"},
		},
	},
	"lua": {
		Name:        "lua",
		Description: "Execute a Lua script in a sandboxed environment. Exposes string operations and smith.view(path), smith.list(path), smith.write(path, content), and smith.print(...) for file operations and output.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"code": map[string]interface{}{
					"type":        "string",
					"description": "Lua script to execute",
				},
			},
			"required": []string{"code"},
		},
	},
	"edit": {
		Name:        "edit",
		Description: "Perform exact-match find-and-replace edits on a file, or create the file if old_string is empty. The path is relative to the working directory and must not contain path traversal.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file (relative to working directory)",
				},
				"old_string": map[string]interface{}{
					"type":        "string",
					"description": "Exact text to find (empty for new files)",
				},
				"new_string": map[string]interface{}{
					"type":        "string",
					"description": "Replacement text",
				},
				"replace_all": map[string]interface{}{
					"type":        "boolean",
					"description": "Replace all occurrences (default: false)",
				},
			},
			"required": []string{"file_path", "old_string", "new_string"},
		},
	},
}

// --- Built-in tool implementations ---

func toolTime(ctx context.Context, argsJSON string) (string, error) {
	// Always return RFC3339 (ISO 8601). Args are accepted but ignored for now.
	_ = argsJSON
	return time.Now().UTC().Format(time.RFC3339), nil
}

func toolList(ctx context.Context, argsJSON string) (string, error) {
	path := "."
	all := false
	if argsJSON != "" && argsJSON != "{}" {
		var p struct {
			Path string `json:"path"`
			All  bool   `json:"all"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &p); err == nil {
			if p.Path != "" {
				path = p.Path
			}
			all = p.All
		}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	var names []string
	for _, e := range entries {
		name := e.Name()
		if !all && len(name) > 0 && name[0] == '.' {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		suffix := ""
		if info.IsDir() {
			suffix = "/"
		}
		names = append(names, name+suffix)
	}

	if len(names) == 0 {
		return "(empty)", nil
	}
	return strings.Join(names, "\n")+"\n", nil
}

func toolView(ctx context.Context, argsJSON string) (string, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if p.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	data, err := os.ReadFile(p.Path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
