package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
)

// --- cmdMemory tests ---

func TestCmdMemory_NoSessionBrowser(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)

	app.cmdMemory(context.Background())

	out := p.Output()
	if !strings.Contains(out, "session history not available") {
		t.Errorf("expected 'session history not available', got: %s", out)
	}
}

func TestCmdMemory_EmptyHistory(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	app.Store = newFakeSessionStore()

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

// --- cmdSession dispatcher tests ---

func TestCmdSession_DefaultIsList(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: store.sessionID, Title: "Chat", CreatedAt: time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC), TurnCount: 2},
	}
	app.Store = store
	app.LR = newScriptReader("") // cancel

	app.cmdSession(context.Background(), "")

	out := p.Output()
	if !strings.Contains(out, "Sessions:") {
		t.Errorf("expected session list, got: %s", out)
	}
}

func TestCmdSession_UnknownSubcommand(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	app.Store = newFakeSessionStore()

	app.cmdSession(context.Background(), "foo")

	out := p.Output()
	if !strings.Contains(out, "Unknown /session subcommand") {
		t.Errorf("expected unknown subcommand message, got: %s", out)
	}
}

// --- cmdSessionList tests ---

func TestCmdSessionList_NoSessionBrowser(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)

	app.cmdSessionList(context.Background())

	out := p.Output()
	if !strings.Contains(out, "session management not available") {
		t.Errorf("expected 'session management not available', got: %s", out)
	}
}

func TestCmdSessionList_Empty(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	app.Store = newFakeSessionStore()
	app.LR = newScriptReader("")

	app.cmdSessionList(context.Background())

	out := p.Output()
	if !strings.Contains(out, "no past sessions found") {
		t.Errorf("expected 'no past sessions found', got: %s", out)
	}
}

func TestCmdSessionList_ShowsSessionsWithTitleAndTurns(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: store.sessionID, Title: "My Chat", CreatedAt: time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC), TurnCount: 5},
		{ID: "other-0000-0000-0000-000000000000", Title: "", CreatedAt: time.Date(2025, 3, 2, 10, 0, 0, 0, time.UTC), TurnCount: 1},
	}
	app.Store = store
	app.LR = newScriptReader("")

	app.cmdSessionList(context.Background())

	out := p.Output()
	if !strings.Contains(out, "My Chat") {
		t.Errorf("expected title 'My Chat', got: %s", out)
	}
	if !strings.Contains(out, "(untitled)") {
		t.Errorf("expected '(untitled)' fallback, got: %s", out)
	}
	if !strings.Contains(out, "5 turns") {
		t.Errorf("expected '5 turns', got: %s", out)
	}
	if !strings.Contains(out, "\xe2\x86\x90 current") {
		t.Errorf("expected current marker, got: %s", out)
	}
}

func TestCmdSessionList_SwitchByNumber(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: "aaaa0000-0000-0000-0000-000000000000", Title: "first"},
		{ID: "bbbb0000-0000-0000-0000-000000000000", Title: "second"},
	}
	app.Store = store
	app.LR = newScriptReader("2")

	app.cmdSessionList(context.Background())

	out := p.Output()
	if !strings.Contains(out, "Switched to session") {
		t.Errorf("expected 'Switched to session', got: %s", out)
	}
	if store.sessionID != "bbbb0000-0000-0000-0000-000000000000" {
		t.Errorf("expected session bbbb..., got: %s", store.sessionID)
	}
}

func TestCmdSessionList_NewFromMenu(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: "aaaa0000-0000-0000-0000-000000000000", Title: "old"},
	}
	app.Store = store
	app.LR = newScriptReader("new")

	app.cmdSessionList(context.Background())

	out := p.Output()
	if !strings.Contains(out, "New session created") {
		t.Errorf("expected 'New session created', got: %s", out)
	}
}

func TestCmdSessionList_InvalidChoice(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: "aaaa0000-0000-0000-0000-000000000000"},
	}
	app.Store = store
	app.LR = newScriptReader("99")

	app.cmdSessionList(context.Background())

	out := p.Output()
	if !strings.Contains(out, "Invalid choice") {
		t.Errorf("expected 'Invalid choice', got: %s", out)
	}
}

