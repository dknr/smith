# smith

LLM agent client/server with WebSocket communication, tool use, and conversation persistence.

## Building

```bash
git clone https://github.com/dknr/smith
cd smith
go build .
```

## Configuration

Create `~/.config/smith/smith.toml`:

```toml
base_url = "https://your-api-endpoint/v1"
api_key = "your-key"
model = "your-model"
```

Both `base_url` and `model` are required. `api_key` is optional.

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

