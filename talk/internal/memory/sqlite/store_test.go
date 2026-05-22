package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := NewStore(dbPath, "sess-1", "user1")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStore_AddAndAll(t *testing.T) {
	s := newTestStore(t)
	s.Add(domain.Message{Role: domain.RoleUser, Content: "hello"})
	s.Add(domain.Message{Role: domain.RoleAssistant, Content: "world"})

	msgs := s.All()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" || msgs[1].Content != "world" {
		t.Error("unexpected message contents")
	}
}

func TestStore_SessionNotMaterializedUntilUserMessage(t *testing.T) {
	s := newTestStore(t)

	// Before any message, no sessions exist
	sessions, err := s.ListSessions(context.Background(), "user1")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}

	// Adding an assistant message should NOT materialize the session
	s.Add(domain.Message{Role: domain.RoleAssistant, Content: "hi there"})
	sessions, _ = s.ListSessions(context.Background(), "user1")
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions after assistant msg, got %d", len(sessions))
	}

	// All should return nil for unmaterialized session
	msgs := s.All()
	if msgs != nil {
		t.Fatalf("expected nil messages for unmaterialized session, got %d", len(msgs))
	}

	// Adding a user message materializes the session
	s.Add(domain.Message{Role: domain.RoleUser, Content: "question"})
	sessions, _ = s.ListSessions(context.Background(), "user1")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session after user msg, got %d", len(sessions))
	}
}

func TestStore_TitleSetFromFirstUserMessage(t *testing.T) {
	s := newTestStore(t)
	s.Add(domain.Message{Role: domain.RoleUser, Content: "What is Go?"})
	s.Add(domain.Message{Role: domain.RoleAssistant, Content: "A programming language."})
	s.Add(domain.Message{Role: domain.RoleUser, Content: "Tell me more"})

	sessions, _ := s.ListSessions(context.Background(), "user1")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Title != "What is Go?" {
		t.Errorf("expected title %q, got %q", "What is Go?", sessions[0].Title)
	}
}

func TestStore_Clear(t *testing.T) {
	s := newTestStore(t)
	s.Add(domain.Message{Role: domain.RoleUser, Content: "hello"})
	s.Add(domain.Message{Role: domain.RoleAssistant, Content: "hi"})

	s.Clear()
	msgs := s.All()
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after clear, got %d", len(msgs))
	}
}

func TestStore_ClearUnmaterializedSession(t *testing.T) {
	s := newTestStore(t)
	// Clear on unmaterialized session should not panic
	s.Clear()
	msgs := s.All()
	if msgs != nil {
		t.Fatalf("expected nil, got %v", msgs)
	}
}

func TestStore_SessionIDAndUserID(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(filepath.Join(dir, "test.db"), "sess-42", "bob")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if s.SessionID() != "sess-42" {
		t.Errorf("expected session ID %q, got %q", "sess-42", s.SessionID())
	}
	if s.UserID() != "bob" {
		t.Errorf("expected user ID %q, got %q", "bob", s.UserID())
	}
}

func TestStore_SetSessionSwitches(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Add messages to first session
	s.Add(domain.Message{Role: domain.RoleUser, Content: "q1"})
	s.Add(domain.Message{Role: domain.RoleAssistant, Content: "a1"})

	// Switch to a new session
	if err := s.SetSession(ctx, "sess-2"); err != nil {
		t.Fatal(err)
	}
	if s.SessionID() != "sess-2" {
		t.Errorf("expected session ID %q, got %q", "sess-2", s.SessionID())
	}

	// New session has no messages yet
	msgs := s.All()
	if msgs != nil {
		t.Fatalf("expected nil messages for new session, got %d", len(msgs))
	}

	// Add message to second session
	s.Add(domain.Message{Role: domain.RoleUser, Content: "q2"})
	msgs = s.All()
	if len(msgs) != 1 || msgs[0].Content != "q2" {
		t.Errorf("unexpected messages in session 2: %v", msgs)
	}

	// Switch back to first session — messages should reload from DB
	if err := s.SetSession(ctx, "sess-1"); err != nil {
		t.Fatal(err)
	}
	msgs = s.All()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages in session 1, got %d", len(msgs))
	}
	if msgs[0].Content != "q1" {
		t.Errorf("expected first message %q, got %q", "q1", msgs[0].Content)
	}
}

func TestStore_ListSessionsMultiple(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.Add(domain.Message{Role: domain.RoleUser, Content: "first"})
	s.Add(domain.Message{Role: domain.RoleAssistant, Content: "reply1"})
	s.Add(domain.Message{Role: domain.RoleUser, Content: "second"})

	_ = s.SetSession(ctx, "sess-2")
	s.Add(domain.Message{Role: domain.RoleUser, Content: "hello sess2"})

	sessions, err := s.ListSessions(ctx, "user1")
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

	s1, err := NewStore(dbPath, "sess-a", "alice")
	if err != nil {
		t.Fatal(err)
	}
	s1.Add(domain.Message{Role: domain.RoleUser, Content: "alice msg"})
	_ = s1.Close()

	s2, err := NewStore(dbPath, "sess-b", "bob")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	s2.Add(domain.Message{Role: domain.RoleUser, Content: "bob msg"})

	// Bob should only see his session
	sessions, _ := s2.ListSessions(context.Background(), "bob")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session for bob, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-b" {
		t.Errorf("expected bob's session, got %s", sessions[0].ID)
	}

	// Alice should only see her session
	sessions, _ = s2.ListSessions(context.Background(), "alice")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session for alice, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-a" {
		t.Errorf("expected alice's session, got %s", sessions[0].ID)
	}
}

