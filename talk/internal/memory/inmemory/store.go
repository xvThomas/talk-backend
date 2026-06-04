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
var _ domain.SessionBrowser = (*Browser)(nil)

// AddMessage appends a message to the given session.
// The session is materialized only when the first user message is added.
// The title is set from the first user message content.
func (r *MessageRepository) AddMessage(msg domain.Message, scope domain.SessionScope) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sd, exists := r.sessions[scope.SessionID()]
	if !exists {
		if msg.Role != domain.RoleUser {
			return
		}
		sd = &sessionData{title: msg.Content, createdAt: time.Now()}
		r.sessions[scope.SessionID()] = sd
	}
	sd.messages = append(sd.messages, msg)
	sd.timestamps = append(sd.timestamps, time.Now())
}

// AllMessages returns a copy of all stored messages for the given session.
func (r *MessageRepository) AllMessages(sessionID string) []domain.Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	sd, exists := r.sessions[sessionID]
	if !exists {
		return nil
	}
	result := make([]domain.Message, len(sd.messages))
	copy(result, sd.messages)
	return result
}

// ClearMessages removes all messages from the given session.
func (r *MessageRepository) ClearMessages(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sd, exists := r.sessions[sessionID]; exists {
		sd.messages = nil
	}
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
			isCompletedReply := sd.messages[j].Role == domain.RoleAssistant &&
				sd.messages[j].Content != "" &&
				len(sd.messages[j].ToolCalls) == 0
			if isCompletedReply {
				turn.Answer = sd.messages[j].Content
				i = j
			}
		}
		turns = append(turns, turn)
	}
	return turns, nil
}

// DeleteSession removes a session and its data from memory.
func (b *Browser) DeleteSession(_ context.Context, sessionID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.sessions, sessionID)
	return nil
}
