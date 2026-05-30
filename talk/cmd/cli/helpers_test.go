package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/mcp"
)

// spyPrinter captures all output for assertions in tests.
type spyPrinter struct {
	out strings.Builder
	err strings.Builder
}

func (p *spyPrinter) Printf(format string, args ...any) { fmt.Fprintf(&p.out, format, args...) }
func (p *spyPrinter) Println(args ...any)               { fmt.Fprintln(&p.out, args...) }
func (p *spyPrinter) Errorf(format string, args ...any) { fmt.Fprintf(&p.err, format, args...) }
func (p *spyPrinter) Output() string                    { return p.out.String() }
func (p *spyPrinter) ErrOutput() string                 { return p.err.String() }
func (p *spyPrinter) Reset()                            { p.out.Reset(); p.err.Reset() }

// scriptReader implements Reader with pre-scripted answers.
type scriptReader struct {
	answers []string
	idx     int
}

func newScriptReader(answers ...string) *scriptReader {
	return &scriptReader{answers: answers}
}

func (s *scriptReader) ReadLine(_ string) (string, error) {
	if s.idx >= len(s.answers) {
		return "", fmt.Errorf("no more scripted answers")
	}
	ans := s.answers[s.idx]
	s.idx++
	return ans, nil
}

// stubPromptProvider returns a fixed system prompt or error.
type stubPromptProvider struct {
	text string
	err  error
}

func (s *stubPromptProvider) SystemPrompt(_ context.Context) (string, error) {
	return s.text, s.err
}

// fakeStore implements domain.MessageStore with minimal in-memory state.
type fakeStore struct {
	messages  []domain.Message
	sessionID string
	userID    string
}

func newFakeStore() *fakeStore {
	return &fakeStore{sessionID: "test-session-id", userID: "test-user"}
}

func (s *fakeStore) Add(msg domain.Message) { s.messages = append(s.messages, msg) }
func (s *fakeStore) All() []domain.Message  { return s.messages }
func (s *fakeStore) Clear()                 { s.messages = nil }
func (s *fakeStore) SessionID() string      { return s.sessionID }
func (s *fakeStore) UserID() string         { return s.userID }

// fakeSessionStore implements both domain.MessageStore and domain.SessionBrowser.
type fakeSessionStore struct {
	fakeStore
	sessions  []domain.SessionSummary
	turns     map[string][]domain.HistoryTurn
	setErr    error
	deleteErr error
	deleted   []string
}

func newFakeSessionStore() *fakeSessionStore {
	return &fakeSessionStore{
		fakeStore: fakeStore{sessionID: "abcd1234-0000-0000-0000-000000000000", userID: "test-user"},
		turns:     make(map[string][]domain.HistoryTurn),
	}
}

func (s *fakeSessionStore) SetSession(_ context.Context, id string) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.sessionID = id
	return nil
}

func (s *fakeSessionStore) ListSessions(_ context.Context, _ string) ([]domain.SessionSummary, error) {
	return s.sessions, nil
}

func (s *fakeSessionStore) LoadSession(_ context.Context, id string) ([]domain.HistoryTurn, error) {
	return s.turns[id], nil
}

func (s *fakeSessionStore) DeleteSession(_ context.Context, id string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = append(s.deleted, id)
	for i, sess := range s.sessions {
		if sess.ID == id {
			s.sessions = append(s.sessions[:i], s.sessions[i+1:]...)
			break
		}
	}
	return nil
}

// fakeRegistry implements mcp.Registry for testing.
type fakeRegistry struct {
	servers []mcp.ServerConfig
	addErr  error
	rmErr   error
}

func (r *fakeRegistry) Add(_ context.Context, cfg mcp.ServerConfig) error {
	if r.addErr != nil {
		return r.addErr
	}
	r.servers = append(r.servers, cfg)
	return nil
}

func (r *fakeRegistry) Remove(_ context.Context, id string) error {
	if r.rmErr != nil {
		return r.rmErr
	}
	for i, s := range r.servers {
		if s.ID == id {
			r.servers = append(r.servers[:i], r.servers[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *fakeRegistry) Get(_ context.Context, id string) (mcp.ServerConfig, error) {
	for _, s := range r.servers {
		if s.ID == id {
			return s, nil
		}
	}
	return mcp.ServerConfig{}, fmt.Errorf("not found")
}

func (r *fakeRegistry) List(_ context.Context) ([]mcp.ServerConfig, error) {
	return r.servers, nil
}

// fakeRouter implements ModelSwitcher for testing.
type fakeRouter struct {
	client domain.LlmClient
	err    error
}

func (r *fakeRouter) Get(_ domain.Model) (domain.LlmClient, error) {
	return r.client, r.err
}

// fakeLlmClient implements domain.LlmClient as a no-op.
type fakeLlmClient struct{}

func (fakeLlmClient) Complete(_ context.Context, _ string, _ []domain.Message, _ []domain.Tool) (*domain.Message, domain.Usage, error) {
	return &domain.Message{Role: "assistant", Content: "fake"}, domain.Usage{}, nil
}

// newTestApp creates an App with a spyPrinter and given options for testing.
func newTestApp(p *spyPrinter) *App {
	return &App{
		Printer:      p,
		Store:        newFakeStore(),
		PP:           &stubPromptProvider{text: "You are a helpful assistant."},
		CurrentModel: "sonnet-4.6",
		LR:           newScriptReader(), // empty reader by default
		Router:       &fakeRouter{client: fakeLlmClient{}},
	}
}