func TestStore_ListSessionsCreatedAtIsSet(t *testing.T) {
	before := time.Now().Add(-time.Second)
	s := newTestStore(t)
	s.Add(domain.Message{Role: domain.RoleUser, Content: "hi"})
	after := time.Now().Add(time.Second)

	sessions, _ := s.ListSessions(context.Background(), "user1")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].CreatedAt.Before(before) || sessions[0].CreatedAt.After(after) {
		t.Errorf("createdAt %v not between %v and %v", sessions[0].CreatedAt, before, after)
	}
}

func TestStore_LoadSessionReturnsNilForUnknown(t *testing.T) {
	s := newTestStore(t)
	turns, err := s.LoadSession(context.Background(), "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if turns != nil {
		t.Fatalf("expected nil turns for unknown session, got %d", len(turns))
	}
}

func TestStore_LoadSessionBuildsTurns(t *testing.T) {
	s := newTestStore(t)
	s.Add(domain.Message{Role: domain.RoleUser, Content: "q1"})
	s.Add(domain.Message{Role: domain.RoleAssistant, Content: "a1"})
	s.Add(domain.Message{Role: domain.RoleUser, Content: "q2"})
	s.Add(domain.Message{Role: domain.RoleAssistant, Content: "a2"})

	turns, err := s.LoadSession(context.Background(), "sess-1")
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
	s := newTestStore(t)
	s.Add(domain.Message{Role: domain.RoleUser, Content: "q1"})

	turns, _ := s.LoadSession(context.Background(), "sess-1")
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
	if turns[0].Question != "q1" || turns[0].Answer != "" {
		t.Errorf("turn 0: got Q=%q A=%q", turns[0].Question, turns[0].Answer)
	}
}

func TestStore_LoadSessionTimestampsAreSet(t *testing.T) {
	before := time.Now().Add(-time.Second)
	s := newTestStore(t)
	s.Add(domain.Message{Role: domain.RoleUser, Content: "q1"})
	s.Add(domain.Message{Role: domain.RoleAssistant, Content: "a1"})
	after := time.Now().Add(time.Second)

	turns, _ := s.LoadSession(context.Background(), "sess-1")
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
	if turns[0].At.Before(before) || turns[0].At.After(after) {
		t.Errorf("turn timestamp %v not between %v and %v", turns[0].At, before, after)
	}
}

func TestStore_AllReturnsCopy(t *testing.T) {
	s := newTestStore(t)
	s.Add(domain.Message{Role: domain.RoleUser, Content: "hello"})

	msgs := s.All()
	msgs[0].Content = "modified"

	original := s.All()
	if original[0].Content != "hello" {
		t.Error("All() did not return a copy; modification affected store")
	}
}

func TestStore_SetSessionDoesNotMaterialize(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.Add(domain.Message{Role: domain.RoleUser, Content: "msg"})

	// Switch to new session without sending a message
	_ = s.SetSession(ctx, "sess-new")

	sessions, _ := s.ListSessions(ctx, "user1")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 materialized session, got %d", len(sessions))
	}
	if sessions[0].ID != "sess-1" {
		t.Errorf("expected session ID %q, got %q", "sess-1", sessions[0].ID)
	}
}

func TestStore_PersistenceAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Create store, add messages, close
	s1, err := NewStore(dbPath, "sess-1", "user1")
	if err != nil {
		t.Fatal(err)
	}
	s1.Add(domain.Message{Role: domain.RoleUser, Content: "persistent question"})
	s1.Add(domain.Message{Role: domain.RoleAssistant, Content: "persistent answer"})
	_ = s1.Close()

	// Reopen with same session — messages should be loaded from disk
	s2, err := NewStore(dbPath, "sess-1", "user1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()

	msgs := s2.All()
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

	s1, err := NewStore(dbPath, "sess-1", "user1")
	if err != nil {
		t.Fatal(err)
	}
	s1.Add(domain.Message{Role: domain.RoleUser, Content: "q1"})
	_ = s1.Close()

	// Reopen with different session — should still list the first one
	s2, err := NewStore(dbPath, "sess-2", "user1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()

	sessions, _ := s2.ListSessions(context.Background(), "user1")
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

func TestStore_NewStoreInvalidPath(t *testing.T) {
	_, err := NewStore(filepath.Join(string(os.PathSeparator), "nonexistent", "deeply", "nested", "test.db"), "s", "u")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}
