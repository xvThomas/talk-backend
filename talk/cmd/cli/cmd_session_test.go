package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
)

func TestCmdMemory_NoSessionBrowser(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	// fakeStore does not implement SessionBrowser

	app.cmdMemory(context.Background())

	out := p.Output()
	if !strings.Contains(out, "session history not available") {
		t.Errorf("expected 'session history not available', got: %s", out)
	}
}

func TestCmdMemory_EmptyHistory(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	app.Store = store

	app.cmdMemory(context.Background())

	out := p.Output()
	if !strings.Contains(out, "no history for current session") {
		t.Errorf("expected 'no history' message, got: %s", out)
	}
}

func TestCmdMemory_ShowsTurns(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.turns[store.sessionID] = []domain.HistoryTurn{
		{Question: "Hello", Answer: "Hi there!", At: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)},
	}
	app.Store = store

	app.cmdMemory(context.Background())

	out := p.Output()
	if !strings.Contains(out, "Hello") {
		t.Errorf("expected question in output, got: %s", out)
	}
	if !strings.Contains(out, "Hi there!") {
		t.Errorf("expected answer in output, got: %s", out)
	}
	if !strings.Contains(out, "Turn 1") {
		t.Errorf("expected 'Turn 1' in output, got: %s", out)
	}
}

func TestCmdMemory_ShowsTitleAndMultipleCalls(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: store.sessionID, Title: "My Topic"},
	}
	store.turns[store.sessionID] = []domain.HistoryTurn{
		{Question: "Q1", Answer: "A1", At: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC), TurnID: "turn-1", CallCount: 3},
	}
	app.Store = store

	app.cmdMemory(context.Background())

	out := p.Output()
	if !strings.Contains(out, "My Topic") {
		t.Errorf("expected session title 'My Topic', got: %s", out)
	}
	if !strings.Contains(out, "3 LLM calls") {
		t.Errorf("expected '3 LLM calls', got: %s", out)
	}
	if !strings.Contains(out, "turn-1") {
		t.Errorf("expected turnID 'turn-1', got: %s", out)
	}
}

func TestCmdSessions_ShowsCurrentMarker(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: store.sessionID, Title: "Current", CreatedAt: time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC), TurnCount: 2},
		{ID: "other-0000-0000-0000-000000000000", Title: "", CreatedAt: time.Date(2025, 3, 2, 10, 0, 0, 0, time.UTC)},
	}
	app.Store = store

	app.cmdSessions(context.Background())

	out := p.Output()
	if !strings.Contains(out, "← current") {
		t.Errorf("expected '← current' marker, got: %s", out)
	}
	if !strings.Contains(out, "(untitled)") {
		t.Errorf("expected '(untitled)' fallback, got: %s", out)
	}
}

func TestCmdSessions_NoSessionBrowser(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)

	app.cmdSessions(context.Background())

	out := p.Output()
	if !strings.Contains(out, "sessions not available") {
		t.Errorf("expected 'sessions not available', got: %s", out)
	}
}

func TestCmdSessions_Empty(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	app.Store = newFakeSessionStore()

	app.cmdSessions(context.Background())

	out := p.Output()
	if !strings.Contains(out, "(no sessions)") {
		t.Errorf("expected '(no sessions)' message, got: %s", out)
	}
}

func TestCmdSessions_ListsSessions(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: "aaaa1111-0000-0000-0000-000000000000", Title: "First chat", CreatedAt: time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC), TurnCount: 3},
		{ID: "bbbb2222-0000-0000-0000-000000000000", Title: "Second chat", CreatedAt: time.Date(2025, 3, 2, 10, 0, 0, 0, time.UTC), TurnCount: 1},
	}
	app.Store = store

	app.cmdSessions(context.Background())

	out := p.Output()
	if !strings.Contains(out, "First chat") {
		t.Errorf("expected 'First chat' in output, got: %s", out)
	}
	if !strings.Contains(out, "Second chat") {
		t.Errorf("expected 'Second chat' in output, got: %s", out)
	}
}

func TestCmdSession_SwitchByPrefix(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: "deadbeef-0000-0000-0000-000000000000", Title: "Old session"},
	}
	app.Store = store

	app.cmdSession(context.Background(), "dead")

	out := p.Output()
	if !strings.Contains(out, "Switched to session") {
		t.Errorf("expected switch confirmation, got: %s", out)
	}
	if store.sessionID != "deadbeef-0000-0000-0000-000000000000" {
		t.Errorf("expected session to be set to deadbeef..., got: %s", store.sessionID)
	}
}

func TestCmdSession_PrefixNotFound(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: "deadbeef-0000-0000-0000-000000000000"},
	}
	app.Store = store

	app.cmdSession(context.Background(), "xxxx")

	out := p.Output()
	if !strings.Contains(out, "No session found matching") {
		t.Errorf("expected 'No session found' message, got: %s", out)
	}
}

func TestCmdSession_InteractiveNew(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: "aaaa0000-0000-0000-0000-000000000000", Title: "old"},
	}
	app.Store = store
	app.LR = newScriptReader("new")

	app.cmdSession(context.Background(), "")

	out := p.Output()
	if !strings.Contains(out, "New session created") {
		t.Errorf("expected 'New session created', got: %s", out)
	}
	if store.sessionID == "abcd1234-0000-0000-0000-000000000000" {
		t.Error("expected sessionID to change from original")
	}
}

func TestCmdSession_InteractiveNumericChoice(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: "aaaa0000-0000-0000-0000-000000000000", Title: "first"},
		{ID: "bbbb0000-0000-0000-0000-000000000000", Title: "second"},
	}
	app.Store = store
	app.LR = newScriptReader("2")

	app.cmdSession(context.Background(), "")

	out := p.Output()
	if !strings.Contains(out, "Switched to session") {
		t.Errorf("expected 'Switched to session', got: %s", out)
	}
	if store.sessionID != "bbbb0000-0000-0000-0000-000000000000" {
		t.Errorf("expected session switched to bbbb..., got: %s", store.sessionID)
	}
}

func TestCmdSession_InteractiveInvalidChoice(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: "aaaa0000-0000-0000-0000-000000000000"},
	}
	app.Store = store
	app.LR = newScriptReader("99")

	app.cmdSession(context.Background(), "")

	out := p.Output()
	if !strings.Contains(out, "Invalid choice") {
		t.Errorf("expected 'Invalid choice', got: %s", out)
	}
}

func TestCmdSession_InteractiveCancel(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: "aaaa0000-0000-0000-0000-000000000000"},
	}
	app.Store = store
	app.LR = newScriptReader("") // empty → cancel

	app.cmdSession(context.Background(), "")

	out := p.Output()
	// Should show sessions menu but not switch
	if strings.Contains(out, "Switched") {
		t.Error("expected no switch on cancel")
	}
}

func TestShortID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abcdefgh-1234", "abcdefgh…"},
		{"short", "short"},
		{"12345678", "12345678"},
		{"123456789", "12345678…"},
	}
	for _, tt := range tests {
		got := shortID(tt.input)
		if got != tt.want {
			t.Errorf("shortID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCmdSession_InteractiveEmptySessions(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = nil
	app.Store = store
	app.LR = newScriptReader("new")

	app.cmdSession(context.Background(), "")

	out := p.Output()
	if !strings.Contains(out, "no past sessions found") {
		t.Errorf("expected 'no past sessions found', got: %s", out)
	}
	if !strings.Contains(out, "New session created") {
		t.Errorf("expected 'New session created', got: %s", out)
	}
}
