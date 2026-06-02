package sqlite

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
)

var scope = domain.NewSessionScope("sess-1", "user1")

func newTestStore(t *testing.T) (*MessageRepository, *Browser, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	r, b, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cleanup := func() { _ = r.Close() }
	t.Cleanup(cleanup)
	return r, b, cleanup
}

func TestStore_AddAndAll(t *testing.T) {
	s, _, _ := newTestStore(t)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "hello"}, scope)
	s.AddMessage(domain.Message{Role: domain.RoleAssistant, Content: "world"}, scope)

	msgs := s.AllMessages(scope.SessionID())
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" || msgs[1].Content != "world" {
		t.Error("unexpected message contents")
	}
}

func TestStore_SessionNotMaterializedUntilUserMessage(t *testing.T) {
	s, b, _ := newTestStore(t)

	// Before any message, no sessions exist
	sessions, err := b.ListSessions(context.Background(), "user1")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}

	// Adding an assistant message should NOT materialize the session
	s.AddMessage(domain.Message{Role: domain.RoleAssistant, Content: "hi there"}, scope)
	sessions, _ = b.ListSessions(context.Background(), "user1")
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions after assistant msg, got %d", len(sessions))
	}

	// AllMessages should return nil for unmaterialized session
	msgs := s.AllMessages(scope.SessionID())
	if msgs != nil {
		t.Fatalf("expected nil messages for unmaterialized session, got %d", len(msgs))
	}

	// Adding a user message materializes the session
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "question"}, scope)
	sessions, _ = b.ListSessions(context.Background(), "user1")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session after user msg, got %d", len(sessions))
	}
}

func TestStore_TitleSetFromFirstUserMessage(t *testing.T) {
	s, b, _ := newTestStore(t)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "What is Go?"}, scope)
	s.AddMessage(domain.Message{Role: domain.RoleAssistant, Content: "A programming language."}, scope)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "Tell me more"}, scope)

	sessions, _ := b.ListSessions(context.Background(), "user1")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Title != "What is Go?" {
		t.Errorf("expected title %q, got %q", "What is Go?", sessions[0].Title)
	}
}

func TestStore_Clear(t *testing.T) {
	s, _, _ := newTestStore(t)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "hello"}, scope)
	s.AddMessage(domain.Message{Role: domain.RoleAssistant, Content: "hi"}, scope)

	s.ClearMessages(scope.SessionID())
	msgs := s.AllMessages(scope.SessionID())
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after clear, got %d", len(msgs))
	}
}

func TestStore_ClearUnmaterializedSession(t *testing.T) {
	s, _, _ := newTestStore(t)
	// Clear on unmaterialized session should not panic
	s.ClearMessages(scope.SessionID())
	msgs := s.AllMessages(scope.SessionID())
	if msgs != nil {
		t.Fatalf("expected nil, got %v", msgs)
	}
}

func TestStore_MultiSession(t *testing.T) {
	s, _, _ := newTestStore(t)
	scope2 := domain.NewSessionScope("sess-2", "user1")

	// Add messages to first session
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "q1"}, scope)
	s.AddMessage(domain.Message{Role: domain.RoleAssistant, Content: "a1"}, scope)

	// New session has no messages yet
	msgs := s.AllMessages(scope2.SessionID())
	if msgs != nil {
		t.Fatalf("expected nil messages for new session, got %d", len(msgs))
	}

	// Add message to second session
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "q2"}, scope2)
	msgs = s.AllMessages(scope2.SessionID())
	if len(msgs) != 1 || msgs[0].Content != "q2" {
		t.Errorf("unexpected messages in session 2: %v", msgs)
	}

	// First session still has its messages
	msgs = s.AllMessages(scope.SessionID())
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages in session 1, got %d", len(msgs))
	}
	if msgs[0].Content != "q1" {
		t.Errorf("expected first message %q, got %q", "q1", msgs[0].Content)
	}
}

