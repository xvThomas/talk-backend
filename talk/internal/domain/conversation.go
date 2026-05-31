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
	client             LlmClient
	modelID            string
	provider           Provider
	store              MessageStore
	promptProvider     PromptProvider
	toolsProvider      func() []Tool
	reporters          []UsageReporter // Multiple reporters for parallel execution
	maxConcurrentTools int             // Maximum concurrent tool executions
}

// NewConversationManager creates a ConversationManager.
func NewConversationManager(client LlmClient, modelID string, provider Provider, store MessageStore, pp PromptProvider, tools func() []Tool, reporters []UsageReporter, maxConcurrentTools int) *ConversationManager {
	return &ConversationManager{
		client:             client,
		modelID:            modelID,
		provider:           provider,
		store:              store,
		promptProvider:     pp,
		toolsProvider:      tools,
		reporters:          reporters,
		maxConcurrentTools: maxConcurrentTools,
	}
}

// sessionID returns the session ID from the store.
func (m *ConversationManager) sessionID() string { return m.store.SessionID() }

// userID returns the user ID from the store.
func (m *ConversationManager) userID() string { return m.store.UserID() }

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

	turnTraceID := GenerateTraceID()
	m.store.Add(Message{Role: RoleUser, Content: userInput, TurnID: turnTraceID})

	turnStartedAt := time.Now()
	turnSpanID := GenerateSpanID()
	var totalUsage Usage
	var allToolCalls []ToolCall // Collect all tool calls for the turn event
	var lastCallEndedAt time.Time
	callCount := 0
	kind := CallKindInitial

	for range maxToolCalls {
		// Get the current conversation context for the API call input
		messages := m.store.All()
		conversationInput := formatMessagesAsInput(messages, systemPrompt)

		callStartedAt := time.Now()
		response, usage, err := m.client.Complete(ctx, systemPrompt, messages, m.toolsProvider())
		if err != nil {
			return "", fmt.Errorf("model completion: %w", err)
		}
		lastCallEndedAt = time.Now()

		// Report API call with input, output, and tool calls
		m.reportAPICall(APICallEvent{
			TraceID:      turnTraceID,
			ParentSpanID: turnSpanID,
			Provider:     m.provider,
			StartedAt:    callStartedAt,
			EndedAt:      lastCallEndedAt,
			Model:        m.modelID,
			Kind:         kind,
			Usage:        usage,
			Input:        conversationInput,
			Output:       formatAPICallOutput(response.Content, response.ToolCalls),
			ToolCalls:    response.ToolCalls,
			SessionID:    m.sessionID(),
			UserID:       m.userID(),
		})

		totalUsage = totalUsage.Add(usage)
		allToolCalls = append(allToolCalls, response.ToolCalls...)
		callCount++
		kind = CallKindToolResult

		storedResponse := *response
		storedResponse.TurnID = turnTraceID
		if strings.TrimSpace(storedResponse.Content) == "" && len(storedResponse.ToolCalls) > 0 {
			storedResponse.Content = formatToolCallSummary(storedResponse.ToolCalls)
		}
		m.store.Add(storedResponse)

		if len(response.ToolCalls) == 0 {
			m.reportConversationTurn(TurnEvent{
				TraceID:    turnTraceID,
				SpanID:     turnSpanID,
				StartedAt:  turnStartedAt,
				EndedAt:    time.Now(),
				Model:      m.modelID,
				TotalUsage: totalUsage,
				CallCount:  callCount,
				Input:      userInput,
				Output:     response.Content,
				ToolCalls:  allToolCalls,
				SessionID:  m.sessionID(),
				UserID:     m.userID(),
			})
			return response.Content, nil
		}

		if err := m.executeToolCalls(ctx, turnTraceID, response.ToolCalls); err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("exceeded maximum tool call iterations (%d)", maxToolCalls)
}

func (m *ConversationManager) executeToolCalls(ctx context.Context, turnID string, calls []ToolCall) error {
	if len(calls) == 1 || m.maxConcurrentTools <= 1 {
		// Execute sequentially for single calls or when concurrency is disabled
		return m.executeToolCallsSequential(ctx, turnID, calls)
	}
	// Execute in parallel with concurrency limit
	return m.executeToolCallsParallel(ctx, turnID, calls)
}

func (m *ConversationManager) executeToolCallsSequential(ctx context.Context, turnID string, calls []ToolCall) error {
	for _, call := range calls {
		result, err := m.executeTool(ctx, call)
		if err != nil {
			return err
		}
		m.store.Add(Message{
			Role:        RoleTool,
			ToolCalls:   []ToolCall{call},
			ToolResults: []ToolResult{result},
			TurnID:      turnID,
		})
	}
	return nil
}

func (m *ConversationManager) executeToolCallsParallel(ctx context.Context, turnID string, calls []ToolCall) error {
	results := make([]ToolResult, len(calls))
	errors := make([]error, len(calls))

	// Limit concurrency using a semaphore pattern
	sem := make(chan struct{}, m.maxConcurrentTools)
	var wg sync.WaitGroup

	for i, call := range calls {
		wg.Add(1)
		go func(idx int, toolCall ToolCall) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := m.executeTool(ctx, toolCall)
			if err != nil {
				errors[idx] = err
				return
			}
			results[idx] = result
		}(i, call)
	}

	wg.Wait()

	// Check for any errors and return the first one found
	for i, err := range errors {
		if err != nil {
			return fmt.Errorf("tool %q failed: %w", calls[i].Name, err)
		}
	}

	for i, call := range calls {
		m.store.Add(Message{
			Role:        RoleTool,
			ToolCalls:   []ToolCall{call},
			ToolResults: []ToolResult{results[i]},
			TurnID:      turnID,
		})
	}
	return nil
}

func (m *ConversationManager) executeTool(ctx context.Context, call ToolCall) (ToolResult, error) {
	for _, t := range m.toolsProvider() {
		if t.Name() == call.Name {
			content, err := t.Execute(ctx, call.Input)
			if err != nil {
				return ToolResult{}, fmt.Errorf("tool %q execution: %w", call.Name, err)
			}
			// ici convertir content (map[string]any) en string pour le stocker dans ToolResult.Content
			// en supposant que le résultat de l'outil est une carte, nous allons le marshaller en JSON pour le stocker comme string
			contentBytes, err := json.Marshal(content)
			if err != nil {
				return ToolResult{}, fmt.Errorf("marshalling tool output for tool %q: %w", call.Name, err)
			}
			return ToolResult{ToolCallID: call.ID, Content: string(contentBytes)}, nil
			// return ToolResult{ToolCallID: call.ID, Content: content}, nil --- IGNORE ---
			// return ToolResult{ToolCallID: call.ID, Content: content.(string)}, nil --- IGNORE ---
		}
	}
	return ToolResult{}, fmt.Errorf("unknown tool %q", call.Name)
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
