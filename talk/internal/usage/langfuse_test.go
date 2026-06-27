package usage

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xvThomas/talk-backend/talk/internal/domain"
)

func TestNewLangfuseUsageReporter_DefaultsAndAuthHeader(t *testing.T) {
	r := NewLangfuseUsageReporter(LangfuseConfig{
		PublicKey: "pk-1",
		SecretKey: "sk-1",
		BaseURL:   "",
	})
	defer r.Close()

	if r.baseURL != "https://cloud.langfuse.com" {
		t.Fatalf("baseURL = %q, want %q", r.baseURL, "https://cloud.langfuse.com")
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("pk-1:sk-1"))
	if r.authHeader != wantAuth {
		t.Fatalf("authHeader = %q, want %q", r.authHeader, wantAuth)
	}
	if r.eventBuffer == nil {
		t.Fatal("eventBuffer should not be nil")
	}
}

func TestLangfuseUsageReporter_HandleMessageEvent_NonAssistantNoBuffer(t *testing.T) {
	r := NewLangfuseUsageReporter(LangfuseConfig{PublicKey: "pk", SecretKey: "sk", BaseURL: "http://localhost"})
	defer r.Close()

	msg := domain.MessageEvent{
		Message: domain.Message{Role: domain.RoleUser},
		Kind:    domain.CallKindInitial,
	}
	if err := r.HandleMessageEvent(context.Background(), msg); err != nil {
		t.Fatalf("HandleMessageEvent unexpected error: %v", err)
	}

	if len(r.eventBuffer) != 0 {
		t.Fatalf("eventBuffer len = %d, want 0", len(r.eventBuffer))
	}
}

func TestLangfuseUsageReporter_HandleMessageEvent_AssistantBuffered(t *testing.T) {
	r := &LangfuseUsageReporter{
		eventBuffer: make(chan traceEvent, 1),
		ctx:         context.Background(),
	}

	msg := domain.MessageEvent{
		Message: domain.Message{Role: domain.RoleAssistant},
		Kind:    domain.CallKindInitial,
	}
	if err := r.HandleMessageEvent(context.Background(), msg); err != nil {
		t.Fatalf("HandleMessageEvent unexpected error: %v", err)
	}

	if len(r.eventBuffer) != 1 {
		t.Fatalf("eventBuffer len = %d, want 1", len(r.eventBuffer))
	}
}

func TestLangfuseUsageReporter_HandleTurnEvent_BufferFullDrops(t *testing.T) {
	r := &LangfuseUsageReporter{
		eventBuffer: make(chan traceEvent, 1),
		ctx:         context.Background(),
	}

	// Fill buffer first.
	r.eventBuffer <- traceEvent{eventType: "turn_event", data: domain.TurnEvent{}}

	if err := r.HandleTurnEvent(context.Background(), domain.TurnEvent{}); err != nil {
		t.Fatalf("HandleTurnEvent unexpected error: %v", err)
	}

	if len(r.eventBuffer) != 1 {
		t.Fatalf("eventBuffer len = %d, want 1 (drop on full)", len(r.eventBuffer))
	}
}

func TestLangfuseUsageReporter_HandleToolCalls_NoOp(t *testing.T) {
	r := &LangfuseUsageReporter{}
	if err := r.HandleToolCallStart(context.Background(), domain.ToolCallEvent{}); err != nil {
		t.Fatalf("HandleToolCallStart unexpected error: %v", err)
	}
	if err := r.HandleToolCallEnd(context.Background(), domain.ToolCallEndEvent{}); err != nil {
		t.Fatalf("HandleToolCallEnd unexpected error: %v", err)
	}
}

func TestLangfuseUsageReporter_ProcessEvent_UnknownType(t *testing.T) {
	r := &LangfuseUsageReporter{}
	err := r.processEvent(traceEvent{eventType: "unknown", data: nil})
	if err == nil || !strings.Contains(err.Error(), "unknown event type") {
		t.Fatalf("processEvent error = %v, want unknown event type", err)
	}
}

func TestLangfuseUsageReporter_ProcessEvent_InvalidPayloadType(t *testing.T) {
	r := &LangfuseUsageReporter{}

	err := r.processEvent(traceEvent{eventType: "message_event", data: "bad"})
	if err == nil || !strings.Contains(err.Error(), "invalid message event data") {
		t.Fatalf("message_event invalid payload error = %v", err)
	}

	err = r.processEvent(traceEvent{eventType: "turn_event", data: "bad"})
	if err == nil || !strings.Contains(err.Error(), "invalid turn event data") {
		t.Fatalf("turn_event invalid payload error = %v", err)
	}
}

