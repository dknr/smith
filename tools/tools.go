package tools

import (
	"context"
	"fmt"
	"sort"
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
	added := make(map[string]bool)
	for name := range builtins {
		if def, ok := r.defMap[name]; ok && !added[def.Name] {
			defs = append(defs, def)
			added[def.Name] = true
		}
	}
	for _, def := range r.defs {
		if builtins[def.Name] && !added[def.Name] {
			defs = append(defs, def)
			added[def.Name] = true
		}
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
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
	r.register("time", toolTime, TimeToolDef)
	r.register("list", toolList, ListToolDef)
	r.register("view", toolView, ViewToolDef)
	r.register("lua", toolLua, LuaToolDef)
	r.register("edit", toolEdit, EditToolDef)
	r.register("git", toolGit, GitToolDef)
	r.register("bash", toolBash, BashToolDef)
	return r
}

func (r *Registry) register(name string, fn toolFunc, def types.ToolDef) {
	r.tools[name] = fn
	r.defMap[name] = def
}

// RegisterFn registers a stateful tool function and its definition.
func (r *Registry) RegisterFn(name string, fn toolFunc, def types.ToolDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[name] = fn
	r.defMap[name] = def
	r.defs = append(r.defs, def)
}

// modeTools maps each mode to the set of tool names available in that mode.
var modeTools = map[types.Mode]map[string]bool{
	types.SafeMode: {
		"time":     true,
		"list":     true,
		"view":     true,
		"lua":      true,
		"soul":     true,
		"memory":   true,
		"search":   true,
	},
	types.EditMode: {
		"time":     true,
		"list":     true,
		"view":     true,
		"lua":      true,
		"soul":     true,
		"memory":   true,
		"git":      true,
		"edit":     true,
		"search":   true,
	},
	types.FullMode: {
		"time":     true,
		"list":     true,
		"view":     true,
		"lua":      true,
		"soul":     true,
		"memory":   true,
		"git":      true,
		"edit":     true,
		"bash":     true,
		"search":   true,
	},
}
