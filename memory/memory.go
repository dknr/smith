package memory

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	sqlite3 "github.com/ncruces/go-sqlite3"
)

// Memory represents a single stored memory entry.
type Memory struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	Category  string    `json:"category"` // lesson, pattern, preference, fact, mistake, context
	Tags      string    `json:"tags"`     // comma-separated
	CreatedAt string    `json:"created_at"`
}

// Store provides an in-memory SQLite-backed agent state (soul + memories).
type Store struct {
	mu   sync.Mutex
	conn *sqlite3.Conn
}

const createTableSQL = `
CREATE TABLE IF NOT EXISTS memories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	content TEXT NOT NULL,
	category TEXT NOT NULL DEFAULT 'context',
	tags TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
	content,
	category,
	tags,
	content='memories',
	content_rowid='id'
);

CREATE TABLE IF NOT EXISTS agent_state (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL DEFAULT ''
);

CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
	INSERT INTO memories_fts(rowid, content, category, tags)
	VALUES (new.id, new.content, new.category, new.tags);
END;

CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
	INSERT INTO memories_fts(memories_fts, rowid, content, category, tags)
	VALUES ('delete', old.id, old.content, old.category, old.tags);
END;

CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
	INSERT INTO memories_fts(memories_fts, rowid, content, category, tags)
	VALUES ('delete', old.id, old.content, old.category, old.tags);
	INSERT INTO memories_fts(rowid, content, category, tags)
	VALUES (new.id, new.content, new.category, new.tags);
END;
`

// New creates a new agent state store backed by an in-memory SQLite database.
func New() (*Store, error) {
	return NewWithDB(":memory:")
}

// NewWithDB creates a new agent state store backed by the given SQLite database path.
// Use ":memory:" for an ephemeral store, or a file path for persistence.
func NewWithDB(dbPath string) (*Store, error) {
	conn, err := sqlite3.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open agent state database: %w", err)
	}

	if err := conn.Exec(createTableSQL); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return &Store{conn: conn}, nil
}

// Close closes the store database connection.
func (s *Store) Close() error {
	return s.conn.Close()
}

// --- Soul ---

// GetSoul returns the agent's soul text.
func (s *Store) GetSoul() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, _, err := s.conn.Prepare("SELECT value FROM agent_state WHERE key = 'soul'")
	if err != nil {
		return "", fmt.Errorf("failed to prepare soul query: %w", err)
	}
	defer stmt.Close()

	if stmt.Step() {
		return stmt.ColumnText(0), nil
	}
	return "", nil
}

// SetSoul sets the agent's soul text.
func (s *Store) SetSoul(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, _, err := s.conn.Prepare("INSERT OR REPLACE INTO agent_state (key, value) VALUES ('soul', ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare soul update: %w", err)
	}
	defer stmt.Close()

	stmt.BindText(1, text)
	if err := stmt.Exec(); err != nil {
		return fmt.Errorf("failed to set soul: %w", err)
	}
	return nil
}

// --- Memories ---

// Add creates a new memory entry and returns its ID.
func (s *Store) Add(content, category, tags string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	stmt, _, err := s.conn.Prepare(
		"INSERT INTO memories (content, category, tags, created_at) VALUES (?, ?, ?, ?)",
	)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare insert: %w", err)
	}
	defer stmt.Close()

	stmt.BindText(1, content)
	stmt.BindText(2, category)
	stmt.BindText(3, tags)
	stmt.BindText(4, now)

	if err := stmt.Exec(); err != nil {
		return 0, fmt.Errorf("failed to insert memory: %w", err)
	}

	stmt2, _, err := s.conn.Prepare("SELECT last_insert_rowid()")
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}
	defer stmt2.Close()

	var id int64
	if stmt2.Step() {
		id = stmt2.ColumnInt64(0)
	}
	return id, nil
}

// LoadAll returns all memories ordered by id DESC, limited to n.
func (s *Store) LoadAll(n int) ([]Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, _, err := s.conn.Prepare("SELECT id, content, category, tags, created_at FROM memories ORDER BY id DESC LIMIT ?")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	stmt.BindInt64(1, int64(n))

	var memories []Memory
	for stmt.Step() {
		m := Memory{
			ID:        stmt.ColumnInt64(0),
			Content:   stmt.ColumnText(1),
			Category:  stmt.ColumnText(2),
			Tags:      stmt.ColumnText(3),
			CreatedAt: stmt.ColumnText(4),
		}
		memories = append(memories, m)
	}

	if err := stmt.Err(); err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}

	return memories, nil
}

// RecentByCategory returns up to n memories of a specific category, ordered by id DESC.
func (s *Store) RecentByCategory(category string, n int) ([]Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, _, err := s.conn.Prepare("SELECT id, content, category, tags, created_at FROM memories WHERE category = ? ORDER BY id DESC LIMIT ?")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	stmt.BindText(1, category)
	stmt.BindInt64(2, int64(n))

	var memories []Memory
	for stmt.Step() {
		m := Memory{
			ID:        stmt.ColumnInt64(0),
			Content:   stmt.ColumnText(1),
			Category:  stmt.ColumnText(2),
			Tags:      stmt.ColumnText(3),
			CreatedAt: stmt.ColumnText(4),
		}
		memories = append(memories, m)
	}

	if err := stmt.Err(); err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}

	return memories, nil
}

// Update replaces the content, category, and tags of an existing memory.
func (s *Store) Update(id int64, content, category, tags string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, _, err := s.conn.Prepare("UPDATE memories SET content = ?, category = ?, tags = ? WHERE id = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare update: %w", err)
	}
	defer stmt.Close()

	stmt.BindText(1, content)
	stmt.BindText(2, category)
	stmt.BindText(3, tags)
	stmt.BindInt64(4, id)

	if err := stmt.Exec(); err != nil {
		return fmt.Errorf("failed to update memory: %w", err)
	}

	return nil
}

// Delete removes a memory by ID.
func (s *Store) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, _, err := s.conn.Prepare("DELETE FROM memories WHERE id = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare delete: %w", err)
	}
	defer stmt.Close()

	stmt.BindInt64(1, id)

	if err := stmt.Exec(); err != nil {
		return fmt.Errorf("failed to delete memory: %w", err)
	}

	return nil
}

// MemoryToJSON serializes a Memory for display.
func MemoryToJSON(m Memory) string {
	data, _ := json.MarshalIndent(m, "", "  ")
	return string(data)
}

// Search performs a full-text search across the memory store.
// It returns memories matching the query, ordered by relevance.
func (s *Store) Search(query string, limit int) ([]Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 10
	}

	stmt, _, err := s.conn.Prepare(`
		SELECT m.id, m.content, m.category, m.tags, m.created_at
		FROM memories_fts f
		JOIN memories m ON m.id = f.rowid
		WHERE memories_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare search query: %w", err)
	}
	defer stmt.Close()

	stmt.BindText(1, query)
	stmt.BindInt(2, limit)

	var memories []Memory
	for stmt.Step() {
		m := Memory{
			ID:        stmt.ColumnInt64(0),
			Content:   stmt.ColumnText(1),
			Category:  stmt.ColumnText(2),
			Tags:      stmt.ColumnText(3),
			CreatedAt: stmt.ColumnText(4),
		}
		memories = append(memories, m)
	}

	if err := stmt.Err(); err != nil {
		return nil, fmt.Errorf("search execution error: %w", err)
	}

	return memories, nil
}
