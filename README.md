# smith

LLM agent client/server with WebSocket communication, tool use, and conversation persistence.

## AI Disclosure

This project was built entirely through the use of "AI" agents, using locally hosted Large Language Models. Earlier in the project, the Charm agent was used to bring up Smith. At this point, Smith is mature enough that Smith itself is used to develop Smith.

The Smith project is an experiment. It was built for the sole purpose of determining that an agent can build itself.

## Building

```bash
git clone https://github.com/dknr/smith
cd smith
make build
```

## Configuration

Create `~/.config/smith/smith.toml`:

```toml
base_url = "https://your-api-endpoint/v1"
api_key = "your-key"
model = "your-model"

[agent]
max_tool_calls = 50
compact_prompt = "Summarize the conversation context concisely."

[provider]
timeout = "30s"
```

Both `base_url` and `model` are required. `api_key` is optional.

- `max_tool_calls` — Maximum tool calls per turn (default: 50)
- `compact_prompt` — Custom prompt for conversation compaction
- `timeout` — Provider request timeout (e.g., "30s", "1m")

## Usage

Start the server:

```bash
smith serve
```

Connect interactively:

```bash
smith chat
```

Send a one-shot message:

```bash
smith send "What time is it?"
```

Check the build version:

```bash
smith --version
```

