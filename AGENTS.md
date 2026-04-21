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
  ├── serve [db_path]  — starts the WebSocket server; optional db_path for persistence
  ├── send [msg]     — one-shot message → response
  └── chat           — interactive loop with session sync

server/server.go     — WebSocket HTTP server; spawns a per-connection Agent
  └── agent/agent.go — conversation loop: calls LLM, handles tool calls, streams text
        └── llm.Provider  — interface (Complete/streaming, Call/structured)
              └── HTTPProvider (llm/http.go) — OpenAI-compatible HTTP client

tools/               — Executable tools: one file per tool (time.go, list.go, view.go, lua.go, edit.go, git.go) + registry (tools.go) + stateful tools (soul.go, memory.go)
  └── tools.go         — Registry infrastructure, mode system (SafeMode, EditMode, FullMode), toolDefs map
memory/memory.go     — In-memory SQLite store for soul (identity) and memories (learnings)
session/session.go   — In-memory SQLite persistence of conversation history
types/message.go     — Message, Request, Response, ToolDef, ToolCall structs + JSON marshaling
config/config.go     — XDG config loader (smith.toml in ~/.config/smith/)
client/client.go     — WebSocket client (Send, Chat with sync)
logging/logging.go   — Dual-output slog setup (file + stdout)
```

**Data flow**: Client → WebSocket → Server → Agent.ProcessMessage → Provider.Call → tool execution loop → stream text back → SQLite save.

## Key patterns

- **Provider interface** (`llm/provider.go`): All LLM backends implement `Complete(ctx, msgs) (<-chan string, error)` for streaming and `Call(ctx, msgs, tools) (CallResult, error)` for structured/one-shot calls. `CallResult` includes `Usage` and `Timing` parsed from the provider's JSON response body.
- **Tool registry** (`tools/tools.go`): Core `Registry` struct with `Definitions()` (returns tool definitions filtered by current mode), `Execute()` (dispatches by name), `SetMode()`, and `Mode()`. Built-in tool implementations live in separate files (`time.go`, `list.go`, `view.go`, `git.go`, `lua.go`, `edit.go`); their `ToolDef` entries are in `toolDefs` in `tools.go`. New built-in tools are added by creating a new file with a `toolXxx()` function and registering it in `NewRegistry()`. Stateful tools (soul, memory) are registered via `RegisterFn()`.
- **Tool modes**: Three modes control which tools are available to the LLM. `safe` (default): `time`, `list`, `view`, `lua`, `soul`, `memory`. `edit`: adds `git` and `edit`. `full`: includes all tools (expandable). Modes are set via `/safe`, `/edit`, `/full` slash commands in the chat client, or via the `mode` field in `Request`. The server intercepts these commands before the agent loop — they never appear in conversation history. `/mode` shows the current mode. Mode changes are sent to the client as `Response` objects with `command: "mode_change"`.
- **Soul**: Plain text identity stored in the agent's SQLite store. The agent reads it via the `soul` tool on kickoff and can modify it freely. No schema — just text.
- **Memory**: Structured learnings stored in the agent's SQLite store. Categories: lesson, pattern, preference, fact, mistake, context. Accessed via the `memory` tool.
- **Kickoff**: Configured in `smith.toml` as `kickoff`. On new sessions, the server sends the kickoff as the first user message, flows through the agent loop (tool calls, text), then the client displays "New session" + kickoff text.
- **Streaming protocol**: Server sends incremental `Response` objects with `Done=false`, final one with `Done=true`. Client diffs content to show deltas.
- **Session sync**: Clients send `Request{Sync: true}` to receive full history. Server responds with history messages then `SyncComplete: true`.
- **History immutability**: `Agent.History()` returns a defensive copy. Internal history is protected by `sync.Mutex`.
- **Config**: `smith.toml` requires `base_url` and `model`. Loaded from XDG config dir (`$XDG_CONFIG_HOME/smith/` or `~/.config/smith/`). Optional fields: `system_prompt` (prepended to every LLM request), `kickoff` (first user message on new sessions).
- **Error responses**: Provider errors use `Role: "error"` (not `"assistant"`), enabling the client to distinguish them from LLM content.
- **Client modes**: `Chat` colorizes tool calls (yellow), errors (red), and shows a grey stats line (`HH:MM:SS | X (Y/s) => Z (W/s) => N (T.s)`). `Send` suppresses tool calls and stats, printing only the final response.
- **New sessions**: On first connect, chat prints a grey `"HH:MM:SS | New session"` banner, then the kickoff text (if configured), then the agent's kickoff response.

## Tool Modes

Tool modes control which tools are available to the LLM. Modes are enforced server-side — the LLM only sees tool definitions matching the current mode.

| Mode  | Tools Available                                              |
|-------|-------------------------------------------------------------|
| `safe`  | `time`, `list`, `view`, `lua`, `soul`, `memory` (lua sandbox is read-only) |
| `edit`  | safe + `git`, `edit`                                       |
| `full`  | edit + all future tools (e.g., `bash`)                     |

### Commands

In the chat client (`smith chat`), use slash commands to switch modes:

```
/safe    → set mode to safe
/edit    → set mode to edit
/full    → set mode to full
/mode    → show current mode
/help    → list all available commands
```

Mode changes are displayed as grey text. Commands are intercepted by the server and never appear in conversation history.

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

## Integration Testing

After unit tests pass, test end-to-end with a real LLM provider.

### Setup

```bash
kill $(lsof -ti:26856) 2>/dev/null; sleep 1
./smith serve &
sleep 2
```

### Running Tests

```bash
./smith send "<message>" --verbose 2>&1
```

Output legend:
- Yellow `[33m` = tool calls
- Grey `[90m` = stats line
- Red `[31m` = errors

### Test Suite

**Basic Greeting**

```bash
./smith send "Hello, world!" --verbose 2>&1
```

Expected: Direct text response, no tool calls.

**Lua Tool — Simple Print**

```bash
./smith send "Use the lua tool to print 'hello from lua'" --verbose 2>&1
```

Expected: `smith.print('hello from lua')` — 1 attempt.

**Lua Tool — Math**

```bash
./smith send "Use the lua tool to calculate 17 * 42 and print the result" --verbose 2>&1
```

Expected: `smith.print(17 * 42)` → 714 — 1 attempt.

**Lua Tool — Directory Listing**

```bash
./smith send "Use the lua tool to list the current directory and print the number of entries" --verbose 2>&1
```

Expected: `smith.list(".")` with `#entries` — 1 attempt.

**Lua Tool — Sieve of Eratosthenes**

```bash
./smith send "Use the lua tool to implement the Sieve of Eratosthenes to find all primes from 0 to 100, then print them as a 10x4 grid (10 rows, 4 columns, comma-separated). Use smith.print for output." --verbose 2>&1
```

Expected: 25 primes in 10×4 grid. May loop 3–4 times (padding/formatting).

**Lua Tool — Fibonacci Grid**

```bash
./smith send "Use the lua tool to compute the first 50 Fibonacci numbers and print them as a 5-row by 10-column grid, comma-separated." --verbose 2>&1
```

Expected: 50 Fibonacci numbers in 5×10 grid. May loop 2–3 times (padding style).

**Lua Tool — Collatz Sequence**

```bash
./smith send "Use the lua tool to compute the Collatz sequence for n=27 (which has the longest sequence for n<30). Print the full sequence and its length." --verbose 2>&1
```

Expected: 112 elements. May loop 3–4 times (looping/formatting).

**Lua Tool — Perfect Numbers**

```bash
./smith send "Use the lua tool to find all perfect numbers up to 1000. A perfect number equals the sum of its proper divisors. Print each one." --verbose 2>&1
```

Expected: 6, 28, 496 — 1 attempt.

**Lua Tool — Pascal's Triangle**

```bash
./smith send "Use the lua tool to generate Pascal's triangle with 10 rows. Print each row on its own line, with numbers separated by spaces." --verbose 2>&1
```

Expected: 10 rows of Pascal's triangle — 1 attempt.

**Lua Tool — Multiplication Table**

```bash
./smith send "Use the lua tool to create a 5x5 multiplication table. Print each row on one line with values separated by tabs (use smith.print with multiple args)." --verbose 2>&1
```

Expected: 5×5 table with tab separators. May loop 1–2 times (multi-arg vs single string).

**Lua Tool — Longest Common Subsequence**

```bash
./smith send "Use the lua tool to find the longest common subsequence of 'ABCBDAB' and 'BDCAB'. Print the LCS string and its length." --verbose 2>&1
```

Expected: LCS = "BCAB", length 4 — 1 attempt.

**Lua Tool — Prime Factorization (small)**

```bash
./smith send "Use the lua tool to find the prime factorization of 13195. Print each prime factor and its exponent." --verbose 2>&1
```

Expected: 5¹, 7¹, 13¹, 29¹ — 1 attempt.

### Known Failures (Timeouts)

These tests exceed the 60s timeout — avoid for now:

- **Anagram grouping** — LLM loops on hash-map-like keying
- **Prime factorization of large numbers** (>10⁹) — exceeds Lua number precision or infinite loops

### Debug Logging

For provider + WebSocket traffic debugging:

```bash
./smith serve --debug &
```

Logs are written to `log/smith-serve.log` (server) and `log/smith-send.log` or `log/smith-chat.log` (client).

### Cleanup

```bash
kill $(lsof -ti:26856) 2>/dev/null
```

**Tip:** Always start with a fresh server. Session history from prior runs can cause the LLM to loop on tools it previously failed with.

## Gotchas

- **Tool call loop**: The agent loops on tool calls until the provider returns text. Each tool call appends to history, executes the tool, appends the result, then calls the provider again.
- **Text streaming**: `streamText` sends one response per character with accumulated content. The client diffs to show incremental text.
- **SQLite is in-memory only**: `session.New()` opens `:memory:` — history is lost on restart. No disk persistence.
- **Debug logging**: `--debug` flag enables debug-level logging to `log/smith-serve.log` (server) and `log/smith-send.log` or `log/smith-chat.log` (client). Server logs write to both file and stdout at Info+ level; debug logs are file-only.
- **Default listen address**: `localhost:26856`. Only change with `-a` if the port is already in use.
- **Go 1.25**: Uses `log/slog` and `sqlite3` from `ncruces` (WASM-optimized). Don't swap in `database/sql` — the ncruces driver uses a different API (`stmt.Step()`, `stmt.BindText()`).
- **Stats/timing from provider**: Token usage and timing come from the LLM provider's JSON response body (`usage` and `timings` keys), not a local stopwatch. Some providers (llama.cpp) include `timings`; others may only include `usage`.
- **Persistent database**: `smith serve [db_path]` opens a SQLite file for both session history and agent memory (soul + memories). Omit `db_path` for in-memory (ephemeral) mode.
