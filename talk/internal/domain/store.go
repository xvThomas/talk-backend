package domain

import (
	"context"
	"time"
)

// SessionSummary holds brief metadata about a past conversation session.
type SessionSummary struct {
	// Unique session identifier
	ID string
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

// MessageStore persists conversation messages.
type MessageStore interface {
	Add(msg Message)
	All() []Message
	Clear()
	SessionID() string
	UserID() string
}

// SessionBrowser provides access to historical sessions stored in an external system.
type SessionBrowser interface {
	SetSession(ctx context.Context, sessionID string) error
	ListSessions(ctx context.Context, userID string) ([]SessionSummary, error)
	LoadSession(ctx context.Context, sessionID string) ([]HistoryTurn, error)
}