func TestStore_ListSessionsMultiple(t *testing.T) {
	s, b, _ := newTestStore(t)
	scope2 := domain.NewSessionScope("sess-2", "user1")

	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "first"}, scope)
	s.AddMessage(domain.Message{Role: domain.RoleAssistant, Content: "reply1"}, scope)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "second"}, scope)

	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "hello sess2"}, scope2)

	sessions, err := b.ListSessions(context.Background(), "user1")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	for _, sess := range sessions {
		switch sess.ID {
		case "sess-1":
			if sess.TurnCount != 2 {
				t.Errorf("sess-1: expected 2 turns, got %d", sess.TurnCount)
			}
			if sess.Title != "first" {
				t.Errorf("sess-1: expected title %q, got %q", "first", sess.Title)
			}
		case "sess-2":
			if sess.TurnCount != 1 {
				t.Errorf("sess-2: expected 1 turn, got %d", sess.TurnCount)
			}
			if sess.Title != "hello sess2" {
				t.Errorf("sess-2: expected title %q, got %q", "hello sess2", sess.Title)
			}
		default:
			t.Errorf("unexpected session ID: %s", sess.ID)
		}
	}
}

func TestStore_ListSessionsFiltersByUserID(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	r, b, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	aliceScope := domain.NewSessionScope("sess-a", "alice")
	bobScope := domain.NewSessionScope("sess-b", "bob")

	r.AddMessage(domain.Message{Role: domain.RoleUser, Content: "alice msg"}, aliceScope)
	r.AddMessage(domain.Message{Role: domain.RoleUser, Content: "bob msg"}, bobScope)

	// Bob should only see his session
	sessions, _ := b.ListSessions(context.Background(), "bob")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session for bob, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-b" {
		t.Errorf("expected bob's session, got %s", sessions[0].ID)
	}

	// Alice should only see her session
	sessions, _ = b.ListSessions(context.Background(), "alice")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session for alice, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-a" {
		t.Errorf("expected alice's session, got %s", sessions[0].ID)
	}
}

func TestStore_ListSessionsCreatedAtIsSet(t *testing.T) {
	before := time.Now().Add(-time.Second)
	s, b, _ := newTestStore(t)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "hi"}, scope)
	after := time.Now().Add(time.Second)

	sessions, _ := b.ListSessions(context.Background(), "user1")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].CreatedAt.Before(before) || sessions[0].CreatedAt.After(after) {
		t.Errorf("createdAt %v not between %v and %v", sessions[0].CreatedAt, before, after)
	}
}

func TestStore_LoadSessionReturnsNilForUnknown(t *testing.T) {
	_, b, _ := newTestStore(t)
	turns, err := b.LoadHistoryTurnsFromSession(context.Background(), "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if turns != nil {
		t.Fatalf("expected nil turns for unknown session, got %d", len(turns))
	}
}

func TestStore_LoadSessionBuildsTurns(t *testing.T) {
	s, b, _ := newTestStore(t)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "q1"}, scope)
	s.AddMessage(domain.Message{Role: domain.RoleAssistant, Content: "a1"}, scope)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "q2"}, scope)
	s.AddMessage(domain.Message{Role: domain.RoleAssistant, Content: "a2"}, scope)

	turns, err := b.LoadHistoryTurnsFromSession(context.Background(), scope.SessionID())
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(turns))
	}
	if turns[0].Question != "q1" || turns[0].Answer != "a1" {
		t.Errorf("turn 0: got Q=%q A=%q", turns[0].Question, turns[0].Answer)
	}
	if turns[1].Question != "q2" || turns[1].Answer != "a2" {
		t.Errorf("turn 1: got Q=%q A=%q", turns[1].Question, turns[1].Answer)
	}
}

