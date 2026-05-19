package langfuse

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
)

// Config holds connection settings for the Langfuse API.
type Config struct {
	PublicKey string
	SecretKey string
	BaseURL   string
}

// LangfuseStore is a domain.MessageStore backed by an in-memory buffer with Langfuse session history access.
// It implements both domain.MessageStore and domain.SessionBrowser.
type LangfuseStore struct {
	mu          sync.Mutex
	messages    []domain.Message
	timestamps  []time.Time
	loadedCount int // number of messages loaded via SetSession (no local timestamps)
	sessionID   string
	userID      string
	httpClient  *http.Client
	baseURL     string
	authHeader  string
}

var _ domain.MessageStore = (*LangfuseStore)(nil)
var _ domain.SessionBrowser = (*LangfuseStore)(nil)

// NewLangfuseStore creates a LangfuseStore with the given session/user identifiers and Langfuse config.
func NewLangfuseStore(sessionID, userID string, cfg Config) *LangfuseStore {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://cloud.langfuse.com"
	}
	authString := fmt.Sprintf("%s:%s", cfg.PublicKey, cfg.SecretKey)
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(authString))

	return &LangfuseStore{
		sessionID:  sessionID,
		userID:     userID,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    baseURL,
		authHeader: authHeader,
	}
}

// Add appends a message to the in-memory buffer.
func (s *LangfuseStore) Add(msg domain.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
	s.timestamps = append(s.timestamps, time.Now())
}

// All returns a copy of all buffered messages.
func (s *LangfuseStore) All() []domain.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]domain.Message, len(s.messages))
	copy(result, s.messages)
	return result
}

// Clear removes all messages from the buffer.
func (s *LangfuseStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
	s.timestamps = nil
	s.loadedCount = 0
}

// SessionID returns the current session identifier.
func (s *LangfuseStore) SessionID() string { return s.sessionID }

// UserID returns the user identifier.
func (s *LangfuseStore) UserID() string { return s.userID }

