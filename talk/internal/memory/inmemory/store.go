package inmemory

import (
	"context"
	"sync"
	"time"

	"github.com/xvThomas/talk-backend/talk/internal/domain"
)

// sessionData holds the messages and metadata for a single session.
type sessionData struct {
	messages   []domain.Message
	timestamps []time.Time
	history    []domain.HistoryTurn
	turnIndex  map[string]int
	title      string
	createdAt  time.Time
}

// core is the shared internal storage for both MessageRepository and SessionBrowser.
type core struct {
	mu       sync.Mutex
	sessions map[string]*sessionData
}

// MessageRepository implements domain.MessageStore backed by in-memory storage.
type MessageRepository struct{ *core }

// Browser implements domain.SessionBrowser backed by in-memory storage.
type Browser struct{ *core }

// New creates a pair of in-memory stores sharing the same underlying data.
func New() (*MessageRepository, *Browser) {
	c := &core{sessions: make(map[string]*sessionData)}
	return &MessageRepository{c}, &Browser{c}
}

var _ domain.MessageStore = (*MessageRepository)(nil)
var _ domain.MessageEventHandler = (*MessageRepository)(nil)
var _ domain.SessionBrowser = (*Browser)(nil)

// HandleMessageEvent appends a message to the given session.
// The session is materialized only when the first user message is added.
// The title is set from the first user message content.
func (r *MessageRepository) HandleMessageEvent(_ context.Context, event domain.MessageEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	msg := event.Message
	scope := event.SessionScope

	sd, exists := r.sessions[scope.SessionID()]
	if !exists {
		if msg.Role != domain.RoleUser {
			return nil
		}
		sd = &sessionData{title: msg.Content, createdAt: time.Now(), turnIndex: make(map[string]int)}
		r.sessions[scope.SessionID()] = sd
	}
	sd.messages = append(sd.messages, msg)
	sd.timestamps = append(sd.timestamps, time.Now())
	return nil
}

// HandleTurnEvent updates turn history for one completed turn.
func (r *MessageRepository) HandleTurnEvent(_ context.Context, event domain.TurnEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	sd, exists := r.sessions[event.SessionScope.SessionID()]
	if !exists || event.TurnID == "" {
		return nil
	}

	if sd.turnIndex == nil {
		sd.turnIndex = make(map[string]int)
	}

	turn := domain.HistoryTurn{
		TurnID:    event.TurnID,
		Question:  event.Input,
		Answer:    event.Output,
		At:        event.EndedAt,
		Model:     event.Model.Name,
		CallCount: event.CallCount,
	}

	if idx, ok := sd.turnIndex[event.TurnID]; ok {
		sd.history[idx] = turn
		return nil
	}

	sd.turnIndex[event.TurnID] = len(sd.history)
	sd.history = append(sd.history, turn)
	return nil
}

// HandleToolCallStart is a no-op for the in-memory store.
func (r *MessageRepository) HandleToolCallStart(_ context.Context, _ domain.ToolCallEvent) error {
	return nil
}

// HandleToolCallEnd is a no-op for the in-memory store.
func (r *MessageRepository) HandleToolCallEnd(_ context.Context, _ domain.ToolCallEndEvent) error {
	return nil
}

// AllMessages returns a copy of all stored messages for the given session.
func (r *MessageRepository) AllMessages(_ context.Context, sessionID string) ([]domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sd, exists := r.sessions[sessionID]
	if !exists {
		return nil, nil
	}
	result := make([]domain.Message, len(sd.messages))
	copy(result, sd.messages)
	return result, nil
}

// ClearMessages removes all messages from the given session.
func (r *MessageRepository) ClearMessages(_ context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sd, exists := r.sessions[sessionID]; exists {
		sd.messages = nil
	}
	return nil
}

// ListSessions returns all known sessions.
func (b *Browser) ListSessions(_ context.Context, _ string) ([]domain.SessionSummary, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	summaries := make([]domain.SessionSummary, 0, len(b.sessions))
	for id, sd := range b.sessions {
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
func (b *Browser) LoadHistoryTurnsFromSession(_ context.Context, sessionID string) ([]domain.HistoryTurn, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	sd, exists := b.sessions[sessionID]
	if !exists {
		return nil, nil
	}
	if len(sd.history) == 0 {
		return nil, nil
	}
	turns := make([]domain.HistoryTurn, len(sd.history))
	copy(turns, sd.history)
	return turns, nil
}

// DeleteSession removes a session and its data from memory.
func (b *Browser) DeleteSession(_ context.Context, sessionID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.sessions, sessionID)
	return nil
}
