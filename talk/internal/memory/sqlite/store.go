package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
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
	tool_name  TEXT NOT NULL DEFAULT '',
	tool_input TEXT NOT NULL DEFAULT '',
	tool_output TEXT NOT NULL DEFAULT '',
	tool_call_id TEXT NOT NULL DEFAULT '',
	turn_id    TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS history_turns (
	session_id  TEXT NOT NULL REFERENCES sessions(id),
	turn_id     TEXT NOT NULL,
	question    TEXT NOT NULL DEFAULT '',
	answer      TEXT NOT NULL DEFAULT '',
	question_at DATETIME,
	answer_at   DATETIME,
	PRIMARY KEY (session_id, turn_id)
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_tool_name ON messages(tool_name);
CREATE INDEX IF NOT EXISTS idx_messages_tool_call_id ON messages(tool_call_id);
CREATE INDEX IF NOT EXISTS idx_history_turns_session_id ON history_turns(session_id);
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
	content := msg.Content
	toolName, toolInput, toolOutput, toolCallID := "", "", "", ""

	if msg.Role == domain.RoleAssistant && len(msg.ToolCalls) > 0 {
		rawCalls, err := json.Marshal(msg.ToolCalls)
		if err == nil {
			toolInput = string(rawCalls)
		}
	}

	if msg.Role == domain.RoleTool {
		if len(msg.ToolCalls) > 0 {
			toolName = msg.ToolCalls[0].Name
			toolCallID = msg.ToolCalls[0].ID
			rawInput, err := json.Marshal(msg.ToolCalls[0].Input)
			if err == nil {
				toolInput = string(rawInput)
			}
		}
		if len(msg.ToolResults) > 0 {
			toolOutput = msg.ToolResults[0].Content
			if toolCallID == "" {
				toolCallID = msg.ToolResults[0].ToolCallID
			}
		}
	}

	if _, err := s.db.Exec(
		"INSERT INTO messages (session_id, role, content, tool_name, tool_input, tool_output, tool_call_id, turn_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		s.sessionID, string(msg.Role), content, toolName, toolInput, toolOutput, toolCallID, msg.TurnID, now,
	); err != nil {
		return
	}

	_ = s.upsertHistoryTurnLocked(msg, now)
	s.messages = append(s.messages, msg)
}

