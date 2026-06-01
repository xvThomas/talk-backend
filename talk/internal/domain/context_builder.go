package domain

import "context"

// ContextBuilder builds the message context for an LLM call by reconciling
// in-memory messages with historical turns loaded from a SessionBrowser.
type ContextBuilder struct {
	store          MessageStore
	sessionBrowser SessionBrowser
	sessionID      string
	contextFull    int // -1 full, 0 lean, N hybrid
}

// NewContextBuilder creates a ContextBuilder.
func NewContextBuilder(store MessageStore, sessionBrowser SessionBrowser, sessionID string, contextFullTurns int) *ContextBuilder {
	return &ContextBuilder{
		store:          store,
		sessionBrowser: sessionBrowser,
		sessionID:      sessionID,
		contextFull:    contextFullTurns,
	}
}

// BuildContextMessages returns the messages that should be sent to the LLM for the given turn.
// When contextFull is negative or no session browser is configured, all in-memory messages are returned.
// Otherwise historical turns are loaded and merged with the detailed messages from the current session.
func (b *ContextBuilder) BuildContextMessages(ctx context.Context, currentTurnID string) []Message {
	allMessages := b.store.AllMessages(b.sessionID)
	if b.contextFull < 0 || b.sessionBrowser == nil {
		return allMessages
	}

	historyTurns, err := b.sessionBrowser.LoadHistoryTurnsFromSession(ctx, b.sessionID)
	if err != nil {
		// Fail open to keep the conversation functional when history loading fails.
		return allMessages
	}

	selectedDetailedTurnIDs := make(map[string]struct{})
	selectedDetailedTurnIDs[currentTurnID] = struct{}{}

	if b.contextFull > 0 {
		for _, turnID := range lastNTurnIDs(allMessages, currentTurnID, b.contextFull) {
			selectedDetailedTurnIDs[turnID] = struct{}{}
		}
	}

	messages := historyTurnsAsMessages(historyTurns, selectedDetailedTurnIDs, currentTurnID)
	for _, msg := range allMessages {
		if msg.TurnID == "" {
			messages = append(messages, msg)
			continue
		}
		if _, ok := selectedDetailedTurnIDs[msg.TurnID]; ok {
			messages = append(messages, msg)
		}
	}

	return messages
}

func historyTurnsAsMessages(turns []HistoryTurn, detailedTurnIDs map[string]struct{}, currentTurnID string) []Message {
	messages := make([]Message, 0, len(turns)*2)
	for _, turn := range turns {
		if turn.TurnID == currentTurnID {
			continue
		}
		if _, ok := detailedTurnIDs[turn.TurnID]; ok {
			continue
		}
		if turn.Question != "" {
			messages = append(messages, Message{Role: RoleUser, Content: turn.Question, TurnID: turn.TurnID})
		}
		if turn.Answer != "" {
			messages = append(messages, Message{Role: RoleAssistant, Content: turn.Answer, TurnID: turn.TurnID})
		}
	}
	return messages
}

func lastNTurnIDs(messages []Message, currentTurnID string, n int) []string {
	if n <= 0 {
		return nil
	}
	ordered := make([]string, 0)
	seen := make(map[string]struct{})
	for _, msg := range messages {
		if msg.Role != RoleUser || msg.TurnID == "" || msg.TurnID == currentTurnID {
			continue
		}
		if _, ok := seen[msg.TurnID]; ok {
			continue
		}
		seen[msg.TurnID] = struct{}{}
		ordered = append(ordered, msg.TurnID)
	}
	if len(ordered) <= n {
		return ordered
	}
	return ordered[len(ordered)-n:]
}
