package usage

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
)

// OpenTelemetry OTLP structures following the standard specification
// https://opentelemetry.io/docs/specs/otlp/

// OTLPTracesPayload is the root structure for OTLP traces
type OTLPTracesPayload struct {
	ResourceSpans []OTLPResourceSpans `json:"resourceSpans"`
}

// OTLPResourceSpans represents spans from a single resource
type OTLPResourceSpans struct {
	Resource   OTLPResource     `json:"resource"`
	ScopeSpans []OTLPScopeSpans `json:"scopeSpans"`
}

// OTLPResource describes the entity that produced the telemetry
type OTLPResource struct {
	Attributes []OTLPAttribute `json:"attributes"`
}

// OTLPScopeSpans represents spans from a single instrumentation scope
type OTLPScopeSpans struct {
	Scope OTLPInstrumentationScope `json:"scope"`
	Spans []OTLPSpan               `json:"spans"`
}

// OTLPInstrumentationScope represents the instrumentation library
type OTLPInstrumentationScope struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// OTLPSpan represents a single span in OpenTelemetry format
type OTLPSpan struct {
	TraceID           string          `json:"traceId"`
	SpanID            string          `json:"spanId"`
	ParentSpanID      string          `json:"parentSpanId,omitempty"`
	Name              string          `json:"name"`
	Kind              int             `json:"kind"`
	StartTimeUnixNano string          `json:"startTimeUnixNano"`
	EndTimeUnixNano   string          `json:"endTimeUnixNano"`
	Attributes        []OTLPAttribute `json:"attributes"`
	Status            OTLPStatus      `json:"status"`
}

// OTLPAttribute represents a key-value pair attribute
type OTLPAttribute struct {
	Key   string    `json:"key"`
	Value OTLPValue `json:"value"`
}

// OTLPValue represents the value of an attribute
type OTLPValue struct {
	StringValue *string  `json:"stringValue,omitempty"`
	IntValue    *int64   `json:"intValue,omitempty"`
	DoubleValue *float64 `json:"doubleValue,omitempty"`
	BoolValue   *bool    `json:"boolValue,omitempty"`
}

// OTLPStatus represents the status of a span
type OTLPStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// OTLPTrace is our internal representation before sending to Langfuse
type OTLPTrace struct {
	Span OTLPSpan
}

// Helper functions to create OTLP values
func stringValue(s string) OTLPValue {
	return OTLPValue{StringValue: &s}
}

func intValue(i int64) OTLPValue {
	return OTLPValue{IntValue: &i}
}

// Mapping functions from domain events to OpenTelemetry format

// apiCallToOTLP converts a domain.APICallEvent to OpenTelemetry format
func (l *LangfuseUsageReporter) apiCallToOTLP(event domain.APICallEvent) (*OTLPTrace, error) {
	traceID := event.TraceID
	spanID := generateSpanID()
	parentSpanID := event.ParentSpanID

	// Provider constants are defined to match OTel GenAI semantic conventions directly.
	system := string(event.Provider)

	attributes := []OTLPAttribute{
		// GenAI semantic conventions
		{Key: "gen_ai.system", Value: stringValue(system)},
		{Key: "gen_ai.request.model", Value: stringValue(event.Model)},
		{Key: "gen_ai.response.model", Value: stringValue(event.Model)},
		{Key: "gen_ai.operation.name", Value: stringValue(string(event.Kind))},

		// Input and output for Langfuse
		{Key: "gen_ai.prompt", Value: stringValue(event.Input)},
		{Key: "gen_ai.completion", Value: stringValue(event.Output)},

		// Langfuse-specific input/output
		{Key: "langfuse.observation.input", Value: stringValue(event.Input)},
		{Key: "langfuse.observation.output", Value: stringValue(event.Output)},

		// Usage information
		{Key: "gen_ai.usage.input_tokens", Value: intValue(event.Usage.InputTokens)},
		{Key: "gen_ai.usage.output_tokens", Value: intValue(event.Usage.OutputTokens)},

		// Anthropic-specific cache information
		{Key: "gen_ai.usage.cache_read_tokens", Value: intValue(event.Usage.CacheReadTokens)},
		{Key: "gen_ai.usage.cache_write_tokens", Value: intValue(event.Usage.CacheWriteTokens)},

		// Langfuse-specific attributes
		{Key: "langfuse.observation.type", Value: stringValue("generation")},
		{Key: "langfuse.observation.level", Value: stringValue("DEFAULT")},
	}

	// Add tool call information if present
	if len(event.ToolCalls) > 0 {
		toolCallsJSON, _ := json.Marshal(event.ToolCalls)
		attributes = append(attributes, OTLPAttribute{
			Key:   "langfuse.observation.metadata.tool_calls",
			Value: stringValue(string(toolCallsJSON)),
		})

		// Add tool names for easier filtering
		var toolNames []string
		for _, tc := range event.ToolCalls {
			toolNames = append(toolNames, tc.Name)
		}
		toolNamesJSON, _ := json.Marshal(toolNames)
		attributes = append(attributes, OTLPAttribute{
			Key:   "langfuse.observation.metadata.tool_names",
			Value: stringValue(string(toolNamesJSON)),
		})
	}

	span := OTLPSpan{
		TraceID:           traceID,
		SpanID:            spanID,
		ParentSpanID:      parentSpanID,
		Name:              fmt.Sprintf("llm_%s", event.Kind),
		Kind:              3, // SPAN_KIND_CLIENT
		StartTimeUnixNano: strconv.FormatInt(event.StartedAt.UnixNano(), 10),
		EndTimeUnixNano:   strconv.FormatInt(event.EndedAt.UnixNano(), 10),
		Attributes:        attributes,
		Status: OTLPStatus{
			Code: 1, // STATUS_CODE_OK
		},
	}

	return &OTLPTrace{Span: span}, nil
}