func TestStore_LoadSessionUserWithoutAnswer(t *testing.T) {
	s, b, _ := newTestStore(t)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "q1"}, scope)

	turns, _ := b.LoadHistoryTurnsFromSession(context.Background(), scope.SessionID())
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
	if turns[0].Question != "q1" || turns[0].Answer != "" {
		t.Errorf("turn 0: got Q=%q A=%q", turns[0].Question, turns[0].Answer)
	}
}

func TestStore_LoadSessionTimestampsAreSet(t *testing.T) {
	before := time.Now().Add(-time.Second)
	s, b, _ := newTestStore(t)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "q1"}, scope)
	s.AddMessage(domain.Message{Role: domain.RoleAssistant, Content: "a1"}, scope)
	after := time.Now().Add(time.Second)

	turns, _ := b.LoadHistoryTurnsFromSession(context.Background(), scope.SessionID())
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
	if turns[0].At.Before(before) || turns[0].At.After(after) {
		t.Errorf("turn timestamp %v not between %v and %v", turns[0].At, before, after)
	}
}

func TestStore_AllMessagesDoesNotShareState(t *testing.T) {
	s, _, _ := newTestStore(t)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "hello"}, scope)

	msgs := s.AllMessages(scope.SessionID())
	msgs[0].Content = "modified"

	original := s.AllMessages(scope.SessionID())
	if original[0].Content != "hello" {
		t.Error("AllMessages() did not return independent data; modification affected store")
	}
}

func TestStore_PersistenceAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Create store, add messages, close
	r1, _, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	r1.AddMessage(domain.Message{Role: domain.RoleUser, Content: "persistent question"}, scope)
	r1.AddMessage(domain.Message{Role: domain.RoleAssistant, Content: "persistent answer"}, scope)
	_ = r1.Close()

	// Reopen — messages should be available from disk
	r2, _, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r2.Close() }()

	msgs := r2.AllMessages(scope.SessionID())
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after reopen, got %d", len(msgs))
	}
	if msgs[0].Content != "persistent question" {
		t.Errorf("expected %q, got %q", "persistent question", msgs[0].Content)
	}
	if msgs[1].Content != "persistent answer" {
		t.Errorf("expected %q, got %q", "persistent answer", msgs[1].Content)
	}
}

func TestStore_PersistenceSessionsListAfterReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	r1, _, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	r1.AddMessage(domain.Message{Role: domain.RoleUser, Content: "q1"}, scope)
	_ = r1.Close()

	// Reopen — should still list the session
	_, b2, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b2.Close() }()

	sessions, _ := b2.ListSessions(context.Background(), "user1")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-1" {
		t.Errorf("expected %q, got %q", "sess-1", sessions[0].ID)
	}
	if sessions[0].Title != "q1" {
		t.Errorf("expected title %q, got %q", "q1", sessions[0].Title)
	}
}

func TestStore_NewInvalidPath(t *testing.T) {
	_, _, err := New(filepath.Join(string(os.PathSeparator), "nonexistent", "deeply", "nested", "test.db"))
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestStore_AddToolMessagePersistsToolMetadata(t *testing.T) {
	s, _, _ := newTestStore(t)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "q"}, scope)
	s.AddMessage(domain.Message{
		Role:    domain.RoleTool,
		Content: "get_weather",
		ToolCalls: []domain.ToolCall{{
			ID:    "call-1",
			Name:  "get_weather",
			Input: map[string]any{"city": "Paris"},
		}},
		ToolResults: []domain.ToolResult{{
			ToolCallID: "call-1",
			Content:    `{"temperature":"20C"}`,
		}},
	}, scope)

	var toolName, toolInput, toolOutput, toolCallID string
	err := s.conn.QueryRow(
		"SELECT tool_name, tool_input, tool_output, tool_call_id FROM messages WHERE session_id = ? AND role = ? ORDER BY id DESC LIMIT 1",
		scope.SessionID(),
		string(domain.RoleTool),
	).Scan(&toolName, &toolInput, &toolOutput, &toolCallID)
	if err != nil {
		t.Fatalf("query tool row: %v", err)
	}

	if toolName != "get_weather" {
		t.Fatalf("expected tool_name %q, got %q", "get_weather", toolName)
	}
	if toolCallID != "call-1" {
		t.Fatalf("expected tool_call_id %q, got %q", "call-1", toolCallID)
	}
	if toolOutput != `{"temperature":"20C"}` {
		t.Fatalf("unexpected tool_output: %q", toolOutput)
	}

	var input map[string]any
	if err := json.Unmarshal([]byte(toolInput), &input); err != nil {
		t.Fatalf("unmarshal tool_input: %v", err)
	}
	if input["city"] != "Paris" {
		t.Fatalf("expected city Paris, got %v", input["city"])
	}
}

