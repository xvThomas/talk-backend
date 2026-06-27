package agui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/xvThomas/talk-backend/talk/internal/domain"
)

func TestAGUIEmitter_HandleToolCallStart(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, err := NewSSEWriter(rec, nil)
	if err != nil {
		t.Fatalf("creating SSEWriter: %v", err)
	}

	emitter := NewAGUIEmitter(sse, nil)

	event := domain.ToolCallEvent{
		TurnID: "turn-1",
		ToolCall: domain.ToolCall{
			ID:    "call-123",
			Name:  "get_weather",
			Input: map[string]any{"city": "Paris"},
		},
	}

	if err := emitter.HandleToolCallStart(context.Background(), event); err != nil {
		t.Fatalf("HandleToolCallStart error: %v", err)
	}

	evts := parseSSEData(t, rec.Body.Bytes())

	if len(evts) != 2 {
		t.Fatalf("got %d events, want 2", len(evts))
	}

	// First event: TOOL_CALL_START
	if got := evts[0]["type"]; got != "TOOL_CALL_START" {
		t.Errorf("event[0] type = %q, want %q", got, "TOOL_CALL_START")
	}
	if got := evts[0]["toolCallId"]; got != "call-123" {
		t.Errorf("event[0] toolCallId = %q, want %q", got, "call-123")
	}
	if got := evts[0]["toolCallName"]; got != "get_weather" {
		t.Errorf("event[0] toolCallName = %q, want %q", got, "get_weather")
	}

	// Second event: TOOL_CALL_ARGS
	if got := evts[1]["type"]; got != "TOOL_CALL_ARGS" {
		t.Errorf("event[1] type = %q, want %q", got, "TOOL_CALL_ARGS")
	}
	if got := evts[1]["toolCallId"]; got != "call-123" {
		t.Errorf("event[1] toolCallId = %q, want %q", got, "call-123")
	}

	// Verify args delta is valid JSON containing "city":"Paris".
	delta, _ := evts[1]["delta"].(string)
	var args map[string]any
	if err := json.Unmarshal([]byte(delta), &args); err != nil {
		t.Fatalf("delta is not valid JSON: %v", err)
	}
	if args["city"] != "Paris" {
		t.Errorf("args[city] = %v, want %q", args["city"], "Paris")
	}
}

func TestAGUIEmitter_HandleToolCallEnd(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, err := NewSSEWriter(rec, nil)
	if err != nil {
		t.Fatalf("creating SSEWriter: %v", err)
	}

	emitter := NewAGUIEmitter(sse, nil)

	event := domain.ToolCallEndEvent{
		TurnID: "turn-1",
		ToolCall: domain.ToolCall{
			ID:   "call-456",
			Name: "get_weather",
		},
		Result: domain.ToolResult{ToolCallID: "call-456", Content: "sunny"},
	}

	if err := emitter.HandleToolCallEnd(context.Background(), event); err != nil {
		t.Fatalf("HandleToolCallEnd error: %v", err)
	}

	evts := parseSSEData(t, rec.Body.Bytes())

	if len(evts) != 1 {
		t.Fatalf("got %d events, want 1", len(evts))
	}

	if got := evts[0]["type"]; got != "TOOL_CALL_END" {
		t.Errorf("event type = %q, want %q", got, "TOOL_CALL_END")
	}
	if got := evts[0]["toolCallId"]; got != "call-456" {
		t.Errorf("event toolCallId = %q, want %q", got, "call-456")
	}
}

func TestAGUIEmitter_HandleMessageEvent(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, err := NewSSEWriter(rec, nil)
	if err != nil {
		t.Fatalf("creating SSEWriter: %v", err)
	}

	emitter := NewAGUIEmitter(sse, nil)

	event := domain.MessageEvent{
		Message: domain.Message{
			Role:    domain.RoleAssistant,
			Content: "Hello!",
		},
	}

	if err := emitter.HandleMessageEvent(context.Background(), event); err != nil {
		t.Fatalf("HandleMessageEvent error: %v", err)
	}

	evts := parseSSEData(t, rec.Body.Bytes())

	if len(evts) != 3 {
		t.Fatalf("got %d events, want 3 (START, CONTENT, END)", len(evts))
	}

	if got := evts[0]["type"]; got != "TEXT_MESSAGE_START" {
		t.Errorf("event[0] type = %q, want %q", got, "TEXT_MESSAGE_START")
	}
	if got := evts[0]["role"]; got != "assistant" {
		t.Errorf("event[0] role = %q, want %q", got, "assistant")
	}
	if got := evts[1]["type"]; got != "TEXT_MESSAGE_CONTENT" {
		t.Errorf("event[1] type = %q, want %q", got, "TEXT_MESSAGE_CONTENT")
	}
	if got := evts[1]["delta"]; got != "Hello!" {
		t.Errorf("event[1] delta = %q, want %q", got, "Hello!")
	}
	if got := evts[2]["type"]; got != "TEXT_MESSAGE_END" {
		t.Errorf("event[2] type = %q, want %q", got, "TEXT_MESSAGE_END")
	}
}