func (s *Store) upsertHistoryTurnLocked(msg domain.Message, now string) error {
	if msg.TurnID == "" {
		return nil
	}

	question := ""
	answer := ""
	var questionAt any
	var answerAt any

	if msg.Role == domain.RoleUser && msg.Content != "" {
		question = msg.Content
		questionAt = now
	}
	if msg.Role == domain.RoleAssistant && msg.Content != "" && len(msg.ToolCalls) == 0 {
		answer = msg.Content
		answerAt = now
	}
	if question == "" && answer == "" {
		return nil
	}

	_, err := s.db.Exec(
		`INSERT INTO history_turns (session_id, turn_id, question, answer, question_at, answer_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(session_id, turn_id) DO UPDATE SET
		   question = CASE WHEN excluded.question <> '' THEN excluded.question ELSE history_turns.question END,
		   answer = CASE WHEN excluded.answer <> '' THEN excluded.answer ELSE history_turns.answer END,
		   question_at = COALESCE(excluded.question_at, history_turns.question_at),
		   answer_at = COALESCE(excluded.answer_at, history_turns.answer_at)`,
		s.sessionID,
		msg.TurnID,
		question,
		answer,
		questionAt,
		answerAt,
	)
	return err
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
	_, _ = s.db.Exec("DELETE FROM history_turns WHERE session_id = ?", s.sessionID)
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

// LoadHistoryTurnsFromSession returns the conversation history for the given session as question/answer pairs.
func (s *Store) LoadHistoryTurnsFromSession(_ context.Context, sessionID string) ([]domain.HistoryTurn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`
		SELECT turn_id, question, answer, question_at, answer_at
		FROM history_turns
		WHERE session_id = ?
		ORDER BY COALESCE(question_at, answer_at), turn_id
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("loading history turns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var turns []domain.HistoryTurn
	for rows.Next() {
		var turn domain.HistoryTurn
		var questionAt sql.NullString
		var answerAt sql.NullString
		if err := rows.Scan(&turn.TurnID, &turn.Question, &turn.Answer, &questionAt, &answerAt); err != nil {
			return nil, fmt.Errorf("scanning history turn: %w", err)
		}
		if questionAt.Valid {
			turn.At, _ = time.Parse(timeFormat, questionAt.String)
		} else if answerAt.Valid {
			turn.At, _ = time.Parse(timeFormat, answerAt.String)
		}
		turns = append(turns, turn)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(turns) > 0 {
		return turns, nil
	}

	// Backward compatibility path for legacy rows/tests where history_turns is empty.
	legacyRows, err := s.db.Query(
		"SELECT role, content, turn_id, created_at, tool_input FROM messages WHERE session_id = ? ORDER BY id",
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("loading legacy session messages: %w", err)
	}
	defer func() { _ = legacyRows.Close() }()

	type msgRow struct {
		role      string
		content   string
		turnID    string
		createdAt time.Time
		toolInput string
	}
	var msgs []msgRow
	for legacyRows.Next() {
		var m msgRow
		var createdAt string
		if err := legacyRows.Scan(&m.role, &m.content, &m.turnID, &createdAt, &m.toolInput); err != nil {
			return nil, fmt.Errorf("scanning legacy message: %w", err)
		}
		m.createdAt, _ = time.Parse(timeFormat, createdAt)
		msgs = append(msgs, m)
	}
	if err := legacyRows.Err(); err != nil {
		return nil, err
	}

	for i := 0; i < len(msgs); i++ {
		if msgs[i].role != string(domain.RoleUser) {
			continue
		}
		turn := domain.HistoryTurn{
			Question: msgs[i].content,
			At:       msgs[i].createdAt,
			TurnID:   msgs[i].turnID,
		}
		for j := i + 1; j < len(msgs); j++ {
			if msgs[j].role == string(domain.RoleUser) {
				break
			}
			if msgs[j].role == string(domain.RoleAssistant) && msgs[j].content != "" && msgs[j].toolInput == "" {
				turn.Answer = msgs[j].content
				i = j
			}
		}
		turns = append(turns, turn)
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
		"SELECT role, content, tool_name, tool_input, tool_output, tool_call_id, turn_id FROM messages WHERE session_id = ? ORDER BY id",
		s.sessionID,
	)
	if err != nil {
		return fmt.Errorf("loading messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	s.messages = nil
	for rows.Next() {
		var role, content, toolName, toolInput, toolOutput, toolCallID, turnID string
		if err := rows.Scan(&role, &content, &toolName, &toolInput, &toolOutput, &toolCallID, &turnID); err != nil {
			return fmt.Errorf("scanning message: %w", err)
		}
		msg := domain.Message{
			Role:    domain.Role(role),
			Content: content,
			TurnID:  turnID,
		}

		switch msg.Role {
		case domain.RoleAssistant:
			if toolInput != "" {
				var calls []domain.ToolCall
				if err := json.Unmarshal([]byte(toolInput), &calls); err == nil {
					msg.ToolCalls = calls
				}
			}
		case domain.RoleTool:
			if toolName != "" || toolInput != "" || toolOutput != "" || toolCallID != "" {
				var input map[string]any
				if toolInput != "" {
					_ = json.Unmarshal([]byte(toolInput), &input)
				}
				msg.ToolCalls = append(msg.ToolCalls, domain.ToolCall{
					ID:    toolCallID,
					Name:  toolName,
					Input: input,
				})
				msg.ToolResults = append(msg.ToolResults, domain.ToolResult{
					ToolCallID: toolCallID,
					Content:    toolOutput,
				})
			}
		}

		s.messages = append(s.messages, msg)
	}
	return rows.Err()
}

// DeleteSession removes a session and all its messages from the database.
func (s *Store) DeleteSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM messages WHERE session_id = ?", sessionID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting messages: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM history_turns WHERE session_id = ?", sessionID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting history turns: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM sessions WHERE id = ?", sessionID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	// If we just deleted the current session, reset in-memory state.
	if sessionID == s.sessionID {
		s.messages = nil
		s.materialized = false
	}
	return nil
}
