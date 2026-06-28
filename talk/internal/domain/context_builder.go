package domain

import "context"

// ContextBuilder builds the message context for an LLM call by reconciling
// in-memory messages with historical turns loaded from a SessionBrowser.
type ContextBuilder struct {
	messageStore   MessageStore
	sessionBrowser SessionBrowser
	sessionID      string
	contextFull    int // -1 full, 0 lean, N hybrid
}

// NewContextBuilder creates a ContextBuilder.
func NewContextBuilder(messageStore MessageStore, sessionBrowser SessionBrowser, sessionID string, contextFullTurns int) *ContextBuilder {
	return &ContextBuilder{
		messageStore:   messageStore,
		sessionBrowser: sessionBrowser,
		sessionID:      sessionID,
		contextFull:    contextFullTurns,
	}
}

// BuildContextMessages returns the messages that should be sent to the LLM for the given turn.
// When contextFull is negative or no session browser is configured, all in-memory messages are returned.
// Otherwise historical turns are loaded and merged with the detailed messages from the current session.
func (b *ContextBuilder) BuildContextMessages(ctx context.Context, currentTurnID string) []Message {
	allMessages, err := b.messageStore.AllMessages(ctx, b.sessionID)
	if err != nil {
		// Fail open to keep the conversation functional when store read fails.
		return nil
	}
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

	// Force-include incomplete turns in detail regardless of context mode.
	for _, turn := range historyTurns {
		if turn.Status == TurnStatusIncomplete {
			selectedDetailedTurnIDs[turn.TurnID] = struct{}{}
		}
	}

	messages := historyTurnsAsMessages(historyTurns, selectedDetailedTurnIDs, currentTurnID)

	// Build a set of turn IDs that actually have detailed messages available.
	availableDetailedTurnIDs := make(map[string]struct{})
	for _, msg := range allMessages {
		if msg.TurnID == "" {
			continue
		}
		if _, ok := selectedDetailedTurnIDs[msg.TurnID]; ok {
			availableDetailedTurnIDs[msg.TurnID] = struct{}{}
		}
	}

	// Fallback: if a turn was selected for detail but has no messages in store,
	// include it as a summary so context is not silently lost.
	for turnID := range selectedDetailedTurnIDs {
		if turnID == currentTurnID {
			continue
		}
		if _, ok := availableDetailedTurnIDs[turnID]; ok {
			continue
		}
		// No detailed messages found — emit summary from history turn as fallback.
		for _, turn := range historyTurns {
			if turn.TurnID == turnID {
				if turn.Question != "" {
					messages = append(messages, Message{Role: RoleUser, Content: turn.Question, TurnID: turn.TurnID})
				}
				if turn.Answer != "" {
					messages = append(messages, Message{Role: RoleAssistant, Content: turn.Answer, TurnID: turn.TurnID})
				}
				break
			}
		}
	}

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
