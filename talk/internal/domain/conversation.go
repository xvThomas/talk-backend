package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

const maxToolCalls = 5

// ConversationManager orchestrates a multi-turn conversation with optional tool calls.
type ConversationManager struct {
	scope          SessionScope
	client         LlmClient
	modelID        string
	provider       OLTPProvider
	store          MessageStore
	promptProvider PromptProvider
	toolsProvider  func() []Tool
	reporters      []UsageReporter
	contextBuilder *ContextBuilder
	executor       *ToolExecutor
}

// ConversationManagerConfig groups all parameters for creating a ConversationManager.
type ConversationManagerConfig struct {
	Client             LlmClient
	ModelID            string
	Scope              SessionScope
	Provider           OLTPProvider
	Store              MessageStore
	SessionBrowser     SessionBrowser
	PromptProvider     PromptProvider
	Tools              func() []Tool
	Reporters          []UsageReporter
	MaxConcurrentTools int
	ContextFullTurns   int
}

// NewConversationManager creates a ConversationManager.
func NewConversationManager(cfg ConversationManagerConfig) *ConversationManager {
	return &ConversationManager{
		scope:          cfg.Scope,
		client:         cfg.Client,
		modelID:        cfg.ModelID,
		provider:       cfg.Provider,
		store:          cfg.Store,
		promptProvider: cfg.PromptProvider,
		toolsProvider:  cfg.Tools,
		reporters:      cfg.Reporters,
		contextBuilder: NewContextBuilder(cfg.Store, cfg.SessionBrowser, cfg.Scope.SessionID, cfg.ContextFullTurns),
		executor:       NewToolExecutor(cfg.Tools, cfg.MaxConcurrentTools),
	}
}

// SetScope updates the active session scope for the conversation manager.
func (m *ConversationManager) SetScope(scope SessionScope) {
	m.scope = scope
	m.contextBuilder.sessionID = scope.SessionID
}

// reportAPICall calls OnAPICall on all reporters in parallel.
func (m *ConversationManager) reportAPICall(event APICallEvent) {
	if len(m.reporters) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, reporter := range m.reporters {
		wg.Add(1)
		go func(r UsageReporter) {
			defer wg.Done()
			defer func() {
				recover() //nolint:errcheck // Don't let reporter panics crash the application
			}()
			r.OnAPICall(event)
		}(reporter)
	}
	wg.Wait()
}

// reportConversationTurn calls OnConversationTurn on all reporters in parallel.
func (m *ConversationManager) reportConversationTurn(event TurnEvent) {
	if len(m.reporters) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, reporter := range m.reporters {
		wg.Add(1)
		go func(r UsageReporter) {
			defer wg.Done()
			defer func() {
				recover() //nolint:errcheck // Don't let reporter panics crash the application
			}()
			r.OnConversationTurn(event)
		}(reporter)
	}
	wg.Wait()
}

// SetClient replaces the active LLM client and model without resetting the conversation history.
func (m *ConversationManager) SetClient(client LlmClient, modelID string) {
	m.client = client
	m.modelID = modelID
}