func TestCmdSessionList_Cancel(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: "aaaa0000-0000-0000-0000-000000000000"},
	}
	app.Store = store
	app.LR = newScriptReader("")

	app.cmdSessionList(context.Background())

	out := p.Output()
	if strings.Contains(out, "Switched") {
		t.Error("expected no switch on cancel")
	}
}

// --- cmdSessionNew tests ---

func TestCmdSessionNew_NoSessionBrowser(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)

	app.cmdSessionNew(context.Background())

	out := p.Output()
	if !strings.Contains(out, "session management not available") {
		t.Errorf("expected 'session management not available', got: %s", out)
	}
}

func TestCmdSessionNew_CreatesSession(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	app.Store = store

	origID := store.sessionID
	app.cmdSessionNew(context.Background())

	out := p.Output()
	if !strings.Contains(out, "New session created") {
		t.Errorf("expected 'New session created', got: %s", out)
	}
	if store.sessionID == origID {
		t.Error("expected sessionID to change")
	}
}

// --- cmdSessionRemove tests ---

func TestCmdSessionRemove_NoSessionBrowser(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)

	app.cmdSessionRemove(context.Background())

	out := p.Output()
	if !strings.Contains(out, "session management not available") {
		t.Errorf("expected 'session management not available', got: %s", out)
	}
}

func TestCmdSessionRemove_EmptyList(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	app.Store = newFakeSessionStore()

	app.cmdSessionRemove(context.Background())

	out := p.Output()
	if !strings.Contains(out, "no sessions to remove") {
		t.Errorf("expected 'no sessions to remove', got: %s", out)
	}
}

func TestCmdSessionRemove_RemovesSession(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: store.sessionID, Title: "current one", CreatedAt: time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC), TurnCount: 2},
		{ID: "other-0000-0000-0000-000000000000", Title: "other one", CreatedAt: time.Date(2025, 3, 2, 10, 0, 0, 0, time.UTC), TurnCount: 1},
	}
	app.Store = store
	app.LR = newScriptReader("2") // select the non-current session

	app.cmdSessionRemove(context.Background())

	out := p.Output()
	if !strings.Contains(out, "Removed session") {
		t.Errorf("expected 'Removed session', got: %s", out)
	}
	if len(store.deleted) != 1 || store.deleted[0] != "other-0000-0000-0000-000000000000" {
		t.Errorf("expected other session to be deleted, got: %v", store.deleted)
	}
}

func TestCmdSessionRemove_CannotRemoveCurrent(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: store.sessionID, Title: "current one"},
		{ID: "other-0000-0000-0000-000000000000", Title: "other one"},
	}
	app.Store = store
	app.LR = newScriptReader("1") // select the current session

	app.cmdSessionRemove(context.Background())

	out := p.Output()
	if !strings.Contains(out, "Cannot remove the current session") {
		t.Errorf("expected cannot-remove message, got: %s", out)
	}
	if len(store.deleted) != 0 {
		t.Errorf("expected nothing deleted, got: %v", store.deleted)
	}
}

func TestCmdSessionRemove_Cancel(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: "other-0000-0000-0000-000000000000", Title: "x"},
	}
	app.Store = store
	app.LR = newScriptReader("")

	app.cmdSessionRemove(context.Background())

	if len(store.deleted) != 0 {
		t.Errorf("expected nothing deleted on cancel, got: %v", store.deleted)
	}
}

func TestCmdSessionRemove_InvalidChoice(t *testing.T) {
	p := &spyPrinter{}
	app := newTestApp(p)
	store := newFakeSessionStore()
	store.sessions = []domain.SessionSummary{
		{ID: "other-0000-0000-0000-000000000000", Title: "x"},
	}
	app.Store = store
	app.LR = newScriptReader("99")

	app.cmdSessionRemove(context.Background())

	out := p.Output()
	if !strings.Contains(out, "Invalid choice") {
		t.Errorf("expected 'Invalid choice', got: %s", out)
	}
}
