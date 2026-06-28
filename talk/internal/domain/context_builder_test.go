package domain

import (
	"context"
	"testing"
)

// stubMessageStore returns pre-configured messages for tests.
type stubMessageStore struct {
	messages []Message
}

func (s *stubMessageStore) AllMessages(_ context.Context, _ string) ([]Message, error) {
	return s.messages, nil
}

func (s *stubMessageStore) ClearMessages(_ context.Context, _ string) error { return nil }

// stubSessionBrowser returns pre-configured history turns for tests.
type stubSessionBrowser struct {
	turns []HistoryTurn
}

func (s *stubSessionBrowser) ListSessions(_ context.Context, _ string) ([]SessionSummary, error) {
	return nil, nil
}

func (s *stubSessionBrowser) LoadHistoryTurnsFromSession(_ context.Context, _ string) ([]HistoryTurn, error) {
	return s.turns, nil
}

func (s *stubSessionBrowser) DeleteSession(_ context.Context, _ string) error { return nil }

func TestContextBuilder_IncompleteTurnForceIncluded_LeanMode(t *testing.T) {
	store := &stubMessageStore{
		messages: []Message{
			{Role: RoleUser, Content: "do stuff", TurnID: "t-incomplete"},
			{Role: RoleAssistant, Content: "calling tool", TurnID: "t-incomplete"},
			{Role: RoleTool, Content: "tool result", TurnID: "t-incomplete"},
			{Role: RoleUser, Content: "next question", TurnID: "t-current"},
		},
	}
	browser := &stubSessionBrowser{
		turns: []HistoryTurn{
			{TurnID: "t-incomplete", Question: "do stuff", Answer: "", Status: TurnStatusIncomplete},
		},
	}

	// contextFull=0 is lean mode — normally summarizes all turns
	cb := NewContextBuilder(store, browser, "sess1", 0)
	msgs := cb.BuildContextMessages(context.Background(), "t-current")

	// The incomplete turn's messages should be force-included in detail.
	found := false
	for _, m := range msgs {
		if m.TurnID == "t-incomplete" && m.Role == RoleTool {
			found = true
			break
		}
	}
	if !found {
		t.Error("incomplete turn messages not force-included in lean mode")
	}
}

func TestBuildContextMessages_IncompleteTurnFallback_NoMessages(t *testing.T) {
	// Simulate: incomplete turn exists in history but messages are gone from store.
	store := &stubMessageStore{
		messages: []Message{
			{Role: RoleUser, Content: "new question", TurnID: "t-current"},
		},
	}
	browser := &stubSessionBrowser{
		turns: []HistoryTurn{
			{TurnID: "t-lost", Question: "old question", Answer: "", Status: TurnStatusIncomplete},
		},
	}

	cb := NewContextBuilder(store, browser, "sess1", 0)
	msgs := cb.BuildContextMessages(context.Background(), "t-current")

	// Should fallback to summary from history turn.
	found := false
	for _, m := range msgs {
		if m.TurnID == "t-lost" && m.Content == "old question" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected fallback summary for incomplete turn with missing messages")
	}
}

func TestContextBuilder_CompleteTurnSummarized_LeanMode(t *testing.T) {
	store := &stubMessageStore{
		messages: []Message{
			{Role: RoleUser, Content: "detailed q", TurnID: "t-old"},
			{Role: RoleAssistant, Content: "detailed a", TurnID: "t-old"},
			{Role: RoleUser, Content: "current q", TurnID: "t-current"},
		},
	}
	browser := &stubSessionBrowser{
		turns: []HistoryTurn{
			{TurnID: "t-old", Question: "detailed q", Answer: "detailed a", Status: TurnStatusComplete},
		},
	}

	cb := NewContextBuilder(store, browser, "sess1", 0)
	msgs := cb.BuildContextMessages(context.Background(), "t-current")

	// In lean mode, completed turn should be summarized (appear from history, not from store messages).
	for _, m := range msgs {
		if m.TurnID == "t-old" && m.Role == RoleAssistant && m.Content == "detailed a" {
			// This is the summary from historyTurnsAsMessages — expected.
			return
		}
	}
	t.Error("expected complete turn to appear as summary in lean mode")
}
