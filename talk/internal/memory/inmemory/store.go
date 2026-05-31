package inmemory

import (
	"context"
	"sync"
	"time"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
)

// sessionData holds the messages and metadata for a single session.
type sessionData struct {
	messages   []domain.Message
	timestamps []time.Time
	title      string
	createdAt  time.Time
}

// InMemoryStore is a thread-safe in-memory implementation of domain.MessageStore and domain.SessionBrowser.
type InMemoryStore struct {
	mu        sync.Mutex
	sessions  map[string]*sessionData
	sessionID string
	userID    string
}

// NewInMemoryStore creates an empty in-memory Store with the given session and user identifiers.
// The session is not materialized until the first user message is added.
func NewInMemoryStore(sessionID, userID string) *InMemoryStore {
	return &InMemoryStore{
		sessions:  make(map[string]*sessionData),
		sessionID: sessionID,
		userID:    userID,
	}
}

var _ domain.MessageStore = (*InMemoryStore)(nil)
var _ domain.SessionBrowser = (*InMemoryStore)(nil)

// Add appends a message to the current session.
// The session is materialized only when the first user message is added.
// The title is set from the first user message content.
func (s *InMemoryStore) Add(msg domain.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sd, exists := s.sessions[s.sessionID]
	if !exists {
		if msg.Role != domain.RoleUser {
			return
		}
		sd = &sessionData{title: msg.Content, createdAt: time.Now()}
		s.sessions[s.sessionID] = sd
	}
	sd.messages = append(sd.messages, msg)
	sd.timestamps = append(sd.timestamps, time.Now())
}

// All returns a copy of all stored messages for the current session.
func (s *InMemoryStore) All() []domain.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	sd, exists := s.sessions[s.sessionID]
	if !exists {
		return nil
	}
	result := make([]domain.Message, len(sd.messages))
	copy(result, sd.messages)
	return result
}

// Clear removes all messages from the current session.
func (s *InMemoryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sd, exists := s.sessions[s.sessionID]; exists {
		sd.messages = nil
	}
}

// SessionID returns the current session identifier.
func (s *InMemoryStore) SessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionID
}

// UserID returns the user identifier.
func (s *InMemoryStore) UserID() string { return s.userID }

// SetSession switches to the given session. Does not create a new session;
// the session will be materialized on the first message added.
func (s *InMemoryStore) SetSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionID = sessionID
	return nil
}

// ListSessions returns all known sessions for the user.
func (s *InMemoryStore) ListSessions(_ context.Context, _ string) ([]domain.SessionSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	summaries := make([]domain.SessionSummary, 0, len(s.sessions))
	for id, sd := range s.sessions {
		turnCount := 0
		for _, m := range sd.messages {
			if m.Role == domain.RoleUser {
				turnCount++
			}
		}
		summaries = append(summaries, domain.SessionSummary{
			ID:        id,
			Title:     sd.title,
			CreatedAt: sd.createdAt,
			TurnCount: turnCount,
		})
	}
	return summaries, nil
}

// LoadHistoryTurnsFromSession returns the conversation history for the given session as question/answer pairs.
func (s *InMemoryStore) LoadHistoryTurnsFromSession(_ context.Context, sessionID string) ([]domain.HistoryTurn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sd, exists := s.sessions[sessionID]
	if !exists {
		return nil, nil
	}
	var turns []domain.HistoryTurn
	for i := 0; i < len(sd.messages); i++ {
		if sd.messages[i].Role != domain.RoleUser {
			continue
		}
		turn := domain.HistoryTurn{Question: sd.messages[i].Content, At: sd.timestamps[i], TurnID: sd.messages[i].TurnID}
		// Find the last assistant message in this turn to capture the final
		// response after tool calls.
		for j := i + 1; j < len(sd.messages); j++ {
			if sd.messages[j].Role == domain.RoleUser {
				break
			}
			if sd.messages[j].Role == domain.RoleAssistant && sd.messages[j].Content != "" && len(sd.messages[j].ToolCalls) == 0 {
				turn.Answer = sd.messages[j].Content
				i = j
			}
		}
		turns = append(turns, turn)
	}
	return turns, nil
}

// DeleteSession removes a session and its data from memory.
func (s *InMemoryStore) DeleteSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	if sessionID == s.sessionID {
		s.sessionID = ""
	}
	return nil
}