func TestStore_AssistantToolCallsRoundTrip(t *testing.T) {
	s, _, _ := newTestStore(t)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "q"}, scope)
	s.AddMessage(domain.Message{
		Role: domain.RoleAssistant,
		ToolCalls: []domain.ToolCall{
			{ID: "call-1", Name: "tool1", Input: map[string]any{"x": 1}},
			{ID: "call-2", Name: "tool2", Input: map[string]any{"x": 2}},
		},
	}, scope)

	// Re-read from DB to verify round-trip
	msgs := s.AllMessages(scope.SessionID())
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after reload, got %d", len(msgs))
	}
	if len(msgs[1].ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls after reload, got %d", len(msgs[1].ToolCalls))
	}
	if msgs[1].ToolCalls[0].Name != "tool1" || msgs[1].ToolCalls[1].Name != "tool2" {
		t.Fatalf("unexpected tool call names after reload: %+v", msgs[1].ToolCalls)
	}
}

// --- Error and edge-case tests to improve coverage ---

func TestStore_DeleteSession(t *testing.T) {
	s, b, _ := newTestStore(t)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "hello", TurnID: "t1"}, scope)
	s.AddMessage(domain.Message{Role: domain.RoleAssistant, Content: "hi", TurnID: "t1"}, scope)

	err := b.DeleteSession(context.Background(), scope.SessionID())
	if err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	// Session and messages should be gone.
	sessions, _ := b.ListSessions(context.Background(), "user1")
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after delete, got %d", len(sessions))
	}
	msgs := s.AllMessages(scope.SessionID())
	if msgs != nil {
		t.Errorf("expected nil messages after delete, got %d", len(msgs))
	}
	// History turns should be gone too.
	turns, _ := b.LoadHistoryTurnsFromSession(context.Background(), scope.SessionID())
	if turns != nil {
		t.Errorf("expected nil turns after delete, got %d", len(turns))
	}
}

func TestStore_DeleteSession_NonExistent(t *testing.T) {
	_, b, _ := newTestStore(t)
	// Deleting a non-existent session should not error.
	err := b.DeleteSession(context.Background(), "does-not-exist")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestStore_AddMessage_AfterClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, _, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	// Operations on a closed DB should not panic (they silently fail).
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "hello"}, scope)
	msgs := s.AllMessages(scope.SessionID())
	if msgs != nil {
		t.Errorf("expected nil messages on closed DB, got %d", len(msgs))
	}
}

func TestStore_ClearMessages_AfterClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, _, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	// Should not panic.
	s.ClearMessages(scope.SessionID())
}

func TestStore_ListSessions_AfterClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	_, b, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = b.Close()

	_, err = b.ListSessions(context.Background(), "user1")
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestStore_LoadHistoryTurns_AfterClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	_, b, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = b.Close()

	_, err = b.LoadHistoryTurnsFromSession(context.Background(), "sess-1")
	if err == nil {
		t.Error("expected error on closed DB")
	}
}

