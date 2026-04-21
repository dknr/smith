package tools

import (
	"context"
	"fmt"
	"sync"

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
	mode    types.Mode
}

type toolFunc func(ctx context.Context, argsJSON string) (string, error)

// Definitions returns the tool definitions for the LLM, filtered by the current mode.
func (r *Registry) Definitions() []types.ToolDef {
	r.mu.Lock()
	defer r.mu.Unlock()
	mode := r.mode
	if mode == "" {
		mode = types.SafeMode
	}
	builtins := modeTools[mode]
	defs := make([]types.ToolDef, 0, len(builtins))
	for name := range builtins {
		if def, ok := r.defMap[name]; ok {
			defs = append(defs, def)
		}
	}
	for _, def := range r.defs {
		if builtins[def.Name] {
			defs = append(defs, def)
		}
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

// SetMode sets the active tool mode.
func (r *Registry) SetMode(mode types.Mode) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mode = mode
}

// Mode returns the current tool mode.
func (r *Registry) Mode() types.Mode {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.mode == "" {
		return types.SafeMode
	}
	return r.mode
}

// NewRegistry creates a Registry with the built-in tools registered.
func NewRegistry() *Registry {
	r := &Registry{
		tools:  make(map[string]toolFunc),
		defMap: make(map[string]types.ToolDef),
		mode:   types.SafeMode,
	}
	for name, def := range toolDefs {
		r.defMap[name] = def
	}
	r.register("time", toolTime)
	r.register("list", toolList)
	r.register("view", toolView)
	r.register("lua", toolLua)
	r.register("edit", toolEdit)
	r.register("git", toolGit)
	r.register("bash", toolBash)
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
		Description: "Execute a Lua script in a sandboxed environment. Exposes string operations and smith.view(path), smith.list(path), and smith.print(...) for read-only file operations and output.",
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
	"git": {
		Name:        "git",
		Description: "Execute non-destructive git subcommands (e.g. status, diff, log, show, branch, tag, ls-files, blame, grep, remote, rev-parse, describe, for-each-ref, reflog, fsck, count-objects, shortlog).",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Git subcommand to execute (e.g. 'status', 'log --oneline', 'diff --stat'). Only non-destructive commands are allowed.",
				},
			},
			"required": []string{"command"},
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
	"bash": {
		Name:        "bash",
		Description: "Execute any shell command.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Shell command to execute",
				},
			},
			"required": []string{"command"},
		},
	},
}

// modeTools maps each mode to the set of tool names available in that mode.
var modeTools = map[types.Mode]map[string]bool{
	types.SafeMode: {
		"time":  true,
		"list":  true,
		"view":  true,
		"lua":   true,
		"soul":  true,
		"memory": true,
	},
	types.EditMode: {
		"time":  true,
		"list":  true,
		"view":  true,
		"lua":   true,
		"soul":  true,
		"memory": true,
		"git":   true,
		"edit":  true,
	},
	types.FullMode: {
		"time":  true,
		"list":  true,
		"view":  true,
		"lua":   true,
		"soul":  true,
		"memory": true,
		"git":   true,
		"edit":  true,
		"bash":  true,
	},
}
