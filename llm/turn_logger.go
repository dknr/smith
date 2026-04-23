package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// TurnLogger writes request and response logs for each LLM call turn.
// Logs are written to files named like "smith-turn-00012-req.json" and "smith-turn-00012-res.json".
type TurnLogger struct {
	dir     string
	counter atomic.Int64
	mu      sync.Mutex
}

// NewTurnLogger creates a new TurnLogger that writes to the given directory.
// If the directory doesn't exist, it will be created.
func NewTurnLogger(dir string) (*TurnLogger, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create turn log directory: %w", err)
	}
	return &TurnLogger{dir: dir}, nil
}

// Next returns the next turn number (1-indexed).
func (l *TurnLogger) Next() int64 {
	return l.counter.Add(1)
}

// indentJSON takes raw JSON bytes and returns them formatted with indentation.
func indentJSON(data []byte) ([]byte, error) {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return data, nil // return original if not valid JSON
	}
	return json.MarshalIndent(v, "", "  ")
}

// LogRequest writes the request body to a turn log file.
func (l *TurnLogger) LogRequest(turn int64, reqBody []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	formatted, err := indentJSON(reqBody)
	if err != nil {
		return fmt.Errorf("failed to format request JSON: %w", err)
	}

	path := filepath.Join(l.dir, fmt.Sprintf("smith-turn-%06d-req.json", turn))
	return os.WriteFile(path, formatted, 0644)
}

// LogResponse writes the response body to a turn log file.
func (l *TurnLogger) LogResponse(turn int64, respBody []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	formatted, err := indentJSON(respBody)
	if err != nil {
		return fmt.Errorf("failed to format response JSON: %w", err)
	}

	path := filepath.Join(l.dir, fmt.Sprintf("smith-turn-%06d-res.json", turn))
	return os.WriteFile(path, formatted, 0644)
}
