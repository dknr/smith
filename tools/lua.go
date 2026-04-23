package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	lua "github.com/Shopify/go-lua"

	"smith/types"
)

// LuaToolDef is the tool definition for "lua".
var LuaToolDef = types.ToolDef{
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
}

// toolLua executes a Lua script in a sandboxed environment.
// Exposes smith.view(path), smith.list(path), and smith.print(...)
// for read-only file operations and output.
func toolLua(ctx context.Context, argsJSON string) (string, error) {
	var p struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if p.Code == "" {
		return "", fmt.Errorf("code is required")
	}

	L := lua.NewState()
	// No Close method on go-lua State — let GC handle it.

	// Add basic string operations (sub, len, gmatch, etc.) while keeping
	// I/O, OS, and debug libraries disabled for sandboxing.
	lua.StringOpen(L)

	// Create the smith table on the stack, then set it as a global.
	L.NewTable() // [smith table]

	// Register smith.* functions as Go closures on the table.
	L.PushGoClosure(toolLuaView, 0)
	L.SetField(-2, "view")
	L.PushGoClosure(toolLuaList, 0)
	L.SetField(-2, "list")
	L.PushGoClosure(toolLuaPrint, 0)
	L.SetField(-2, "print")

	L.SetGlobal("smith")

	// Context-aware timeout hook — checks ctx.Done every 100k instructions.
	goHook := func(L *lua.State, _ lua.Debug) {
		select {
		case <-ctx.Done():
			L.PushString("execution cancelled")
			L.Error()
		default:
		}
	}
	lua.SetDebugHook(L, goHook, lua.MaskCount, 100000)

	if err := L.Load(strings.NewReader(p.Code), "lua_tool", "t"); err != nil {
		return "", fmt.Errorf("lua compile error: %w", err)
	}

	if err := L.ProtectedCall(0, 0, 0); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("execution cancelled: %w", ctx.Err())
		}
		return "", fmt.Errorf("lua runtime error: %w", err)
	}

	// Collect output from the print log.
	L.Global("luaPrintLog") // push luaPrintLog table onto stack
	if L.IsTable(-1) {
		var parts []string
		L.PushNil() // first key (nil) to start iteration
		for L.Next(-2) {
			// stack: [table, key, value]
			if s, ok := L.ToString(-1); ok {
				parts = append(parts, s)
			}
			L.Pop(1) // remove value; key stays for next Next() call
		}
		L.Pop(1) // remove luaPrintLog table
		if len(parts) > 0 {
			return strings.Join(parts, "\n"), nil
		}
	}
	L.Pop(1) // clean up if not a table

	return "", nil
}

// toolLuaView implements smith.view(path) — read a file.
func toolLuaView(L *lua.State) int {
	path, ok := L.ToString(1)
	if !ok || path == "" {
		L.PushNil()
		L.PushString("path is required")
		return 2
	}
	data, err := readFile(path)
	if err != nil {
		L.PushNil()
		L.PushString(err.Error())
		return 2
	}
	L.PushString(data)
	return 1
}

// toolLuaList implements smith.list(path) — list directory contents.
func toolLuaList(L *lua.State) int {
	path, ok := L.ToString(1)
	if !ok || path == "" {
		path = "."
	}
	entries, err := readDir(path)
	if err != nil {
		L.PushNil()
		L.PushString(err.Error())
		return 2
	}
	L.NewTable() // [table]
	for i, name := range entries {
		L.PushInteger(i + 1) // [table, key]
		L.PushString(name)   // [table, key, value]
		L.RawSet(-3)         // table[key] = value; pops key+value
	}
	return 1
}

// toolLuaPrint implements smith.print(...) — collect output.
func toolLuaPrint(L *lua.State) int {
	args := make([]string, 0, L.Top())
	for i := 1; i <= L.Top(); i++ {
		if s, ok := L.ToString(i); ok {
			args = append(args, s)
		}
	}
	// Ensure luaPrintLog table exists.
	L.Global("luaPrintLog")
	if !L.IsTable(-1) {
		L.Pop(1) // remove non-table
		L.NewTable()
		L.SetGlobal("luaPrintLog")
	}
	// Push the table again and append each arg.
	L.Global("luaPrintLog")
	n := L.RawLength(-1)
	for _, arg := range args {
		n++
		L.PushInteger(n)
		L.PushString(arg)
		L.RawSet(-3)
	}
	L.Pop(1) // remove table
	return 0
}

// readFile reads a file and returns its contents.
func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// readDir lists directory entries, returning names with directory suffix.
func readDir(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		suffix := ""
		if info, err := e.Info(); err == nil && info.IsDir() {
			suffix = "/"
		}
		names = append(names, name+suffix)
	}
	return names, nil
}
