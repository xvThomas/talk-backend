package domain

import (
	"context"
	"time"
)

// SessionSummary holds brief metadata about a past conversation session.
type SessionSummary struct {
	// Unique session identifier
	ID string
	// Title of the session (typically the first user question)
	Title string
	// Timestamp of session creation (or last update)
	CreatedAt time.Time
	// Number of turns in the session, may be zero if unavailable
	TurnCount int
}

// HistoryTurn represents a single question/answer pair from a past conversation.
type HistoryTurn struct {
	// The user's question in this turn.
	Question string
	// The assistant's answer in this turn.
	Answer string
	// Timestamp of when this turn occurred (optional, may be zero if unavailable).
	At time.Time
	// The model used for this turn (optional, may be empty if unavailable).
	Model string
	// The number of times this turn has been called (optional, may be zero if unavailable).
	CallCount int
	// Unique identifier for this turn, used to reconcile in-memory and remote sources.
	TurnID string
}

// SessionScope identifies the current conversation context.
// It is an immutable value type; switching sessions means creating a new SessionScope.
type SessionScope struct {
	SessionID string
	UserID    string
}

// NewSessionScope creates a SessionScope.
func NewSessionScope(sessionID, userID string) SessionScope {
	return SessionScope{SessionID: sessionID, UserID: userID}
}

// MessageStore persists conversation messages.
// Implementations are fully stateless — all identity context is passed via parameters.
type MessageStore interface {
	// AddMessage adds a message to the given session.
	AddMessage(msg Message, scope SessionScope)
	// AllMessages returns all messages for the given session.
	AllMessages(sessionID string) []Message
	// ClearMessages removes all messages for the given session.
	ClearMessages(sessionID string)
}

// SessionBrowser provides access to historical sessions stored in an external system.
type SessionBrowser interface {
	ListSessions(ctx context.Context, userID string) ([]SessionSummary, error)
	LoadHistoryTurnsFromSession(ctx context.Context, sessionID string) ([]HistoryTurn, error)
	DeleteSession(ctx context.Context, sessionID string) error
}