func TestLangfuseUsageReporter_SendTrace_SuccessAndErrors(t *testing.T) {
	trace := &OTLPTrace{Span: OTLPSpan{TraceID: "trace", SpanID: "span", Name: "n"}}

	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", req.Method)
			}
			if req.URL.Path != "/api/public/otel/v1/traces" {
				t.Fatalf("path = %s, want /api/public/otel/v1/traces", req.URL.Path)
			}
			if req.Header.Get("Authorization") != "Basic test-auth" {
				t.Fatalf("authorization header mismatch: %q", req.Header.Get("Authorization"))
			}
			if req.Header.Get("x-langfuse-ingestion-version") != "4" {
				t.Fatalf("ingestion header mismatch: %q", req.Header.Get("x-langfuse-ingestion-version"))
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		r := &LangfuseUsageReporter{
			httpClient: server.Client(),
			baseURL:    server.URL,
			authHeader: "Basic test-auth",
		}
		if err := r.sendTrace(trace); err != nil {
			t.Fatalf("sendTrace unexpected error: %v", err)
		}
	})

	t.Run("http error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		r := &LangfuseUsageReporter{httpClient: server.Client(), baseURL: server.URL, authHeader: "Basic test-auth"}
		err := r.sendTrace(trace)
		if err == nil || !strings.Contains(err.Error(), "HTTP 400") {
			t.Fatalf("sendTrace error = %v, want HTTP 400", err)
		}
	})

	t.Run("transport error", func(t *testing.T) {
		r := &LangfuseUsageReporter{httpClient: &http.Client{Timeout: 10 * time.Millisecond}, baseURL: "http://127.0.0.1:1", authHeader: "Basic test-auth"}
		err := r.sendTrace(trace)
		if err == nil || !strings.Contains(err.Error(), "sending request") {
			t.Fatalf("sendTrace error = %v, want sending request error", err)
		}
	})
}

func TestLangfuseUsageReporter_OTLPConversions(t *testing.T) {
	r := &LangfuseUsageReporter{}
	now := time.Now()

	msg := domain.MessageEvent{
		Message: domain.Message{
			Role:      domain.RoleAssistant,
			TurnID:    domain.GenerateTraceID(),
			ToolCalls: []domain.ToolCall{{ID: "tc-1", Name: "weather", Input: map[string]any{"city": "Paris"}}},
		},
		Model: domain.Model{OLTPProvider: domain.OLTPProviderAnthropic, APIModelID: "claude-sonnet-4-5"},
		Kind:  domain.CallKindInitial,
		APICall: domain.APICallEvent{
			Input:  "hello",
			Output: "world",
		},
		Usage:      domain.Usage{InputTokens: 10, OutputTokens: 20},
		StartedAt:  now.Add(-2 * time.Second),
		EndedAt:    now,
		TurnSpanID: domain.GenerateSpanID(),
	}
	msgTrace, err := r.apiCallToOTLP(msg)
	if err != nil {
		t.Fatalf("apiCallToOTLP unexpected error: %v", err)
	}
	if msgTrace == nil || msgTrace.Span.TraceID == "" || msgTrace.Span.SpanID == "" {
		t.Fatalf("apiCallToOTLP produced invalid trace/span IDs: %+v", msgTrace)
	}
	if msgTrace.Span.Name != "llm_initial" {
		t.Fatalf("apiCallToOTLP span name = %q, want %q", msgTrace.Span.Name, "llm_initial")
	}

	turn := domain.TurnEvent{
		TurnID:       domain.GenerateTraceID(),
		TurnSpanID:   domain.GenerateSpanID(),
		StartedAt:    now.Add(-4 * time.Second),
		EndedAt:      now,
		SessionScope: domain.NewSessionScope("session-1", "user-1"),
		Model:        domain.Model{Name: "sonnet-4.6"},
		TotalUsage:   domain.Usage{InputTokens: 30, OutputTokens: 40},
		CallCount:    2,
		Input:        "turn input",
		Output:       "turn output",
		ToolCalls:    []domain.ToolCall{{ID: "tc-2", Name: "search"}},
	}
	turnTrace, err := r.conversationTurnToOTLP(turn)
	if err != nil {
		t.Fatalf("conversationTurnToOTLP unexpected error: %v", err)
	}
	if turnTrace == nil || turnTrace.Span.TraceID == "" || turnTrace.Span.SpanID == "" {
		t.Fatalf("conversationTurnToOTLP produced invalid trace/span IDs: %+v", turnTrace)
	}
	if turnTrace.Span.Name != "conversation_turn" {
		t.Fatalf("conversationTurnToOTLP span name = %q, want %q", turnTrace.Span.Name, "conversation_turn")
	}
}
