package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"

	_ "modernc.org/sqlite"
)

const timeFormat = "2006-01-02T15:04:05Z"

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
	id         TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL,
	title      TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL REFERENCES sessions(id),
	role       TEXT NOT NULL,
	content    TEXT NOT NULL DEFAULT '',
	turn_id    TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages(session_id);
`

// Store is a persistent SQLite implementation of domain.MessageStore and domain.SessionBrowser.
type Store struct {
	db        *sql.DB
	mu        sync.Mutex
	sessionID string
	userID    string
	// messages caches current session messages in memory for fast All() access.
	messages []domain.Message
	// materialized tracks whether the current session exists in the database.
	materialized bool
}

// NewStore opens (or creates) a SQLite database at dbPath and returns a Store.
func NewStore(dbPath, sessionID, userID string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db: %w", err)
	}
	// Enable WAL mode for better concurrency.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	s := &Store{
		db:        db,
		sessionID: sessionID,
		userID:    userID,
	}
	// If the session already exists in DB, load its messages.
	if err := s.loadCurrentSession(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// DB returns the underlying database connection for sharing with other components.
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Add appends a message to the current session.
// The session is materialized in the database only on the first user message.
func (s *Store) Add(msg domain.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.materialized {
		if msg.Role != domain.RoleUser {
			return
		}
		// Materialize session with title = first user message.
		title := msg.Content
		if _, err := s.db.Exec(
			"INSERT INTO sessions (id, user_id, title, created_at) VALUES (?, ?, ?, ?)",
			s.sessionID, s.userID, title, time.Now().UTC().Format(timeFormat),
		); err != nil {
			return
		}
		s.materialized = true
	}

	now := time.Now().UTC().Format(timeFormat)
	if _, err := s.db.Exec(
		"INSERT INTO messages (session_id, role, content, turn_id, created_at) VALUES (?, ?, ?, ?, ?)",
		s.sessionID, string(msg.Role), msg.Content, msg.TurnID, now,
	); err != nil {
		return
	}
	s.messages = append(s.messages, msg)
}

// All returns all messages for the current session.
func (s *Store) All() []domain.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.messages) == 0 {
		return nil
	}
	result := make([]domain.Message, len(s.messages))
	copy(result, s.messages)
	return result
}

// Clear removes all messages from the current session (in DB and in memory).
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.db.Exec("DELETE FROM messages WHERE session_id = ?", s.sessionID)
	s.messages = nil
}

// SessionID returns the current session identifier.
func (s *Store) SessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionID
}

// UserID returns the user identifier.
func (s *Store) UserID() string { return s.userID }

// SetSession switches to the given session and loads its messages from the database.
func (s *Store) SetSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionID = sessionID
	s.messages = nil
	s.materialized = false
	return s.loadCurrentSessionLocked()
}

// ListSessions returns all sessions for the given user, ordered by creation date (newest first).
func (s *Store) ListSessions(_ context.Context, userID string) ([]domain.SessionSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`
		SELECT s.id, s.title, s.created_at,
		       COUNT(CASE WHEN m.role = 'user' THEN 1 END) AS turn_count
		FROM sessions s
		LEFT JOIN messages m ON m.session_id = s.id
		WHERE s.user_id = ?
		GROUP BY s.id
		ORDER BY s.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sessions []domain.SessionSummary
	for rows.Next() {
		var ss domain.SessionSummary
		var createdAt string
		if err := rows.Scan(&ss.ID, &ss.Title, &createdAt, &ss.TurnCount); err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}
		ss.CreatedAt, _ = time.Parse(timeFormat, createdAt)
		sessions = append(sessions, ss)
	}
	return sessions, rows.Err()
}

// LoadSession returns the conversation history for the given session as question/answer pairs.
func (s *Store) LoadSession(_ context.Context, sessionID string) ([]domain.HistoryTurn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(
		"SELECT role, content, turn_id, created_at FROM messages WHERE session_id = ? ORDER BY id",
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("loading session messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type msgRow struct {
		role      string
		content   string
		turnID    string
		createdAt time.Time
	}
	var msgs []msgRow
	for rows.Next() {
		var m msgRow
		var createdAt string
		if err := rows.Scan(&m.role, &m.content, &m.turnID, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning message: %w", err)
		}
		m.createdAt, _ = time.Parse(timeFormat, createdAt)
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var turns []domain.HistoryTurn
	for i := 0; i < len(msgs); i++ {
		if msgs[i].role == string(domain.RoleUser) {
			turn := domain.HistoryTurn{
				Question: msgs[i].content,
				At:       msgs[i].createdAt,
				TurnID:   msgs[i].turnID,
			}
			if i+1 < len(msgs) && msgs[i+1].role == string(domain.RoleAssistant) {
				turn.Answer = msgs[i+1].content
				i++
			}
			turns = append(turns, turn)
		}
	}
	return turns, nil
}

// loadCurrentSession checks if the session already exists in the DB and loads messages.
func (s *Store) loadCurrentSession() error {
	return s.loadCurrentSessionLocked()
}

func (s *Store) loadCurrentSessionLocked() error {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = ?", s.sessionID).Scan(&count)
	if err != nil {
		return fmt.Errorf("checking session existence: %w", err)
	}
	if count == 0 {
		s.materialized = false
		s.messages = nil
		return nil
	}
	s.materialized = true

	rows, err := s.db.Query(
		"SELECT role, content, turn_id FROM messages WHERE session_id = ? ORDER BY id",
		s.sessionID,
	)
	if err != nil {
		return fmt.Errorf("loading messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	s.messages = nil
	for rows.Next() {
		var role, content, turnID string
		if err := rows.Scan(&role, &content, &turnID); err != nil {
			return fmt.Errorf("scanning message: %w", err)
		}
		s.messages = append(s.messages, domain.Message{
			Role:    domain.Role(role),
			Content: content,
			TurnID:  turnID,
		})
	}
	return rows.Err()
}