func TestAGUIEmitter_HandleMessageEvent_SkipsNonAssistant(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, err := NewSSEWriter(rec, nil)
	if err != nil {
		t.Fatalf("creating SSEWriter: %v", err)
	}

	emitter := NewAGUIEmitter(sse, nil)

	// User messages should be skipped.
	event := domain.MessageEvent{
		Message: domain.Message{
			Role:    domain.RoleUser,
			Content: "hi",
		},
	}

	if err := emitter.HandleMessageEvent(context.Background(), event); err != nil {
		t.Fatalf("HandleMessageEvent error: %v", err)
	}

	evts := parseSSEData(t, rec.Body.Bytes())
	if len(evts) != 0 {
		t.Errorf("got %d events for user message, want 0", len(evts))
	}
}

func TestAGUIEmitter_HandleMessageEvent_SkipsToolCallMessages(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, err := NewSSEWriter(rec, nil)
	if err != nil {
		t.Fatalf("creating SSEWriter: %v", err)
	}

	emitter := NewAGUIEmitter(sse, nil)

	event := domain.MessageEvent{
		Message: domain.Message{
			Role:      domain.RoleAssistant,
			Content:   "",
			ToolCalls: []domain.ToolCall{{ID: "tc-1", Name: "tool"}},
		},
	}

	if err := emitter.HandleMessageEvent(context.Background(), event); err != nil {
		t.Fatalf("HandleMessageEvent error: %v", err)
	}

	evts := parseSSEData(t, rec.Body.Bytes())
	if len(evts) != 0 {
		t.Errorf("got %d events for tool-call message, want 0", len(evts))
	}
}