// Chat sends a user message and returns the final assistant text response.
// Tool calls are resolved automatically up to maxToolCalls iterations.
func (m *ConversationManager) Chat(ctx context.Context, userInput string) (string, error) {
	systemPrompt, err := m.promptProvider.SystemPrompt(ctx)
	if err != nil {
		return "", fmt.Errorf("loading system prompt: %w", err)
	}

	// turnID is used to correlate all messages, API calls, and tool calls for this conversation turn in observability.
	turnID := GenerateTraceID()
	// Store the user message in the conversation history before processing to ensure it's included in the context
	// for the first API call and in observability.
	m.store.AddMessage(Message{Role: RoleUser, Content: userInput, TurnID: turnID}, m.scope)

	turnStartedAt := time.Now()
	// turnSpanID is used to correlate all events for this conversation turn in observability. It is the parent span for all API call spans in this turn.
	turnSpanID := GenerateSpanID()
	var totalUsage Usage
	var allToolCalls []ToolCall // Collect all tool calls for the turn event
	var lastCallEndedAt time.Time
	callCount := 0
	// kind tracks whether the API call is the initial LLM call or a subsequent call after tool results, for observability purposes.
	kind := CallKindInitial

	for range maxToolCalls {
		// Get the current conversation context for the API call input
		messages := m.contextBuilder.BuildContextMessages(ctx, turnID)
		conversationInput := formatMessagesAsInput(messages, systemPrompt)

		// Send the conversation to the LLM and get the response with token usage.
		// The response may contain tool calls that need to be executed.
		callStartedAt := time.Now()
		response, usage, err := m.client.Complete(ctx, systemPrompt, messages, m.toolsProvider())
		if err != nil {
			return "", fmt.Errorf("model completion: %w", err)
		}
		lastCallEndedAt = time.Now()

		// Report API call with input, output, and tool calls
		m.reportAPICall(APICallEvent{
			TraceID:      turnID,
			ParentSpanID: turnSpanID,
			OLTPProvider: m.provider,
			StartedAt:    callStartedAt,
			EndedAt:      lastCallEndedAt,
			Model:        m.modelID,
			Kind:         kind,
			Usage:        usage,
			Input:        conversationInput,
			Output:       formatAPICallOutput(response.Content, response.ToolCalls),
			ToolCalls:    response.ToolCalls,
			SessionID:    m.scope.SessionID,
			UserID:       m.scope.UserID,
		})

		totalUsage = totalUsage.Add(usage)
		allToolCalls = append(allToolCalls, response.ToolCalls...)
		callCount++
		kind = CallKindToolResult

		// Store the assistant response in the conversation history,
		// including tool calls as a special message type.
		storedResponse := *response
		storedResponse.TurnID = turnID
		if strings.TrimSpace(storedResponse.Content) == "" && len(storedResponse.ToolCalls) > 0 {
			storedResponse.Content = formatToolCallSummary(storedResponse.ToolCalls)
		}
		m.store.AddMessage(storedResponse, m.scope)

		// If the model responded with content without tool calls, or if we've reached the maximum
		// tool call iterations, end the conversation turn.
		if len(response.ToolCalls) == 0 {
			m.reportConversationTurn(TurnEvent{
				TraceID:    turnID,
				SpanID:     turnSpanID,
				StartedAt:  turnStartedAt,
				EndedAt:    time.Now(),
				Model:      m.modelID,
				TotalUsage: totalUsage,
				CallCount:  callCount,
				Input:      userInput,
				Output:     response.Content,
				ToolCalls:  allToolCalls,
				SessionID:  m.scope.SessionID,
				UserID:     m.scope.UserID,
			})
			return response.Content, nil
		}

		// If the model responded with tool calls, execute them and feed the results back 
		// to the model in the next iteration.
		toolMsgs, err := m.executor.Execute(ctx, turnID, response.ToolCalls)
		if err != nil {
			return "", err
		}
		for _, msg := range toolMsgs {
			m.store.AddMessage(msg, m.scope)
		}
	}

	return "", fmt.Errorf("exceeded maximum tool call iterations (%d)", maxToolCalls)
}

// formatMessagesAsInput formats the conversation messages as a readable input string for observability
// formatMessagesAsInput returns the most relevant input for observability.
// For the initial call it returns the last user message.
// For tool_result calls it returns the tool results that were fed back to the LLM.
func formatMessagesAsInput(messages []Message, systemPrompt string) string {
	if len(messages) == 0 {
		return systemPrompt
	}

	// If the latest messages are tool results, concatenate all contiguous tool
	// outputs because the model receives each tool output as a separate message.
	last := messages[len(messages)-1]
	if last.Role == RoleTool && len(last.ToolResults) > 0 {
		start := len(messages) - 1
		for start > 0 && messages[start-1].Role == RoleTool {
			start--
		}
		var b strings.Builder
		isFirst := true
		for i := start; i < len(messages); i++ {
			for _, tr := range messages[i].ToolResults {
				if !isFirst {
					b.WriteString("\n")
				}
				b.WriteString(tr.Content)
				isFirst = false
			}
		}
		return b.String()
	}

	// Otherwise return the last user message
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleUser {
			return messages[i].Content
		}
	}

	return systemPrompt
}

// formatAPICallOutput returns a human-readable output for an API call.
// When the LLM responds with tool calls instead of text, the content is empty;
// in that case we format the tool calls as JSON so Langfuse shows a meaningful output.
func formatAPICallOutput(content string, toolCalls []ToolCall) string {
	if content != "" {
		return content
	}
	if len(toolCalls) == 0 {
		return ""
	}
	var b strings.Builder
	for i, tc := range toolCalls {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, `{"tool_name": %q, "input": %s}`, tc.Name, marshalInput(tc.Input))
	}
	return b.String()
}

func marshalInput(input map[string]any) string {
	raw, err := json.Marshal(input)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func formatToolCallSummary(toolCalls []ToolCall) string {
	if len(toolCalls) == 0 {
		return ""
	}
	if len(toolCalls) == 1 {
		return fmt.Sprintf("Calling tool %s.", toolCalls[0].Name)
	}

	names := make([]string, 0, len(toolCalls))
	for _, tc := range toolCalls {
		names = append(names, tc.Name)
	}

	return fmt.Sprintf("Calling tools %s.", strings.Join(names, ", "))
}
