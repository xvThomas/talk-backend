package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrSystemPrompt is returned when the system prompt cannot be loaded.
var ErrSystemPrompt = errors.New("loading system prompt")

// ErrMaxToolIterations is returned when the tool execution loop exhausts its iteration budget.
var ErrMaxToolIterations = errors.New("maximum tool call iterations exceeded")

const maxToolCalls = 5

// ConversationManager orchestrates a multi-turn conversation with optional tool calls.
type ConversationManager struct {
	sessionScope   SessionScope
	llmClient      LlmClient
	modelID        string
	oltpProvider   OLTPProvider
	messageStore   MessageStore
	promptProvider PromptProvider
	toolsProvider  func() []Tool
	messageHandler MessageEventHandler
	contextBuilder *ContextBuilder
	toolExecutor   *ToolExecutor
	thinkingEffort ThinkingEffort
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
	EventHandlers      MessageEventHandler
	MaxConcurrentTools int
	ContextFullTurns   int
}

// NewConversationManager creates a ConversationManager.
func NewConversationManager(cfg ConversationManagerConfig) *ConversationManager {
	messageHandler := cfg.EventHandlers
	if messageHandler == nil {
		messageHandler = NoOpMessageEventHandler{}
	}

	return &ConversationManager{
		sessionScope:   cfg.Scope,
		llmClient:      cfg.Client,
		modelID:        cfg.ModelID,
		oltpProvider:   cfg.Provider,
		messageStore:   cfg.Store,
		promptProvider: cfg.PromptProvider,
		toolsProvider:  cfg.Tools,
		messageHandler: messageHandler,
		contextBuilder: NewContextBuilder(cfg.Store, cfg.SessionBrowser, cfg.Scope.SessionID(), cfg.ContextFullTurns),
		toolExecutor:   NewToolExecutor(cfg.Tools, cfg.MaxConcurrentTools, messageHandler),
	}
}

// SetScope updates the active session scope for the conversation manager.
func (m *ConversationManager) SetScope(scope SessionScope) {
	m.sessionScope = scope
	m.contextBuilder.sessionID = scope.SessionID()
}

// SetClient replaces the active LLM client and model without resetting the conversation history.
func (m *ConversationManager) SetClient(client LlmClient, modelID string) {
	m.llmClient = client
	m.modelID = modelID
}

// SetThinkingEffort changes the thinking/reasoning level for subsequent LLM calls.
func (m *ConversationManager) SetThinkingEffort(effort ThinkingEffort) {
	m.thinkingEffort = effort
}

// ThinkingEffort returns the current thinking/reasoning level.
func (m *ConversationManager) ThinkingEffort() ThinkingEffort {
	return m.thinkingEffort
}

