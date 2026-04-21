package session

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	sqlite3 "github.com/ncruces/go-sqlite3"
	"smith/types"
)

// Session stores conversation history in an in-memory SQLite database.
type Session struct {
	mu              sync.Mutex
	conn            *sqlite3.Conn
	activeSessionID int64
}

const createTableSQL = `
CREATE TABLE IF NOT EXISTS messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	role TEXT NOT NULL,
	content TEXT NOT NULL DEFAULT '',
	tool_calls TEXT DEFAULT NULL,
	tool_call_id TEXT DEFAULT NULL,
	session_id INTEGER
);
`

const createSessionTableSQL = `
CREATE TABLE IF NOT EXISTS sessions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	archived INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL
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

	// Migrate old schema: add session_id column if missing.
	var colCount int
	stmt, _, err := conn.Prepare("PRAGMA table_info(messages)")
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to prepare table info: %w", err)
	}
	for stmt.Step() {
		if stmt.ColumnText(1) == "session_id" {
			colCount++
		}
	}
	stmt.Close()
	if err := stmt.Err(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to check session_id column: %w", err)
	}
	if colCount == 0 {
		if err := conn.Exec("ALTER TABLE messages ADD COLUMN session_id INTEGER"); err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to add session_id column: %w", err)
		}
	}

	if err := conn.Exec(createSessionTableSQL); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create sessions table: %w", err)
	}

	// Migrate: ensure there is at least one active session.
	var sessionCount int
	stmt, _, err = conn.Prepare("SELECT COUNT(*) FROM sessions")
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to prepare session count query: %w", err)
	}
	for stmt.Step() {
		sessionCount = stmt.ColumnInt(0)
	}
	stmt.Close()
	if err := stmt.Err(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to count sessions: %w", err)
	}

	if sessionCount == 0 {
		// New DB or first run — create initial session.
		now := time.Now().UTC().Format(time.RFC3339)
		if err := insertSession(conn, now); err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to create initial session: %w", err)
		}

		id, err := getLastInsertID(conn)
		if err != nil {
			conn.Close()
			return nil, err
		}

		// Migrate existing messages to the initial session.
		var msgCount int
		stmt, _, err = conn.Prepare("SELECT COUNT(*) FROM messages")
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to prepare message count query: %w", err)
		}
		for stmt.Step() {
			msgCount = stmt.ColumnInt(0)
		}
		stmt.Close()
		if err := stmt.Err(); err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to count messages: %w", err)
		}
		if msgCount > 0 {
			if err := updateMessagesSession(conn, id); err != nil {
				conn.Close()
				return nil, err
			}
		}

		return &Session{conn: conn, activeSessionID: id}, nil
	}

	// Existing DB — find the active session.
	activeID, err := getActiveSessionID(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	if activeID == 0 {
		// No active session (all archived) — create one.
		now := time.Now().UTC().Format(time.RFC3339)
		if err := insertSession(conn, now); err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to create active session: %w", err)
		}

		activeID, err = getLastInsertID(conn)
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	return &Session{conn: conn, activeSessionID: activeID}, nil
}

// Close closes the session database connection.
func (s *Session) Close() error {
	return s.conn.Close()
}

// LoadHistory loads all messages from the active session, ordered by insertion order.
func (s *Session) LoadHistory() ([]types.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, _, err := s.conn.Prepare("SELECT role, content, tool_calls, tool_call_id FROM messages WHERE session_id = ? ORDER BY id ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query: %w", err)
	}
	stmt.BindInt(1, int(s.activeSessionID))
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

// ArchiveCurrent marks the active session as archived and creates a new one.
// Returns the ID of the new session.
func (s *Session) ArchiveCurrent() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Archive the current session.
	if err := archiveSession(s.conn, s.activeSessionID); err != nil {
		return 0, fmt.Errorf("failed to archive session: %w", err)
	}

	// Create a new active session.
	now := time.Now().UTC().Format(time.RFC3339)
	if err := insertSession(s.conn, now); err != nil {
		return 0, fmt.Errorf("failed to create new session: %w", err)
	}

	newID, err := getLastInsertID(s.conn)
	if err != nil {
		return 0, err
	}

	s.activeSessionID = newID
	return newID, nil
}

// Clear deletes all messages from the active session.
func (s *Session) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := clearMessagesForSession(s.conn, s.activeSessionID); err != nil {
		return err
	}

	// Reset the autoincrement counter so new IDs start from 1.
	err := s.conn.Exec("DELETE FROM sqlite_sequence WHERE name='messages'")
	if err != nil {
		return fmt.Errorf("failed to reset sequence: %w", err)
	}

	return nil
}

// Append saves messages to the active session.
func (s *Session) Append(messages ...types.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, _, err := s.conn.Prepare(
		"INSERT INTO messages (role, content, tool_calls, tool_call_id, session_id) VALUES (?, ?, ?, ?, ?)",
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
		stmt.BindInt(5, int(s.activeSessionID))

		if err := stmt.Exec(); err != nil {
			stmt.ClearBindings()
			return fmt.Errorf("failed to insert message: %w", err)
		}
		stmt.ClearBindings()
	}

	return nil
}

// insertSession inserts a new session row and returns no error.
func insertSession(conn *sqlite3.Conn, createdAt string) error {
	stmt, _, err := conn.Prepare("INSERT INTO sessions (archived, created_at) VALUES (0, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare insert session: %w", err)
	}
	stmt.BindText(1, createdAt)
	if err := stmt.Exec(); err != nil {
		stmt.Close()
		return fmt.Errorf("failed to insert session: %w", err)
	}
	stmt.Close()
	return nil
}

// getLastInsertID returns the id of the most recently inserted session.
func getLastInsertID(conn *sqlite3.Conn) (int64, error) {
	stmt, _, err := conn.Prepare("SELECT id FROM sessions ORDER BY id DESC LIMIT 1")
	if err != nil {
		return 0, fmt.Errorf("failed to prepare session ID query: %w", err)
	}
	var id int64
	for stmt.Step() {
		id = int64(stmt.ColumnInt(0))
	}
	stmt.Close()
	if err := stmt.Err(); err != nil {
		return 0, fmt.Errorf("failed to get session ID: %w", err)
	}
	return id, nil
}

// getActiveSessionID returns the id of the active (non-archived) session, or 0.
func getActiveSessionID(conn *sqlite3.Conn) (int64, error) {
	stmt, _, err := conn.Prepare("SELECT id FROM sessions WHERE archived = 0 LIMIT 1")
	if err != nil {
		return 0, fmt.Errorf("failed to prepare active session query: %w", err)
	}
	var id int64
	for stmt.Step() {
		id = int64(stmt.ColumnInt(0))
	}
	stmt.Close()
	if err := stmt.Err(); err != nil {
		return 0, fmt.Errorf("failed to get active session: %w", err)
	}
	return id, nil
}

// archiveSession marks a session as archived.
func archiveSession(conn *sqlite3.Conn, id int64) error {
	stmt, _, err := conn.Prepare("UPDATE sessions SET archived = 1 WHERE id = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare archive session: %w", err)
	}
	stmt.BindInt(1, int(id))
	if err := stmt.Exec(); err != nil {
		stmt.Close()
		return fmt.Errorf("failed to archive session: %w", err)
	}
	stmt.Close()
	return nil
}

// updateMessagesSession assigns all messages to the given session.
func updateMessagesSession(conn *sqlite3.Conn, sessionID int64) error {
	stmt, _, err := conn.Prepare("UPDATE messages SET session_id = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare update messages: %w", err)
	}
	stmt.BindInt(1, int(sessionID))
	if err := stmt.Exec(); err != nil {
		stmt.Close()
		return fmt.Errorf("failed to migrate messages: %w", err)
	}
	stmt.Close()
	return nil
}

// clearMessagesForSession deletes all messages for the given session.
func clearMessagesForSession(conn *sqlite3.Conn, sessionID int64) error {
	stmt, _, err := conn.Prepare("DELETE FROM messages WHERE session_id = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare clear messages: %w", err)
	}
	stmt.BindInt(1, int(sessionID))
	if err := stmt.Exec(); err != nil {
		stmt.Close()
		return fmt.Errorf("failed to clear messages: %w", err)
	}
	stmt.Close()
	return nil
}