// conversationTurnToOTLP converts a domain.TurnEvent to OpenTelemetry format
func (l *LangfuseUsageReporter) conversationTurnToOTLP(event domain.TurnEvent) (*OTLPTrace, error) {
	traceID := event.TraceID
	spanID := event.SpanID

	attributes := []OTLPAttribute{
		// Trace-level attributes (Langfuse OTLP mapping)
		{Key: "langfuse.trace.name", Value: stringValue("conversation_turn")},
		{Key: "langfuse.session.id", Value: stringValue(event.SessionID)},
		{Key: "langfuse.user.id", Value: stringValue(event.UserID)},
		{Key: "langfuse.trace.input", Value: stringValue(event.Input)},
		{Key: "langfuse.trace.output", Value: stringValue(event.Output)},

		// Model information
		{Key: "gen_ai.request.model", Value: stringValue(event.Model)},
		{Key: "gen_ai.response.model", Value: stringValue(event.Model)},

		// Input and output for Langfuse
		{Key: "gen_ai.prompt", Value: stringValue(event.Input)},
		{Key: "gen_ai.completion", Value: stringValue(event.Output)},

		// Langfuse observation-level input/output
		{Key: "langfuse.observation.input", Value: stringValue(event.Input)},
		{Key: "langfuse.observation.output", Value: stringValue(event.Output)},

		// Aggregated usage information
		{Key: "gen_ai.usage.input_tokens", Value: intValue(event.TotalUsage.InputTokens)},
		{Key: "gen_ai.usage.output_tokens", Value: intValue(event.TotalUsage.OutputTokens)},
		{Key: "gen_ai.usage.cache_read_tokens", Value: intValue(event.TotalUsage.CacheReadTokens)},
		{Key: "gen_ai.usage.cache_write_tokens", Value: intValue(event.TotalUsage.CacheWriteTokens)},

		// Turn-specific information
		{Key: "call_count", Value: intValue(int64(event.CallCount))},

		// Langfuse-specific attributes
		{Key: "langfuse.observation.type", Value: stringValue("span")},
		{Key: "langfuse.observation.level", Value: stringValue("DEFAULT")},
	}

	// Add tool call information if present
	if len(event.ToolCalls) > 0 {
		toolCallsJSON, _ := json.Marshal(event.ToolCalls)
		attributes = append(attributes, OTLPAttribute{
			Key:   "langfuse.observation.metadata.tool_calls",
			Value: stringValue(string(toolCallsJSON)),
		})

		// Add tool names for easier filtering
		var toolNames []string
		for _, tc := range event.ToolCalls {
			toolNames = append(toolNames, tc.Name)
		}
		toolNamesJSON, _ := json.Marshal(toolNames)
		attributes = append(attributes, OTLPAttribute{
			Key:   "langfuse.observation.metadata.tool_names",
			Value: stringValue(string(toolNamesJSON)),
		})

		// Add tool count
		attributes = append(attributes, OTLPAttribute{
			Key:   "langfuse.observation.metadata.tool_count",
			Value: intValue(int64(len(event.ToolCalls))),
		})
	}

	span := OTLPSpan{
		TraceID:           traceID,
		SpanID:            spanID,
		Name:              "conversation_turn",
		Kind:              1, // SPAN_KIND_INTERNAL
		StartTimeUnixNano: strconv.FormatInt(event.StartedAt.UnixNano(), 10),
		EndTimeUnixNano:   strconv.FormatInt(event.EndedAt.UnixNano(), 10),
		Attributes:        attributes,
		Status: OTLPStatus{
			Code: 1, // STATUS_CODE_OK
		},
	}

	return &OTLPTrace{Span: span}, nil
}
