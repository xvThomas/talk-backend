package usage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/xvThomas/LLMClientWrapper/talk-libs/version"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
)

// LangfuseUsageReporter implements domain.UsageReporter by sending traces to Langfuse
// via OpenTelemetry OTLP over HTTP. It buffers events and sends them asynchronously
// for minimal performance impact.
type LangfuseUsageReporter struct {
	httpClient  *http.Client
	baseURL     string
	authHeader  string
	eventBuffer chan traceEvent
	wg          sync.WaitGroup
	cancel      context.CancelFunc
	ctx         context.Context
}

// traceEvent represents an event to be sent to Langfuse
type traceEvent struct {
	eventType string // "api_call" or "conversation_turn"
	data      any
}

// LangfuseConfig holds configuration for the Langfuse client
type LangfuseConfig struct {
	PublicKey string
	SecretKey string
	BaseURL   string
}

var _ domain.UsageReporter = (*LangfuseUsageReporter)(nil) // compile-time interface check

// NewLangfuseUsageReporter creates a new Langfuse usage reporter.
// It starts a background worker to process events asynchronously.
func NewLangfuseUsageReporter(config LangfuseConfig) *LangfuseUsageReporter {
	// Create Basic Auth header: base64(publicKey:secretKey)
	authString := fmt.Sprintf("%s:%s", config.PublicKey, config.SecretKey)
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(authString))

	// Set default base URL if not provided
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://cloud.langfuse.com"
	}

	ctx, cancel := context.WithCancel(context.Background())

	reporter := &LangfuseUsageReporter{
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Reasonable timeout for HTTP requests
		},
		baseURL:     baseURL,
		authHeader:  authHeader,
		eventBuffer: make(chan traceEvent, 1000), // Buffer up to 1000 events
		cancel:      cancel,
		ctx:         ctx,
	}

	// Start background worker
	reporter.wg.Add(1)
	go reporter.worker()

	return reporter
}

// OnAPICall implements domain.UsageReporter by buffering the API call event.
func (l *LangfuseUsageReporter) OnAPICall(event domain.APICallEvent) {
	select {
	case l.eventBuffer <- traceEvent{eventType: "api_call", data: event}:
	case <-l.ctx.Done():
		return
	default:
		// Buffer full, drop event to prevent blocking
		// In production, we might want to log this
	}
}

// OnConversationTurn implements domain.UsageReporter by buffering the conversation turn event.
func (l *LangfuseUsageReporter) OnConversationTurn(event domain.TurnEvent) {
	select {
	case l.eventBuffer <- traceEvent{eventType: "conversation_turn", data: event}:
	case <-l.ctx.Done():
		return
	default:
		// Buffer full, drop event to prevent blocking
	}
}

// Close gracefully shuts down the reporter, flushing any pending events.
func (l *LangfuseUsageReporter) Close() {
	l.cancel()
	close(l.eventBuffer)
	l.wg.Wait()
}

// worker processes events in the background
func (l *LangfuseUsageReporter) worker() {
	defer l.wg.Done()

	for event := range l.eventBuffer {
		_ = l.processEvent(event)
	}
}

// processEvent converts domain events to OpenTelemetry format and sends to Langfuse
func (l *LangfuseUsageReporter) processEvent(event traceEvent) error {
	var otlpTrace *OTLPTrace
	var err error

	switch event.eventType {
	case "api_call":
		apiEvent, ok := event.data.(domain.APICallEvent)
		if !ok {
			return fmt.Errorf("invalid api_call event data")
		}
		otlpTrace, err = l.apiCallToOTLP(apiEvent)
	case "conversation_turn":
		turnEvent, ok := event.data.(domain.TurnEvent)
		if !ok {
			return fmt.Errorf("invalid conversation_turn event data")
		}
		otlpTrace, err = l.conversationTurnToOTLP(turnEvent)
	default:
		return fmt.Errorf("unknown event type: %s", event.eventType)
	}

	if err != nil {
		return fmt.Errorf("converting to OTLP: %w", err)
	}

	return l.sendTrace(otlpTrace)
}

// sendTrace sends an OpenTelemetry trace to Langfuse
func (l *LangfuseUsageReporter) sendTrace(trace *OTLPTrace) error {
	payload := &OTLPTracesPayload{
		ResourceSpans: []OTLPResourceSpans{
			{
				Resource: OTLPResource{
					Attributes: []OTLPAttribute{
						{Key: "service.name", Value: stringValue("talks")},
						{Key: "service.version", Value: stringValue(version.Version)},
					},
				},
				ScopeSpans: []OTLPScopeSpans{
					{
						Scope: OTLPInstrumentationScope{
							Name:    "talks",
							Version: version.Version,
						},
						Spans: []OTLPSpan{trace.Span},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling OTLP payload: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST",
		l.baseURL+"/api/public/otel/v1/traces", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", l.authHeader)
	req.Header.Set("x-langfuse-ingestion-version", "4")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d from Langfuse", resp.StatusCode)
	}

	return nil
}
