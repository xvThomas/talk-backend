package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ToolExecutionResult captures a single tool execution with its timing metadata.
type ToolExecutionResult struct {
	Message   Message
	StartedAt time.Time
	EndedAt   time.Time
}

// ToolExecutor handles the execution of tool calls and returns messages
// that can be added to the conversation store.
type ToolExecutor struct {
	toolsProvider func() []Tool
	maxConcurrent int
	eventHandler  MessageEventHandler
}

// NewToolExecutor creates a new ToolExecutor with the given tools provider and concurrency limit.
func NewToolExecutor(toolsProvider func() []Tool, maxConcurrent int, eventHandler MessageEventHandler) *ToolExecutor {
	return &ToolExecutor{toolsProvider: toolsProvider, maxConcurrent: maxConcurrent, eventHandler: eventHandler}
}

// Execute runs the given tool calls and returns the resulting messages.
// It chooses sequential or parallel execution based on the concurrency configuration.
func (e *ToolExecutor) Execute(ctx context.Context, turnID string, calls []ToolCall) ([]ToolExecutionResult, error) {
	available := e.toolsProvider()
	if len(available) == 0 {
		return nil, fmt.Errorf("model requested tool calls but no tools are registered")
	}

	if len(calls) == 1 || e.maxConcurrent <= 1 {
		return e.executeSequential(ctx, turnID, calls)
	}
	return e.executeParallel(ctx, turnID, calls)
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

func (e *ToolExecutor) executeSequential(ctx context.Context, turnID string, calls []ToolCall) ([]ToolExecutionResult, error) {
	executions := make([]ToolExecutionResult, 0, len(calls))
	for _, call := range calls {
		startedAt := time.Now()
		if e.eventHandler != nil {
			if err := e.eventHandler.HandleToolCallStart(ctx, ToolCallEvent{
				TurnID:    turnID,
				ToolCall:  call,
				StartedAt: startedAt,
			}); err != nil {
				return nil, fmt.Errorf("handling tool call start: %w", err)
			}
		}
		result, execErr := e.ExecuteTool(ctx, call)
		endedAt := time.Now()
		if execErr != nil {
			result = ToolResult{ToolCallID: call.ID, Content: formatToolError(execErr)}
		}
		if e.eventHandler != nil {
			if err := e.eventHandler.HandleToolCallEnd(ctx, ToolCallEndEvent{
				TurnID:    turnID,
				ToolCall:  call,
				Result:    result,
				StartedAt: startedAt,
				EndedAt:   endedAt,
			}); err != nil {
				return nil, fmt.Errorf("handling tool call end: %w", err)
			}
		}
		executions = append(executions, ToolExecutionResult{
			Message: Message{
				Role:        RoleTool,
				ToolCalls:   []ToolCall{call},
				ToolResults: []ToolResult{result},
				TurnID:      turnID,
			},
			StartedAt: startedAt,
			EndedAt:   endedAt,
		})
	}
	return executions, nil
}

func (e *ToolExecutor) executeParallel(ctx context.Context, turnID string, calls []ToolCall) ([]ToolExecutionResult, error) {
	executions := make([]ToolExecutionResult, len(calls))
	errors := make([]error, len(calls))

	sem := make(chan struct{}, e.maxConcurrent)
	var wg sync.WaitGroup

	for i, call := range calls {
		idx, toolCall := i, call
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()

			startedAt := time.Now()
			if e.eventHandler != nil {
				if err := e.eventHandler.HandleToolCallStart(ctx, ToolCallEvent{
					TurnID:    turnID,
					ToolCall:  toolCall,
					StartedAt: startedAt,
				}); err != nil {
					errors[idx] = fmt.Errorf("handling tool call start: %w", err)
					return
				}
			}
			result, execErr := e.ExecuteTool(ctx, toolCall)
			endedAt := time.Now()
			if execErr != nil {
				result = ToolResult{ToolCallID: toolCall.ID, Content: formatToolError(execErr)}
			}
			if e.eventHandler != nil {
				if err := e.eventHandler.HandleToolCallEnd(ctx, ToolCallEndEvent{
					TurnID:    turnID,
					ToolCall:  toolCall,
					Result:    result,
					StartedAt: startedAt,
					EndedAt:   endedAt,
				}); err != nil {
					errors[idx] = fmt.Errorf("handling tool call end: %w", err)
					return
				}
			}
			executions[idx] = ToolExecutionResult{
				Message: Message{
					Role:        RoleTool,
					ToolCalls:   []ToolCall{toolCall},
					ToolResults: []ToolResult{result},
					TurnID:      turnID,
				},
				StartedAt: startedAt,
				EndedAt:   endedAt,
			}
		})
	}

	wg.Wait()

	for i, err := range errors {
		if err != nil {
			return nil, fmt.Errorf("tool %q failed: %w", calls[i].Name, err)
		}
	}

	return executions, nil
}

// formatToolError produces a JSON error payload for a failed tool execution.
func formatToolError(err error) string {
	raw, _ := json.Marshal(map[string]any{
		"ok":    false,
		"error": map[string]string{"kind": "tool_error", "message": err.Error()},
	})
	return string(raw)
}