// SetSession switches to a different session and reloads its messages from Langfuse.
func (s *LangfuseStore) SetSession(ctx context.Context, sessionID string) error {
	msgs, err := s.loadMessagesForSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("loading session %q: %w", sessionID, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionID = sessionID
	s.messages = msgs
	s.timestamps = nil
	s.loadedCount = len(msgs)
	return nil
}

// ListSessions returns all sessions associated with userID, newest first.
func (s *LangfuseStore) ListSessions(ctx context.Context, userID string) ([]domain.SessionSummary, error) {
	endpoint := fmt.Sprintf("%s/api/public/sessions?limit=50&userId=%s",
		s.baseURL, url.QueryEscape(userID))

	body, err := s.apiGet(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			ID        string    `json:"id"`
			CreatedAt time.Time `json:"createdAt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing sessions response: %w", err)
	}

	summaries := make([]domain.SessionSummary, len(resp.Data))
	for i, d := range resp.Data {
		summaries[i] = domain.SessionSummary{
			ID:        d.ID,
			CreatedAt: d.CreatedAt,
		}
	}
	return summaries, nil
}

// LoadSession fetches the turn history for a given sessionID.
// It always queries the Langfuse API so that previous CLI runs sharing the
// same sessionID are included. For the current session, locally-added
// messages that Langfuse has not yet ingested are appended after
// deduplication.
func (s *LangfuseStore) LoadSession(ctx context.Context, sessionID string) ([]domain.HistoryTurn, error) {
	apiTurns, err := s.fetchTurns(ctx, sessionID)
	if err != nil {
		if sessionID != s.sessionID {
			return nil, err
		}
		// Current session: fall back to local buffer when API is unreachable.
		return s.turnsFromLocalBuffer(), nil
	}

	if sessionID != s.sessionID {
		return apiTurns, nil
	}

	return mergeTurns(apiTurns, s.turnsFromLocalBuffer()), nil
}

// fetchTurns queries the Langfuse API for traces+observations and builds turns.
func (s *LangfuseStore) fetchTurns(ctx context.Context, sessionID string) ([]domain.HistoryTurn, error) {
	traces, err := s.fetchTraces(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	turns := make([]domain.HistoryTurn, 0, len(traces))
	for _, t := range traces {
		obs, err := s.fetchObservations(ctx, t.id)
		if err != nil {
			continue
		}
		turns = append(turns, buildHistoryTurn(t, obs))
	}
	return turns, nil
}

// turnsFromLocalBuffer builds HistoryTurns only from messages that were
// added via Add() during the current CLI run (indices >= loadedCount).
func (s *LangfuseStore) turnsFromLocalBuffer() []domain.HistoryTurn {
	s.mu.Lock()
	offset := s.loadedCount
	if offset >= len(s.messages) {
		s.mu.Unlock()
		return nil
	}
	msgs := make([]domain.Message, len(s.messages)-offset)
	copy(msgs, s.messages[offset:])
	ts := make([]time.Time, len(s.timestamps))
	copy(ts, s.timestamps)
	s.mu.Unlock()

	var turns []domain.HistoryTurn
	for i := 0; i < len(msgs); i++ {
		if msgs[i].Role != domain.RoleUser {
			continue
		}
		question := msgs[i].Content
		turnID := msgs[i].TurnID
		var at time.Time
		if i < len(ts) {
			at = ts[i]
		}

		// Scan forward for the last assistant message with content.
		var answer string
		j := i + 1
		for j < len(msgs) && msgs[j].Role != domain.RoleUser {
			if msgs[j].Role == domain.RoleAssistant && msgs[j].Content != "" {
				answer = msgs[j].Content
			}
			j++
		}
		if answer != "" {
			turns = append(turns, domain.HistoryTurn{
				Question: question,
				Answer:   answer,
				At:       at,
				TurnID:   turnID,
			})
		}
		i = j - 1
	}
	return turns
}

// mergeTurns appends local turns to API turns, skipping any local turn
// whose TurnID already exists in the API results. The TurnID is the trace
// ID shared between the local buffer and Langfuse.
func mergeTurns(api, local []domain.HistoryTurn) []domain.HistoryTurn {
	if len(local) == 0 {
		return api
	}
	if len(api) == 0 {
		return local
	}

	known := make(map[string]struct{}, len(api))
	for _, t := range api {
		if t.TurnID != "" {
			known[t.TurnID] = struct{}{}
		}
	}

	result := make([]domain.HistoryTurn, len(api))
	copy(result, api)
	for _, t := range local {
		if t.TurnID != "" {
			if _, dup := known[t.TurnID]; dup {
				continue
			}
		}
		result = append(result, t)
	}
	return result
}

// traceRef is a lightweight struct with the data needed to build a HistoryTurn,
// extracted from Langfuse traces.
type traceRef struct {
	id        string
	createdAt time.Time
	input     string
	output    string
	model     string
}

// observation is a struct with the data needed to count LLM calls,
// extracted from Langfuse observations.
type observation struct {
	name   string
	input  string
	output string
}

func (s *LangfuseStore) loadMessagesForSession(ctx context.Context, sessionID string) ([]domain.Message, error) {
	traces, err := s.fetchTraces(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	var msgs []domain.Message
	for _, t := range traces {
		if t.input != "" {
			msgs = append(msgs, domain.Message{Role: domain.RoleUser, Content: t.input})
		}
		if t.output != "" {
			msgs = append(msgs, domain.Message{Role: domain.RoleAssistant, Content: t.output})
		}
	}
	return msgs, nil
}

func (s *LangfuseStore) fetchTraces(ctx context.Context, sessionID string) ([]traceRef, error) {
	endpoint := fmt.Sprintf("%s/api/public/traces?sessionId=%s&orderBy=timestamp.asc",
		s.baseURL, url.QueryEscape(sessionID))

	body, err := s.apiGet(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			ID        string          `json:"id"`
			Timestamp time.Time       `json:"timestamp"`
			Input     json.RawMessage `json:"input"`
			Output    json.RawMessage `json:"output"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing traces response: %w", err)
	}

	refs := make([]traceRef, len(resp.Data))
	for i, d := range resp.Data {
		refs[i] = traceRef{
			id:        d.ID,
			createdAt: d.Timestamp,
			input:     rawJSONToString(d.Input),
			output:    rawJSONToString(d.Output),
		}
	}
	return refs, nil
}

func (s *LangfuseStore) fetchObservations(ctx context.Context, traceID string) ([]observation, error) {
	endpoint := fmt.Sprintf("%s/api/public/observations?traceId=%s",
		s.baseURL, url.QueryEscape(traceID))

	body, err := s.apiGet(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			Name   string          `json:"name"`
			Input  json.RawMessage `json:"input"`
			Output json.RawMessage `json:"output"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing observations response: %w", err)
	}

	obs := make([]observation, len(resp.Data))
	for i, d := range resp.Data {
		obs[i] = observation{name: d.Name, input: rawJSONToString(d.Input), output: rawJSONToString(d.Output)}
	}
	return obs, nil
}

func buildHistoryTurn(t traceRef, obs []observation) domain.HistoryTurn {
	callCount := 0
	for _, o := range obs {
		if o.name == "llm_"+string(domain.CallKindInitial) ||
			o.name == "llm_"+string(domain.CallKindToolResult) {
			callCount++
		}
	}
	return domain.HistoryTurn{
		Question:  t.input,
		Answer:    t.output,
		At:        t.createdAt,
		Model:     t.model,
		CallCount: callCount,
		TurnID:    t.id,
	}
}

// rawJSONToString extracts a string from arbitrary JSON.
// Returns the unquoted string if the value is a JSON string, or the raw JSON text otherwise.
func rawJSONToString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

func (s *LangfuseStore) apiGet(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", s.authHeader)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("langfuse API HTTP %d for %s: %s", resp.StatusCode, endpoint, string(errBody))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	return body, nil
}
