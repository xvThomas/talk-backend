package agui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	"github.com/xvThomas/talk-backend/talk/internal/domain"
)

func TestHandler_ValidRequest(t *testing.T) {
	handler := NewHandler(nil, nil, []string{"sonnet-4.6", "haiku-4.5"})

	body := `{"messages":[{"id":"m1","role":"user","content":"hello"}],"forwardedProps":{"model":"sonnet-4.6"}}`
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

	// With nil chatFn: RUN_STARTED → RUN_FINISHED
	if len(evts) != 2 {
		for i, e := range evts {
			t.Logf("event[%d]: %v", i, e["type"])
		}
		t.Fatalf("got %d events, want 2", len(evts))
	}

	assertEventType(t, evts[0], events.EventTypeRunStarted)
	assertEventType(t, evts[1], events.EventTypeRunFinished)

	// RUN_STARTED must include a threadId.
	if evts[0]["threadId"] == nil {
		t.Error("RUN_STARTED missing threadId")
	}
}

func TestHandler_WithThreadID(t *testing.T) {
	handler := NewHandler(nil, nil, []string{"sonnet-4.6"})

	body := `{"threadId":"existing-thread","messages":[{"id":"m1","role":"user","content":"hi"}],"forwardedProps":{"model":"sonnet-4.6"}}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	evts := parseSSEEvents(t, rec.Body.Bytes())
	if tid, _ := evts[0]["threadId"].(string); tid != "existing-thread" {
		t.Errorf("threadId = %q, want %q", tid, "existing-thread")
	}
}

func TestHandler_MalformedJSON(t *testing.T) {
	handler := NewHandler(nil, nil, []string{"sonnet-4.6"})

	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader("{invalid"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandler_EmptyMessages(t *testing.T) {
	handler := NewHandler(nil, nil, []string{"sonnet-4.6"})

	body := `{"messages":[],"forwardedProps":{"model":"sonnet-4.6"}}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandler_WithChatFunc(t *testing.T) {
	chatFn := func(_ context.Context, _ string, modelAlias string, messages []types.Message, opts ChatOptions) error {
		content := fmt.Sprintf("%v", messages[0].Content)
		emitter := NewAGUIEmitter(opts.SSEWriter, nil)
		return emitter.HandleMessageEvent(context.Background(), domain.MessageEvent{
			Message: domain.Message{
				Role:    domain.RoleAssistant,
				Content: "response to: " + content + " (model: " + modelAlias + ")",
			},
		})
	}

	handler := NewHandler(nil, chatFn, []string{"sonnet-4.6"})

	body := `{"messages":[{"id":"m1","role":"user","content":"ping"}],"forwardedProps":{"model":"sonnet-4.6"}}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	evts := parseSSEEvents(t, rec.Body.Bytes())

	// RUN_STARTED, TEXT_MESSAGE_START, TEXT_MESSAGE_CONTENT, TEXT_MESSAGE_END, RUN_FINISHED
	if len(evts) != 5 {
		for i, e := range evts {
			t.Logf("event[%d]: %v", i, e["type"])
		}
		t.Fatalf("got %d events, want 5", len(evts))
	}

	assertEventType(t, evts[0], events.EventTypeRunStarted)
	assertEventType(t, evts[1], events.EventTypeTextMessageStart)
	assertEventType(t, evts[2], events.EventTypeTextMessageContent)
	assertEventType(t, evts[3], events.EventTypeTextMessageEnd)
	assertEventType(t, evts[4], events.EventTypeRunFinished)

	if delta, _ := evts[2]["delta"].(string); delta != "response to: ping (model: sonnet-4.6)" {
		t.Errorf("delta = %q, want %q", delta, "response to: ping (model: sonnet-4.6)")
	}

	// TEXT_MESSAGE_START must have role=assistant.
	if role, _ := evts[1]["role"].(string); role != "assistant" {
		t.Errorf("TEXT_MESSAGE_START role = %q, want %q", role, "assistant")
	}
}

func TestHandler_ChatFuncError(t *testing.T) {
	chatFn := func(_ context.Context, _ string, _ string, _ []types.Message, _ ChatOptions) error {
		return context.DeadlineExceeded
	}

	handler := NewHandler(nil, chatFn, []string{"sonnet-4.6"})

	body := `{"messages":[{"id":"m1","role":"user","content":"hi"}],"forwardedProps":{"model":"sonnet-4.6"}}`
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

func TestHandler_MissingForwardedProps(t *testing.T) {
	handler := NewHandler(nil, nil, []string{"sonnet-4.6", "haiku-4.5"})

	body := `{"messages":[{"id":"m1","role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	evts := parseSSEEvents(t, rec.Body.Bytes())
	if len(evts) != 1 {
		t.Fatalf("got %d events, want 1", len(evts))
	}
	assertEventType(t, evts[0], events.EventTypeRunError)

	msg, _ := evts[0]["message"].(string)
	if !strings.Contains(msg, "model field is required") {
		t.Errorf("error message = %q, want it to contain 'model field is required'", msg)
	}
	if !strings.Contains(msg, "sonnet-4.6") {
		t.Errorf("error message = %q, want it to list available models", msg)
	}
}

func TestHandler_EmptyModelAlias(t *testing.T) {
	handler := NewHandler(nil, nil, []string{"sonnet-4.6"})

	body := `{"messages":[{"id":"m1","role":"user","content":"hi"}],"forwardedProps":{"model":""}}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	evts := parseSSEEvents(t, rec.Body.Bytes())
	if len(evts) != 1 {
		t.Fatalf("got %d events, want 1", len(evts))
	}
	assertEventType(t, evts[0], events.EventTypeRunError)

	msg, _ := evts[0]["message"].(string)
	if !strings.Contains(msg, "model field is required") {
		t.Errorf("error message = %q, want it to contain 'model field is required'", msg)
	}
}

func TestHandler_UnknownModel(t *testing.T) {
	handler := NewHandler(nil, nil, []string{"sonnet-4.6", "haiku-4.5"})

	body := `{"messages":[{"id":"m1","role":"user","content":"hi"}],"forwardedProps":{"model":"unknown-model"}}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	evts := parseSSEEvents(t, rec.Body.Bytes())
	if len(evts) != 1 {
		t.Fatalf("got %d events, want 1", len(evts))
	}
	assertEventType(t, evts[0], events.EventTypeRunError)

	msg, _ := evts[0]["message"].(string)
	if !strings.Contains(msg, "Unknown model") {
		t.Errorf("error message = %q, want it to contain 'Unknown model'", msg)
	}
	if !strings.Contains(msg, "sonnet-4.6") {
		t.Errorf("error message = %q, want it to list available models", msg)
	}
}

func TestHandler_ForwardedPropsNotAMap(t *testing.T) {
	handler := NewHandler(nil, nil, []string{"sonnet-4.6"})

	body := `{"messages":[{"id":"m1","role":"user","content":"hi"}],"forwardedProps":"not-a-map"}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	evts := parseSSEEvents(t, rec.Body.Bytes())
	if len(evts) != 1 {
		t.Fatalf("got %d events, want 1", len(evts))
	}
	assertEventType(t, evts[0], events.EventTypeRunError)

	msg, _ := evts[0]["message"].(string)
	if !strings.Contains(msg, "model field is required") {
		t.Errorf("error message = %q, want it to contain 'model field is required'", msg)
	}
}

func TestHandler_ModelPassedToChatFunc(t *testing.T) {
	var receivedModel string
	chatFn := func(_ context.Context, _ string, modelAlias string, _ []types.Message, _ ChatOptions) error {
		receivedModel = modelAlias
		return nil
	}

	handler := NewHandler(nil, chatFn, []string{"haiku-4.5", "sonnet-4.6"})

	body := `{"messages":[{"id":"m1","role":"user","content":"hi"}],"forwardedProps":{"model":"haiku-4.5"}}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if receivedModel != "haiku-4.5" {
		t.Errorf("chatFn received model = %q, want %q", receivedModel, "haiku-4.5")
	}
}

func TestHandler_ClientDisconnectDuringChat(t *testing.T) {
	chatFn := func(ctx context.Context, _ string, _ string, _ []types.Message, _ ChatOptions) error {
		// Simulate client disconnect: context is already cancelled when chatFn runs.
		return ctx.Err()
	}

	handler := NewHandler(nil, chatFn, []string{"sonnet-4.6"})

	body := `{"messages":[{"id":"m1","role":"user","content":"hi"}],"forwardedProps":{"model":"sonnet-4.6"}}`
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to simulate client disconnect.

	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	evts := parseSSEEvents(t, rec.Body.Bytes())

	// Should have at most RUN_STARTED — no TEXT_MESSAGE events or RUN_FINISHED.
	for _, evt := range evts {
		evtType, _ := evt["type"].(string)
		if evtType == string(events.EventTypeTextMessageStart) ||
			evtType == string(events.EventTypeTextMessageContent) ||
			evtType == string(events.EventTypeTextMessageEnd) ||
			evtType == string(events.EventTypeRunFinished) {
			t.Errorf("unexpected event after disconnect: %s", evtType)
		}
	}
}

func TestHandler_NoGoroutineLeak(t *testing.T) {
	chatFn := func(ctx context.Context, _ string, _ string, _ []types.Message, _ ChatOptions) error {
		return ctx.Err()
	}

	handler := NewHandler(nil, chatFn, []string{"sonnet-4.6"})

	baseline := runtime.NumGoroutine()

	for range 10 {
		body := `{"messages":[{"id":"m1","role":"user","content":"hi"}],"forwardedProps":{"model":"sonnet-4.6"}}`
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
	}

	// Allow goroutines to settle.
	runtime.Gosched()

	after := runtime.NumGoroutine()
	// Tolerate up to 2 goroutine variance for runtime background work.
	if after > baseline+2 {
		t.Errorf("goroutine leak: before=%d, after=%d", baseline, after)
	}
}

func TestHandler_ToolCallEventsInSSEStream(t *testing.T) {
	// chatFn uses AGUIEmitter to emit tool call events via the SSEWriter in ChatOptions.
	chatFn := func(ctx context.Context, _ string, _ string, _ []types.Message, opts ChatOptions) error {
		emitter := NewAGUIEmitter(opts.SSEWriter, nil)
		tc := domain.ToolCall{ID: "call-abc", Name: "get_weather", Input: map[string]any{"city": "Paris"}}

		_ = emitter.HandleToolCallStart(ctx, domain.ToolCallEvent{
			TurnID:   "turn-1",
			ToolCall: tc,
		})
		_ = emitter.HandleToolCallEnd(ctx, domain.ToolCallEndEvent{
			TurnID:   "turn-1",
			ToolCall: tc,
			Result:   domain.ToolResult{ToolCallID: "call-abc", Content: "sunny"},
		})
		return emitter.HandleMessageEvent(ctx, domain.MessageEvent{
			Message: domain.Message{
				Role:    domain.RoleAssistant,
				Content: "The weather is sunny",
			},
		})
	}

	handler := NewHandler(nil, chatFn, []string{"sonnet-4.6"})

	body := `{"messages":[{"id":"m1","role":"user","content":"weather?"}],"forwardedProps":{"model":"sonnet-4.6"}}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	evts := parseSSEEvents(t, rec.Body.Bytes())

	// Expected sequence:
	// RUN_STARTED, TOOL_CALL_START, TOOL_CALL_ARGS, TOOL_CALL_END,
	// TEXT_MESSAGE_START, TEXT_MESSAGE_CONTENT, TEXT_MESSAGE_END, RUN_FINISHED
	if len(evts) != 8 {
		for i, e := range evts {
			t.Logf("event[%d]: %v", i, e["type"])
		}
		t.Fatalf("got %d events, want 8", len(evts))
	}

	assertEventType(t, evts[0], events.EventTypeRunStarted)
	assertEventType(t, evts[1], events.EventTypeToolCallStart)
	assertEventType(t, evts[2], events.EventTypeToolCallArgs)
	assertEventType(t, evts[3], events.EventTypeToolCallEnd)
	assertEventType(t, evts[4], events.EventTypeTextMessageStart)
	assertEventType(t, evts[5], events.EventTypeTextMessageContent)
	assertEventType(t, evts[6], events.EventTypeTextMessageEnd)
	assertEventType(t, evts[7], events.EventTypeRunFinished)

	// Verify tool call IDs are consistent.
	if id := evts[1]["toolCallId"]; id != "call-abc" {
		t.Errorf("TOOL_CALL_START toolCallId = %v, want %q", id, "call-abc")
	}
	if id := evts[3]["toolCallId"]; id != "call-abc" {
		t.Errorf("TOOL_CALL_END toolCallId = %v, want %q", id, "call-abc")
	}
}

func TestHandler_MultipleToolCallsInOneIteration(t *testing.T) {
	chatFn := func(ctx context.Context, _ string, _ string, _ []types.Message, opts ChatOptions) error {
		emitter := NewAGUIEmitter(opts.SSEWriter, nil)
		tools := []domain.ToolCall{
			{ID: "call-1", Name: "get_weather", Input: map[string]any{"city": "Paris"}},
			{ID: "call-2", Name: "get_time", Input: map[string]any{"tz": "CET"}},
		}
		for _, tc := range tools {
			_ = emitter.HandleToolCallStart(ctx, domain.ToolCallEvent{TurnID: "turn-1", ToolCall: tc})
			_ = emitter.HandleToolCallEnd(ctx, domain.ToolCallEndEvent{TurnID: "turn-1", ToolCall: tc, Result: domain.ToolResult{ToolCallID: tc.ID, Content: "ok"}})
		}
		return emitter.HandleMessageEvent(ctx, domain.MessageEvent{
			Message: domain.Message{Role: domain.RoleAssistant, Content: "done"},
		})
	}

	handler := NewHandler(nil, chatFn, []string{"sonnet-4.6"})

	body := `{"messages":[{"id":"m1","role":"user","content":"hi"}],"forwardedProps":{"model":"sonnet-4.6"}}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	evts := parseSSEEvents(t, rec.Body.Bytes())

	// RUN_STARTED + 2*(START+ARGS+END) + TEXT_MESSAGE_START/CONTENT/END + RUN_FINISHED = 11
	if len(evts) != 11 {
		for i, e := range evts {
			t.Logf("event[%d]: %v", i, e["type"])
		}
		t.Fatalf("got %d events, want 11", len(evts))
	}

	// First tool triplet.
	assertEventType(t, evts[1], events.EventTypeToolCallStart)
	assertEventType(t, evts[2], events.EventTypeToolCallArgs)
	assertEventType(t, evts[3], events.EventTypeToolCallEnd)
	if evts[1]["toolCallId"] != "call-1" {
		t.Errorf("first TOOL_CALL_START toolCallId = %v, want %q", evts[1]["toolCallId"], "call-1")
	}

	// Second tool triplet.
	assertEventType(t, evts[4], events.EventTypeToolCallStart)
	assertEventType(t, evts[5], events.EventTypeToolCallArgs)
	assertEventType(t, evts[6], events.EventTypeToolCallEnd)
	if evts[4]["toolCallId"] != "call-2" {
		t.Errorf("second TOOL_CALL_START toolCallId = %v, want %q", evts[4]["toolCallId"], "call-2")
	}
}

func TestHandler_MultiIterationToolLoop(t *testing.T) {
	chatFn := func(ctx context.Context, _ string, _ string, _ []types.Message, opts ChatOptions) error {
		emitter := NewAGUIEmitter(opts.SSEWriter, nil)

		// Simulate iteration 1: tool call.
		tc1 := domain.ToolCall{ID: "call-iter1", Name: "search", Input: map[string]any{"q": "hello"}}
		_ = emitter.HandleToolCallStart(ctx, domain.ToolCallEvent{TurnID: "turn-1", ToolCall: tc1})
		_ = emitter.HandleToolCallEnd(ctx, domain.ToolCallEndEvent{TurnID: "turn-1", ToolCall: tc1, Result: domain.ToolResult{ToolCallID: tc1.ID, Content: "found"}})

		// Simulate iteration 2: another tool call.
		tc2 := domain.ToolCall{ID: "call-iter2", Name: "fetch", Input: map[string]any{"url": "http://x"}}
		_ = emitter.HandleToolCallStart(ctx, domain.ToolCallEvent{TurnID: "turn-1", ToolCall: tc2})
		_ = emitter.HandleToolCallEnd(ctx, domain.ToolCallEndEvent{TurnID: "turn-1", ToolCall: tc2, Result: domain.ToolResult{ToolCallID: tc2.ID, Content: "data"}})

		return emitter.HandleMessageEvent(ctx, domain.MessageEvent{
			Message: domain.Message{Role: domain.RoleAssistant, Content: "final answer"},
		})
	}

	handler := NewHandler(nil, chatFn, []string{"sonnet-4.6"})

	body := `{"messages":[{"id":"m1","role":"user","content":"q"}],"forwardedProps":{"model":"sonnet-4.6"}}`
	req := httptest.NewRequest(http.MethodPost, "/agent", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	evts := parseSSEEvents(t, rec.Body.Bytes())

	// RUN_STARTED + 2*(START+ARGS+END) + TEXT_MESSAGE_START/CONTENT/END + RUN_FINISHED = 11
	if len(evts) != 11 {
		for i, e := range evts {
			t.Logf("event[%d]: %v", i, e["type"])
		}
		t.Fatalf("got %d events, want 11", len(evts))
	}

	// Verify both iterations produced distinct tool call IDs.
	if evts[1]["toolCallId"] != "call-iter1" {
		t.Errorf("iter1 toolCallId = %v, want %q", evts[1]["toolCallId"], "call-iter1")
	}
	if evts[4]["toolCallId"] != "call-iter2" {
		t.Errorf("iter2 toolCallId = %v, want %q", evts[4]["toolCallId"], "call-iter2")
	}
}
