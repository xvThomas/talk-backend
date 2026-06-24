package agui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
)

func TestHandler_ValidRequest(t *testing.T) {
	handler := NewHandler(nil, nil)

	body := `{"messages":[{"id":"m1","role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/event-stream")
	}

	evts := parseSSEEvents(t, rec.Body.Bytes())

	if len(evts) != 5 {
		t.Fatalf("got %d events, want 5", len(evts))
	}

	assertEventType(t, evts[0], events.EventTypeRunStarted)
	assertEventType(t, evts[1], events.EventTypeTextMessageStart)
	assertEventType(t, evts[2], events.EventTypeTextMessageContent)
	assertEventType(t, evts[3], events.EventTypeTextMessageEnd)
	assertEventType(t, evts[4], events.EventTypeRunFinished)

	// RUN_STARTED must include a threadId.
	if evts[0]["threadId"] == nil {
		t.Error("RUN_STARTED missing threadId")
	}

	// TEXT_MESSAGE_CONTENT must have a delta.
	if evts[2]["delta"] == nil || evts[2]["delta"] == "" {
		t.Error("TEXT_MESSAGE_CONTENT missing delta")
	}

	// TEXT_MESSAGE_START must have role=assistant.
	if role, _ := evts[1]["role"].(string); role != "assistant" {
		t.Errorf("TEXT_MESSAGE_START role = %q, want %q", role, "assistant")
	}
}

func TestHandler_WithThreadID(t *testing.T) {
	handler := NewHandler(nil, nil)

	body := `{"threadId":"existing-thread","messages":[{"id":"m1","role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	evts := parseSSEEvents(t, rec.Body.Bytes())
	if tid, _ := evts[0]["threadId"].(string); tid != "existing-thread" {
		t.Errorf("threadId = %q, want %q", tid, "existing-thread")
	}
}

func TestHandler_MalformedJSON(t *testing.T) {
	handler := NewHandler(nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader("{invalid"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandler_EmptyMessages(t *testing.T) {
	handler := NewHandler(nil, nil)

	body := `{"messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandler_WithChatFunc(t *testing.T) {
	chatFn := func(_ context.Context, _ string, messages []types.Message) (string, error) {
		content := fmt.Sprintf("%v", messages[0].Content)
		return "response to: " + content, nil
	}

	handler := NewHandler(nil, chatFn)

	body := `{"messages":[{"id":"m1","role":"user","content":"ping"}]}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	evts := parseSSEEvents(t, rec.Body.Bytes())
	if delta, _ := evts[2]["delta"].(string); delta != "response to: ping" {
		t.Errorf("delta = %q, want %q", delta, "response to: ping")
	}
}

func TestHandler_ChatFuncError(t *testing.T) {
	chatFn := func(_ context.Context, _ string, _ []types.Message) (string, error) {
		return "", context.DeadlineExceeded
	}

	handler := NewHandler(nil, chatFn)

	body := `{"messages":[{"id":"m1","role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	evts := parseSSEEvents(t, rec.Body.Bytes())

	// Should have RUN_STARTED then RUN_ERROR.
	if len(evts) < 2 {
		t.Fatalf("got %d events, want at least 2", len(evts))
	}
	assertEventType(t, evts[0], events.EventTypeRunStarted)
	assertEventType(t, evts[1], events.EventTypeRunError)

	if msg, _ := evts[1]["message"].(string); msg == "" {
		t.Error("RUN_ERROR missing message")
	}
}

// --- helpers ---

func parseSSEEvents(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var result []map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		jsonData := strings.TrimPrefix(line, "data: ")
		var m map[string]any
		if err := json.Unmarshal([]byte(jsonData), &m); err != nil {
			t.Fatalf("unmarshaling event: %v\ndata: %s", err, jsonData)
		}
		result = append(result, m)
	}
	return result
}

func assertEventType(t *testing.T, m map[string]any, want events.EventType) {
	t.Helper()
	got, _ := m["type"].(string)
	if got != string(want) {
		t.Errorf("event type = %q, want %q", got, want)
	}
}