func TestStore_HistoryTurn_UpsertOnMultipleMessages(t *testing.T) {
	s, b, _ := newTestStore(t)
	turnID := "turn-upsert-1"
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "question", TurnID: turnID}, scope)
	s.AddMessage(domain.Message{Role: domain.RoleAssistant, Content: "answer", TurnID: turnID}, scope)

	turns, err := b.LoadHistoryTurnsFromSession(context.Background(), scope.SessionID())
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
	if turns[0].Question != "question" || turns[0].Answer != "answer" {
		t.Errorf("got Q=%q A=%q", turns[0].Question, turns[0].Answer)
	}
}

func TestStore_HistoryTurn_ToolMessagesDoNotCreateTurns(t *testing.T) {
	s, b, _ := newTestStore(t)
	turnID := "turn-tool-1"
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "question", TurnID: turnID}, scope)
	// An assistant message WITH tool calls should not create a turn answer.
	s.AddMessage(domain.Message{
		Role:      domain.RoleAssistant,
		Content:   "calling tool",
		TurnID:    turnID,
		ToolCalls: []domain.ToolCall{{ID: "c1", Name: "t1", Input: map[string]any{}}},
	}, scope)
	// A tool result should not create a turn.
	s.AddMessage(domain.Message{
		Role:        domain.RoleTool,
		TurnID:      turnID,
		ToolResults: []domain.ToolResult{{ToolCallID: "c1", Content: "result"}},
	}, scope)
	// The final assistant answer without tool calls should update the turn.
	s.AddMessage(domain.Message{Role: domain.RoleAssistant, Content: "final answer", TurnID: turnID}, scope)

	turns, _ := b.LoadHistoryTurnsFromSession(context.Background(), scope.SessionID())
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
	if turns[0].Answer != "final answer" {
		t.Errorf("expected answer %q, got %q", "final answer", turns[0].Answer)
	}
}

func TestStore_ToolMessageWithoutToolCalls(t *testing.T) {
	s, _, _ := newTestStore(t)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "q"}, scope)
	// A tool message with only ToolResults (no ToolCalls) should still persist.
	s.AddMessage(domain.Message{
		Role:        domain.RoleTool,
		TurnID:      "t1",
		ToolResults: []domain.ToolResult{{ToolCallID: "call-99", Content: "output"}},
	}, scope)

	msgs := s.AllMessages(scope.SessionID())
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	toolMsg := msgs[1]
	if len(toolMsg.ToolResults) != 1 || toolMsg.ToolResults[0].Content != "output" {
		t.Errorf("unexpected tool results: %+v", toolMsg.ToolResults)
	}
}

func TestStore_EmptyContentMessages(t *testing.T) {
	s, _, _ := newTestStore(t)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "start"}, scope)
	// Assistant message with empty content but tool calls should be stored.
	s.AddMessage(domain.Message{
		Role:      domain.RoleAssistant,
		Content:   "",
		ToolCalls: []domain.ToolCall{{ID: "c1", Name: "tool1", Input: map[string]any{"a": "b"}}},
	}, scope)

	msgs := s.AllMessages(scope.SessionID())
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if len(msgs[1].ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(msgs[1].ToolCalls))
	}
}

func TestStore_LargeContent(t *testing.T) {
	s, _, _ := newTestStore(t)
	// Store a large message to verify no truncation.
	large := make([]byte, 100_000)
	for i := range large {
		large[i] = 'A' + byte(i%26)
	}
	content := string(large)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: content}, scope)
	msgs := s.AllMessages(scope.SessionID())
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != content {
		t.Error("large content was truncated or corrupted")
	}
}

func TestStore_TurnIDPersisted(t *testing.T) {
	s, _, _ := newTestStore(t)
	s.AddMessage(domain.Message{Role: domain.RoleUser, Content: "q", TurnID: "turn-abc"}, scope)
	msgs := s.AllMessages(scope.SessionID())
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].TurnID != "turn-abc" {
		t.Errorf("expected TurnID %q, got %q", "turn-abc", msgs[0].TurnID)
	}
}
