package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateTraceID generates a random 16-byte trace ID as a hex string.
func GenerateTraceID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateSpanID generates a random 8-byte span ID as a hex string.
func GenerateSpanID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateSessionID generates a random UUID v4 as a formatted string.
func GenerateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Usage holds token consumption for a single LLM API call.
type Usage struct {
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64 // tokens served from prompt cache
	CacheWriteTokens int64 // tokens written to prompt cache (Anthropic only)
}

// Add returns the sum of two Usage values.
func (u Usage) Add(other Usage) Usage {
	return Usage{
		InputTokens:      u.InputTokens + other.InputTokens,
		OutputTokens:     u.OutputTokens + other.OutputTokens,
		CacheReadTokens:  u.CacheReadTokens + other.CacheReadTokens,
		CacheWriteTokens: u.CacheWriteTokens + other.CacheWriteTokens,
	}
}

// CallKind classifies the type of LLM call within a conversation turn.
type CallKind string

const (
	// CallKindInitial is the first call of a turn (user → assistant).
	CallKindInitial CallKind = "initial"
	// CallKindToolResult is a subsequent call after tool execution.
	CallKindToolResult CallKind = "tool_result"
)

// APICallEvent is emitted after each individual Complete() invocation.
type APICallEvent struct {
	TraceID      string    // Shared trace ID for the parent turn
	ParentSpanID string    // SpanID of the parent conversation_turn span
	Provider     Provider  // LLM provider (anthropic, openai, mistral)
	StartedAt    time.Time // When the API call started
	EndedAt      time.Time // When the API call completed
	Model        string
	Kind         CallKind
	Usage        Usage
	Input        string     // The input prompt for this API call
	Output       string     // The response content from the model
	ToolCalls    []ToolCall // Tool calls made in this API call (if any)
	SessionID    string     // Session identifier shared across the CLI session
	UserID       string     // User identifier ("anonymous" until auth is added)
}

// TurnEvent is emitted once at the end of a full Chat() turn (all calls aggregated).
type TurnEvent struct {
	TraceID    string    // Trace ID shared with child API call spans
	SpanID     string    // Span ID for this turn (parent of API call spans)
	StartedAt  time.Time // When the conversation turn started
	EndedAt    time.Time // When the conversation turn completed
	Model      string
	TotalUsage Usage
	CallCount  int
	Input      string     // The original user question
	Output     string     // The final assistant response
	ToolCalls  []ToolCall // All tool calls made during this turn
	SessionID  string     // Session identifier shared across the CLI session
	UserID     string     // User identifier ("anonymous" until auth is added)
}

// UsageReporter receives usage telemetry events.
type UsageReporter interface {
	OnAPICall(event APICallEvent)
	OnConversationTurn(event TurnEvent)
}

// NoOpUsageReporter silently discards all events.
type NoOpUsageReporter struct{}

func (NoOpUsageReporter) OnAPICall(APICallEvent)       {}
func (NoOpUsageReporter) OnConversationTurn(TurnEvent) {}