// Chat sends a user message and returns the final assistant text response.
// Tool calls are resolved automatically up to maxToolCalls iterations.
func (m *ConversationManager) Chat(ctx context.Context, userInput string) (string, error) {
	systemPrompt, err := m.promptProvider.SystemPrompt(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrSystemPrompt, err)
	}

	// turnID is used to correlate all messages, API calls, and tool calls for this conversation turn in observability.
	turnID := GenerateTraceID()
	// turnSpanID is used to correlate all events for this conversation turn in observability. It is the parent span for all API call spans in this turn.
	turnSpanID := GenerateSpanID()
	turnStartedAt := time.Now()
	model := Model{Name: m.modelID, OLTPProvider: m.oltpProvider}
	// Store the user message in the conversation history before processing to ensure it's included in the context
	// for the first API call and in observability.
	if err := m.messageHandler.HandleMessageEvent(ctx, MessageEvent{
		Message:      Message{Role: RoleUser, Content: userInput, TurnID: turnID},
		SessionScope: m.sessionScope,
		Model:        model,
		TurnSpanID:   turnSpanID,
		Kind:         CallKindInitial,
		StartedAt:    turnStartedAt,
	}); err != nil {
		return "", fmt.Errorf("handling user message event: %w", err)
	}
	var totalUsage Usage
	allToolCalls := make([]ToolCall, 0, 8) // pre-allocate to avoid repeated growth
	var lastCallEndedAt time.Time
	callCount := 0
	// kind tracks whether the API call is the initial LLM call or a subsequent call after tool results, for observability purposes.
	kind := CallKindInitial

	// Resolve tools once for the entire turn — they don't change between iterations.
	tools := m.toolsProvider()

	for range maxToolCalls {
		// Get the current conversation context for the API call input
		messages := m.contextBuilder.BuildContextMessages(ctx, turnID)
		conversationInput := formatMessagesAsInput(messages, systemPrompt)

		// Send the conversation to the LLM and get the response with token usage.
		// The response may contain tool calls that need to be executed.
		callStartedAt := time.Now()
		response, usage, err := m.llmClient.Complete(ctx, systemPrompt, messages, tools, CompletionOptions{
			ThinkingEffort: m.thinkingEffort,
		})
		if err != nil {
			return "", fmt.Errorf("model completion: %w", err)
		}
		lastCallEndedAt = time.Now()

		totalUsage = totalUsage.Add(usage)
		allToolCalls = append(allToolCalls, response.ToolCalls...)
		callCount++

		// Store the assistant response in the conversation history,
		// including tool calls as a special message type.
		storedResponse := *response
		storedResponse.TurnID = turnID
		if strings.TrimSpace(storedResponse.Content) == "" && len(storedResponse.ToolCalls) > 0 {
			storedResponse.Content = formatToolCallSummary(storedResponse.ToolCalls)
		}
		if err := m.messageHandler.HandleMessageEvent(ctx, MessageEvent{
			Message:      storedResponse,
			SessionScope: m.sessionScope,
			Model:        model,
			TurnSpanID:   turnSpanID,
			Kind:         kind,
			Usage:        usage,
			StartedAt:    callStartedAt,
			EndedAt:      lastCallEndedAt,
			APICall: APICallEvent{
				StartedAt: callStartedAt,
				EndedAt:   lastCallEndedAt,
				Input:     conversationInput,
				Output:    formatAPICallOutput(response.Content, response.ToolCalls),
			},
		}); err != nil {
			return "", fmt.Errorf("handling assistant message event: %w", err)
		}

		kind = CallKindToolResult

		// If the model responded with content without tool calls, or if we've reached the maximum
		// tool call iterations, end the conversation turn.
		if len(response.ToolCalls) == 0 {
			if err := m.messageHandler.HandleTurnEvent(ctx, TurnEvent{
				TurnID:       turnID,
				TurnSpanID:   turnSpanID,
				StartedAt:    turnStartedAt,
				EndedAt:      time.Now(),
				SessionScope: m.sessionScope,
				Model:        model,
				TotalUsage:   totalUsage,
				CallCount:    callCount,
				Input:        userInput,
				Output:       response.Content,
				ToolCalls:    allToolCalls,
			}); err != nil {
				return "", fmt.Errorf("handling turn event: %w", err)
			}
			return response.Content, nil
		}

		// If the model responded with tool calls, execute them and feed the results back
		// to the model in the next iteration.
		toolExecutions, err := m.toolExecutor.Execute(ctx, turnID, response.ToolCalls)
		if err != nil {
			return "", err
		}
		for _, toolExecution := range toolExecutions {
			if err := m.messageHandler.HandleMessageEvent(ctx, MessageEvent{
				Message:      toolExecution.Message,
				SessionScope: m.sessionScope,
				Model:        model,
				TurnSpanID:   turnSpanID,
				Kind:         CallKindToolResult,
				StartedAt:    toolExecution.StartedAt,
				EndedAt:      toolExecution.EndedAt,
			}); err != nil {
				return "", fmt.Errorf("handling tool result event: %w", err)
			}
		}
	}

	return "", ErrMaxToolIterations
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
	var b strings.Builder
	if len(toolCalls) == 1 {
		b.WriteString("Calling tool ")
		b.WriteString(toolCalls[0].Name)
		b.WriteByte('.')
		return b.String()
	}
	b.WriteString("Calling tools ")
	for i, tc := range toolCalls {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(tc.Name)
	}
	b.WriteByte('.')
	return b.String()
}
