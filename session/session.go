package session

import (
	"encoding/json"
	"fmt"
	"sync"

	sqlite3 "github.com/ncruces/go-sqlite3"
	"smith/types"
)

// Session stores conversation history in an in-memory SQLite database.
type Session struct {
	mu   sync.Mutex
	conn *sqlite3.Conn
}

const createTableSQL = `
CREATE TABLE IF NOT EXISTS messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	role TEXT NOT NULL,
	content TEXT NOT NULL DEFAULT '',
	tool_calls TEXT DEFAULT NULL,
	tool_call_id TEXT DEFAULT NULL
);
`

// New creates a new in-memory session and initializes the database.
func New() (*Session, error) {
	return NewWithDB(":memory:")
}

// NewWithDB creates a new session backed by the given SQLite database path.
// Use ":memory:" for an ephemeral session, or a file path for persistence.
func NewWithDB(dbPath string) (*Session, error) {
	conn, err := sqlite3.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open session database: %w", err)
	}

	if err := conn.Exec(createTableSQL); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create session table: %w", err)
	}

	return &Session{conn: conn}, nil
}

// Close closes the session database connection.
func (s *Session) Close() error {
	return s.conn.Close()
}

// LoadHistory loads all messages from the session, ordered by insertion order.
func (s *Session) LoadHistory() ([]types.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, _, err := s.conn.Prepare("SELECT role, content, tool_calls, tool_call_id FROM messages ORDER BY id ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	var messages []types.Message
	for stmt.Step() {
		m := types.Message{
			Role:    stmt.ColumnText(0),
			Content: stmt.ColumnText(1),
		}
		toolCallsJSON := stmt.ColumnText(2)
		m.ToolID = stmt.ColumnText(3)

		if toolCallsJSON != "" {
			var toolCalls []types.ToolCall
			if err := json.Unmarshal([]byte(toolCallsJSON), &toolCalls); err != nil {
				return nil, fmt.Errorf("failed to parse tool_calls JSON: %w", err)
			}
			m.ToolCalls = toolCalls
		}

		messages = append(messages, m)
	}

	if err := stmt.Err(); err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}

	return messages, nil
}

// Clear deletes all messages from the session.
func (s *Session) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.conn.Exec("DELETE FROM messages")
	if err != nil {
		return fmt.Errorf("failed to clear session: %w", err)
	}

	// Reset the autoincrement counter so new IDs start from 1.
	err = s.conn.Exec("DELETE FROM sqlite_sequence WHERE name='messages'")
	if err != nil {
		return fmt.Errorf("failed to reset sequence: %w", err)
	}

	return nil
}

// Append saves messages to the session.
func (s *Session) Append(messages ...types.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, _, err := s.conn.Prepare(
		"INSERT INTO messages (role, content, tool_calls, tool_call_id) VALUES (?, ?, ?, ?)",
	)
	if err != nil {
		return fmt.Errorf("failed to prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, m := range messages {
		var toolCallsJSON string
		if len(m.ToolCalls) > 0 {
			data, err := json.Marshal(m.ToolCalls)
			if err != nil {
				return fmt.Errorf("failed to marshal tool_calls: %w", err)
			}
			toolCallsJSON = string(data)
		}

		stmt.BindText(1, m.Role)
		stmt.BindText(2, m.Content)
		stmt.BindText(3, toolCallsJSON)
		stmt.BindText(4, m.ToolID)

		if err := stmt.Exec(); err != nil {
			stmt.ClearBindings()
			return fmt.Errorf("failed to insert message: %w", err)
		}
		stmt.ClearBindings()
	}

	return nil
}
