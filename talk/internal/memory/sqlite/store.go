package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/xvThomas/talk-backend/talk/internal/domain"

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

// db is the shared database handle for both MessageRepository and Browser.
type db struct {
	conn *sql.DB
	mu   sync.RWMutex
}

// MessageRepository implements domain.MessageStore backed by SQLite.
type MessageRepository struct{ *db }

// Browser implements domain.SessionBrowser backed by SQLite.
type Browser struct{ *db }

var _ domain.MessageStore = (*MessageRepository)(nil)
var _ domain.MessageEventHandler = (*MessageRepository)(nil)
var _ domain.SessionBrowser = (*Browser)(nil)

// New opens (or creates) a SQLite database at dbPath and returns a MessageRepository and Browser.
func New(dbPath string) (*MessageRepository, *Browser, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("opening sqlite db: %w", err)
	}
	// Enable WAL mode for better concurrency.
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	// SQLite supports only one writer at a time.
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(0)

	if _, err := conn.Exec(schema); err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("creating schema: %w", err)
	}

	d := &db{conn: conn}
	return &MessageRepository{d}, &Browser{d}, nil
}

// DB returns the underlying database connection for sharing with other components.
func (d *db) DB() *sql.DB { return d.conn }

// Close closes the underlying database connection.
func (d *db) Close() error { return d.conn.Close() }

// HandleMessageEvent appends a message to the given session.
// The session is materialized in the database only on the first user message.
func (r *MessageRepository) HandleMessageEvent(ctx context.Context, event domain.MessageEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	msg := event.Message
	scope := event.SessionScope

	// Check if session is materialized.
	materialized, err := r.isSessionMaterialized(ctx, scope.SessionID())
	if err != nil {
		return err
	}
	if !materialized {
		if msg.Role != domain.RoleUser {
			return nil
		}
		// Materialize session with title = first user message.
		title := msg.Content
		if _, err := r.conn.ExecContext(ctx,
			"INSERT INTO sessions (id, user_id, title, created_at) VALUES (?, ?, ?, ?)",
			scope.SessionID(), scope.UserID(), title, time.Now().UTC().Format(timeFormat),
		); err != nil {
			return fmt.Errorf("materializing session: %w", err)
		}
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

	if _, err := r.conn.ExecContext(ctx,
		"INSERT INTO messages (session_id, role, content, tool_name, tool_input, tool_output, tool_call_id, turn_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		scope.SessionID(), string(msg.Role), content, toolName, toolInput, toolOutput, toolCallID, msg.TurnID, now,
	); err != nil {
		return fmt.Errorf("inserting message: %w", err)
	}

	return nil
}

// HandleTurnEvent persists one completed turn into history_turns.
func (r *MessageRepository) HandleTurnEvent(ctx context.Context, event domain.TurnEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if event.TurnID == "" {
		return nil
	}

	questionAt := any(nil)
	answerAt := any(nil)
	if !event.StartedAt.IsZero() {
		questionAt = event.StartedAt.UTC().Format(timeFormat)
	}
	if !event.EndedAt.IsZero() {
		answerAt = event.EndedAt.UTC().Format(timeFormat)
	}

	if _, err := r.conn.ExecContext(ctx,
		`INSERT INTO history_turns (session_id, turn_id, question, answer, question_at, answer_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(session_id, turn_id) DO UPDATE SET
		   question = CASE WHEN excluded.question <> '' THEN excluded.question ELSE history_turns.question END,
		   answer = CASE WHEN excluded.answer <> '' THEN excluded.answer ELSE history_turns.answer END,
		   question_at = COALESCE(excluded.question_at, history_turns.question_at),
		   answer_at = COALESCE(excluded.answer_at, history_turns.answer_at)`,
		event.SessionScope.SessionID(),
		event.TurnID,
		event.Input,
		event.Output,
		questionAt,
		answerAt,
	); err != nil {
		return fmt.Errorf("upserting history turn: %w", err)
	}

	return nil
}

func (r *MessageRepository) isSessionMaterialized(ctx context.Context, sessionID string) (bool, error) {
	var count int
	if err := r.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE id = ?", sessionID).Scan(&count); err != nil {
		return false, fmt.Errorf("checking session existence: %w", err)
	}
	return count > 0, nil
}

// AllMessages returns all messages for the given session.
func (r *MessageRepository) AllMessages(ctx context.Context, sessionID string) ([]domain.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.conn.QueryContext(ctx,
		"SELECT role, content, tool_name, tool_input, tool_output, tool_call_id, turn_id FROM messages WHERE session_id = ? ORDER BY id",
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var messages []domain.Message
	for rows.Next() {
		var role, content, toolName, toolInput, toolOutput, toolCallID, turnID string
		if err := rows.Scan(&role, &content, &toolName, &toolInput, &toolOutput, &toolCallID, &turnID); err != nil {
			return nil, fmt.Errorf("scanning message: %w", err)
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

		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating messages: %w", err)
	}
	return messages, nil
}

// ClearMessages removes all messages from the given session (in DB).
func (r *MessageRepository) ClearMessages(ctx context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.conn.ExecContext(ctx, "DELETE FROM messages WHERE session_id = ?", sessionID); err != nil {
		return fmt.Errorf("clearing messages: %w", err)
	}
	if _, err := r.conn.ExecContext(ctx, "DELETE FROM history_turns WHERE session_id = ?", sessionID); err != nil {
		return fmt.Errorf("clearing history turns: %w", err)
	}
	return nil
}

// ListSessions returns all sessions for the given user, ordered by creation date (newest first).
func (b *Browser) ListSessions(ctx context.Context, userID string) ([]domain.SessionSummary, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	rows, err := b.conn.QueryContext(ctx, `
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

	sessions := []domain.SessionSummary{}
	for rows.Next() {
		var ss domain.SessionSummary
		var createdAt string
		if err := rows.Scan(&ss.ID, &ss.Title, &createdAt, &ss.TurnCount); err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}
		parsed, err := time.Parse(timeFormat, createdAt)
		if err != nil {
			slog.Warn("parsing session created_at", "session_id", ss.ID, "value", createdAt, "error", err)
		}
		ss.CreatedAt = parsed
		sessions = append(sessions, ss)
	}
	return sessions, rows.Err()
}

// LoadHistoryTurnsFromSession returns the conversation history for the given session as question/answer pairs.
func (b *Browser) LoadHistoryTurnsFromSession(ctx context.Context, sessionID string) ([]domain.HistoryTurn, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	rows, err := b.conn.QueryContext(ctx, `
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
			parsed, err := time.Parse(timeFormat, questionAt.String)
			if err != nil {
				slog.Warn("parsing question_at", "session_id", sessionID, "turn_id", turn.TurnID, "error", err)
			}
			turn.At = parsed
		} else if answerAt.Valid {
			parsed, err := time.Parse(timeFormat, answerAt.String)
			if err != nil {
				slog.Warn("parsing answer_at", "session_id", sessionID, "turn_id", turn.TurnID, "error", err)
			}
			turn.At = parsed
		}
		turns = append(turns, turn)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return turns, nil
}

// DeleteSession removes a session and all its messages from the database.
func (b *Browser) DeleteSession(ctx context.Context, sessionID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	tx, err := b.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM messages WHERE session_id = ?", sessionID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting messages: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM history_turns WHERE session_id = ?", sessionID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting history turns: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", sessionID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("deleting session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