func TestAGUIEmitter_CancelledContext(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, err := NewSSEWriter(rec, nil)
	if err != nil {
		t.Fatalf("creating SSEWriter: %v", err)
	}

	emitter := NewAGUIEmitter(sse, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := domain.ToolCallEvent{
		TurnID:   "turn-1",
		ToolCall: domain.ToolCall{ID: "call-789", Name: "tool"},
	}

	if err := emitter.HandleToolCallStart(ctx, event); err != nil {
		t.Fatalf("expected nil error on cancelled context, got: %v", err)
	}

	// No events should be written.
	evts := parseSSEData(t, rec.Body.Bytes())
	if len(evts) != 0 {
		t.Errorf("got %d events on cancelled context, want 0", len(evts))
	}
}

// parseSSEData extracts JSON objects from SSE data frames.
func parseSSEData(t *testing.T, data []byte) []map[string]any {
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

func TestAGUIEmitter_ReasoningThenText(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, err := NewSSEWriter(rec, nil)
	if err != nil {
		t.Fatalf("creating SSEWriter: %v", err)
	}

	emitter := NewAGUIEmitter(sse, nil)

	event := domain.MessageEvent{
		Message: domain.Message{
			Role:     domain.RoleAssistant,
			Content:  "The answer is 42.",
			Thinking: "Let me think about this...",
		},
	}

	if err := emitter.HandleMessageEvent(context.Background(), event); err != nil {
		t.Fatalf("HandleMessageEvent error: %v", err)
	}

	evts := parseSSEData(t, rec.Body.Bytes())

	// 5 reasoning + 3 text = 8 events
	if len(evts) != 8 {
		for i, e := range evts {
			t.Logf("event[%d]: %v", i, e["type"])
		}
		t.Fatalf("got %d events, want 8", len(evts))
	}

	assertSSEEventType(t, evts[0], events.EventTypeReasoningStart)
	assertSSEEventType(t, evts[1], events.EventTypeReasoningMessageStart)
	assertSSEEventType(t, evts[2], events.EventTypeReasoningMessageContent)
	assertSSEEventType(t, evts[3], events.EventTypeReasoningMessageEnd)
	assertSSEEventType(t, evts[4], events.EventTypeReasoningEnd)
	assertSSEEventType(t, evts[5], events.EventTypeTextMessageStart)
	assertSSEEventType(t, evts[6], events.EventTypeTextMessageContent)
	assertSSEEventType(t, evts[7], events.EventTypeTextMessageEnd)

	// Verify reasoning content.
	if got := evts[2]["delta"]; got != "Let me think about this..." {
		t.Errorf("reasoning delta = %q, want %q", got, "Let me think about this...")
	}
	// Verify reasoning role.
	if got := evts[1]["role"]; got != "reasoning" {
		t.Errorf("reasoning role = %q, want %q", got, "reasoning")
	}
	// Verify reasoning messageId consistency.
	reasoningID := evts[0]["messageId"]
	for i := 1; i <= 4; i++ {
		if evts[i]["messageId"] != reasoningID {
			t.Errorf("event[%d] messageId = %v, want %v", i, evts[i]["messageId"], reasoningID)
		}
	}
	// Verify text content.
	if got := evts[6]["delta"]; got != "The answer is 42." {
		t.Errorf("text delta = %q, want %q", got, "The answer is 42.")
	}
}

func TestAGUIEmitter_ReasoningWithToolCalls(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, err := NewSSEWriter(rec, nil)
	if err != nil {
		t.Fatalf("creating SSEWriter: %v", err)
	}

	emitter := NewAGUIEmitter(sse, nil)

	// Assistant message with thinking AND tool calls → reasoning emitted, no text.
	event := domain.MessageEvent{
		Message: domain.Message{
			Role:      domain.RoleAssistant,
			Content:   "",
			Thinking:  "I need to call a tool.",
			ToolCalls: []domain.ToolCall{{ID: "tc-1", Name: "search"}},
		},
	}

	if err := emitter.HandleMessageEvent(context.Background(), event); err != nil {
		t.Fatalf("HandleMessageEvent error: %v", err)
	}

	evts := parseSSEData(t, rec.Body.Bytes())

	// Only 5 reasoning events, no text events.
	if len(evts) != 5 {
		for i, e := range evts {
			t.Logf("event[%d]: %v", i, e["type"])
		}
		t.Fatalf("got %d events, want 5", len(evts))
	}

	assertSSEEventType(t, evts[0], events.EventTypeReasoningStart)
	assertSSEEventType(t, evts[1], events.EventTypeReasoningMessageStart)
	assertSSEEventType(t, evts[2], events.EventTypeReasoningMessageContent)
	assertSSEEventType(t, evts[3], events.EventTypeReasoningMessageEnd)
	assertSSEEventType(t, evts[4], events.EventTypeReasoningEnd)
}

func TestAGUIEmitter_NoReasoningWhenThinkingEmpty(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, err := NewSSEWriter(rec, nil)
	if err != nil {
		t.Fatalf("creating SSEWriter: %v", err)
	}

	emitter := NewAGUIEmitter(sse, nil)

	event := domain.MessageEvent{
		Message: domain.Message{
			Role:     domain.RoleAssistant,
			Content:  "Simple response.",
			Thinking: "",
		},
	}

	if err := emitter.HandleMessageEvent(context.Background(), event); err != nil {
		t.Fatalf("HandleMessageEvent error: %v", err)
	}

	evts := parseSSEData(t, rec.Body.Bytes())

	// Only 3 text events, no reasoning.
	if len(evts) != 3 {
		for i, e := range evts {
			t.Logf("event[%d]: %v", i, e["type"])
		}
		t.Fatalf("got %d events, want 3", len(evts))
	}

	assertSSEEventType(t, evts[0], events.EventTypeTextMessageStart)
	assertSSEEventType(t, evts[1], events.EventTypeTextMessageContent)
	assertSSEEventType(t, evts[2], events.EventTypeTextMessageEnd)
}

func TestAGUIEmitter_NoReasoningForUserMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, err := NewSSEWriter(rec, nil)
	if err != nil {
		t.Fatalf("creating SSEWriter: %v", err)
	}

	emitter := NewAGUIEmitter(sse, nil)

	event := domain.MessageEvent{
		Message: domain.Message{
			Role:     domain.RoleUser,
			Content:  "hi",
			Thinking: "this should not emit anything",
		},
	}

	if err := emitter.HandleMessageEvent(context.Background(), event); err != nil {
		t.Fatalf("HandleMessageEvent error: %v", err)
	}

	evts := parseSSEData(t, rec.Body.Bytes())
	if len(evts) != 0 {
		t.Errorf("got %d events for user message with thinking, want 0", len(evts))
	}
}

func TestAGUIEmitter_ReasoningCancelledContext(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, err := NewSSEWriter(rec, nil)
	if err != nil {
		t.Fatalf("creating SSEWriter: %v", err)
	}

	emitter := NewAGUIEmitter(sse, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := domain.MessageEvent{
		Message: domain.Message{
			Role:     domain.RoleAssistant,
			Content:  "answer",
			Thinking: "thinking...",
		},
	}

	if err := emitter.HandleMessageEvent(ctx, event); err != nil {
		t.Fatalf("expected nil error on cancelled context, got: %v", err)
	}

	// No events should be written when context is cancelled.
	evts := parseSSEData(t, rec.Body.Bytes())
	if len(evts) != 0 {
		t.Errorf("got %d events on cancelled context, want 0", len(evts))
	}
}

func assertSSEEventType(t *testing.T, m map[string]any, want events.EventType) {
	t.Helper()
	got, _ := m["type"].(string)
	if got != string(want) {
		t.Errorf("event type = %q, want %q", got, want)
	}
}
