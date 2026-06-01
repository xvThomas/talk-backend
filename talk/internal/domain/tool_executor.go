package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ToolExecutor handles the execution of tool calls and returns messages
// that can be added to the conversation store.
type ToolExecutor struct {
	toolsProvider func() []Tool
}

// NewToolExecutor creates a new ToolExecutor with the given tools provider.
func NewToolExecutor(toolsProvider func() []Tool) *ToolExecutor {
	return &ToolExecutor{toolsProvider: toolsProvider}
}

// ExecuteTool executes a single tool call and returns the result.
func (e *ToolExecutor) ExecuteTool(ctx context.Context, call ToolCall) (ToolResult, error) {
	for _, t := range e.toolsProvider() {
		if t.Name() == call.Name {
			content, err := t.Execute(ctx, call.Input)
			if err != nil {
				return ToolResult{}, fmt.Errorf("tool %q execution: %w", call.Name, err)
			}
			contentBytes, err := json.Marshal(content)
			if err != nil {
				return ToolResult{}, fmt.Errorf("marshalling tool output for tool %q: %w", call.Name, err)
			}
			return ToolResult{ToolCallID: call.ID, Content: string(contentBytes)}, nil
		}
	}
	return ToolResult{}, fmt.Errorf("unknown tool %q", call.Name)
}

// ExecuteToolCalls executes tool calls sequentially and returns messages to be stored.
func (e *ToolExecutor) ExecuteToolCalls(ctx context.Context, turnID string, calls []ToolCall) ([]Message, error) {
	messages := make([]Message, 0, len(calls))
	for _, call := range calls {
		result, err := e.ExecuteTool(ctx, call)
		if err != nil {
			return nil, err
		}
		messages = append(messages, Message{
			Role:        RoleTool,
			ToolCalls:   []ToolCall{call},
			ToolResults: []ToolResult{result},
			TurnID:      turnID,
		})
	}
	return messages, nil
}

// ExecuteToolCallsParallel executes tool calls in parallel and returns messages to be stored.
func (e *ToolExecutor) ExecuteToolCallsParallel(ctx context.Context, turnID string, calls []ToolCall, maxConcurrent int) ([]Message, error) {
	results := make([]ToolResult, len(calls))
	errors := make([]error, len(calls))

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for i, call := range calls {
		wg.Add(1)
		go func(idx int, toolCall ToolCall) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := e.ExecuteTool(ctx, toolCall)
			if err != nil {
				errors[idx] = err
				return
			}
			results[idx] = result
		}(i, call)
	}

	wg.Wait()

	for i, err := range errors {
		if err != nil {
			return nil, fmt.Errorf("tool %q failed: %w", calls[i].Name, err)
		}
	}

	messages := make([]Message, 0, len(calls))
	for i, call := range calls {
		messages = append(messages, Message{
			Role:        RoleTool,
			ToolCalls:   []ToolCall{call},
			ToolResults: []ToolResult{results[i]},
			TurnID:      turnID,
		})
	}
	return messages, nil
}