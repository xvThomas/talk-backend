package inmemory

import (
	"context"
	"sync"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
)

// InMemoryStore is a thread-safe in-memory implementation of domain.MessageStore and domain.SessionBrowser.
type InMemoryStore struct {
	mu        sync.Mutex
	messages  []domain.Message
	sessionID string
	userID    string
}

// NewInMemoryStore creates an empty in-memory Store with the given session and user identifiers.
func NewInMemoryStore(sessionID, userID string) *InMemoryStore {
	return &InMemoryStore{sessionID: sessionID, userID: userID}
}

var _ domain.MessageStore = (*InMemoryStore)(nil)
var _ domain.SessionBrowser = (*InMemoryStore)(nil)

// Add appends a message to the store.
func (s *InMemoryStore) Add(msg domain.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}

// All returns a copy of all stored messages.
func (s *InMemoryStore) All() []domain.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]domain.Message, len(s.messages))
	copy(result, s.messages)
	return result
}

// Clear removes all messages from the store.
func (s *InMemoryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
}

// SessionID returns the current session identifier.
func (s *InMemoryStore) SessionID() string { return s.sessionID }

// UserID returns the user identifier.
func (s *InMemoryStore) UserID() string { return s.userID }

// SetSession is a no-op for the in-memory store (not used locally).
func (s *InMemoryStore) SetSession(ctx context.Context, sessionID string) error {
	return nil
}

// ListSessions is a no-op for the in-memory store (not used locally).
func (s *InMemoryStore) ListSessions(ctx context.Context, userID string) ([]domain.SessionSummary, error) {
	return nil, nil
}

// LoadSession is a no-op for the in-memory store (not used locally).
func (s *InMemoryStore) LoadSession(ctx context.Context, sessionID string) ([]domain.HistoryTurn, error) {
	return nil, nil
}
