# Smith

LLM agent client/server with WebSocket communication, tool use, and conversation persistence.

## Commands

```bash
go build                                    # Build binary
go test ./...                               # Run all tests
go test ./package/... -run TestName         # Run specific test
```

## Architecture

```
cmd/root.go          — cobra CLI entry point; wires everything together
  ├── serve          — starts the WebSocket server
  ├── send [msg]     — one-shot message → response
  └── chat           — interactive loop with session sync

server/server.go     — WebSocket HTTP server; spawns a per-connection Agent
  └── agent/agent.go — conversation loop: calls LLM, handles tool calls, streams text
        └── llm.Provider  — interface (Complete/streaming, Call/structured)
              └── HTTPProvider (llm/http.go) — OpenAI-compatible HTTP client

tools/tools.go       — Registry of executable tools (time, list, view)
session/session.go   — In-memory SQLite persistence of conversation history
types/message.go     — Message, Request, Response, ToolDef, ToolCall structs + JSON marshaling
config/config.go     — XDG config loader (smith.toml in ~/.config/smith/)
client/client.go     — WebSocket client (Send, Chat with sync)
logging/logging.go   — Dual-output slog setup (file + stdout)
```

**Data flow**: Client → WebSocket → Server → Agent.ProcessMessage → Provider.Call → tool execution loop → stream text back → SQLite save.

## Key patterns

- **Provider interface** (`llm/provider.go`): All LLM backends implement `Complete(ctx, msgs) (<-chan string, error)` for streaming and `Call(ctx, msgs, tools) (CallResult, error)` for structured/one-shot calls.
- **Tool registry** (`tools/tools.go`): Tools are registered in `NewRegistry()` and exposed via `Definitions()` for the LLM and `Execute()` for dispatch. New tools are added by registering a `toolFunc` and a `ToolDef` in `toolDefs`.
- **Streaming protocol**: Server sends incremental `Response` objects with `Done=false`, final one with `Done=true`. Client diffs content to show deltas.
- **Session sync**: Clients send `Request{Sync: true}` to receive full history. Server responds with history messages then `SyncComplete: true`.
- **History immutability**: `Agent.History()` returns a defensive copy. Internal history is protected by `sync.Mutex`.
- **Config**: `smith.toml` requires `base_url` and `model`. Loaded from XDG config dir (`$XDG_CONFIG_HOME/smith/` or `~/.config/smith/`).

## Testing

After making changes, **run tests immediately** — do not ask the user to verify.

```bash
go test ./...                              # All tests
go test ./package/... -run TestName        # Single test
go test ./package/... -v -run TestName     # Verbose
```

- Use `fakeProvider` (in `agent_test.go`) for deterministic tests — set `callText` for text responses or `callTools` for tool call responses.
- Tests use `session.New()` which creates an in-memory SQLite database — no external deps needed.
- Always close session in tests: `defer sess.Close()`.
- Test helper `newFakeAgent(callText, callTools)` and `newFakeAgentWithErr(err)` are available.

## Gotchas

- **Tool call loop**: The agent loops on tool calls until the provider returns text. Each tool call appends to history, executes the tool, appends the result, then calls the provider again.
- **Text streaming**: `streamText` sends one response per character with accumulated content. The client diffs to show incremental text.
- **SQLite is in-memory only**: `session.New()` opens `:memory:` — history is lost on restart. No disk persistence.
- **Protocol logger**: When `--log-protocol` is passed to `serve`, full request/response JSON is written to `smith-protocol.log`.
- **Default listen address**: `localhost:26856`. Only change with `-a` if the port is already in use.
- **Go 1.25**: Uses `log/slog` and `sqlite3` from `ncruces` (WASM-optimized). Don't swap in `database/sql` — the ncruces driver uses a different API (`stmt.Step()`, `stmt.BindText()`).
