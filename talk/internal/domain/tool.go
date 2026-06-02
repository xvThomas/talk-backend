package domain

import "context"

// Tool is the type-erased interface used internally by the conversation
// engine and LLM converters. It works with raw maps and string output.
type Tool interface {
	Name() string
	Description() string
	InputSchema() (map[string]any, error)  // JSON Schema for the input parameters
	OutputSchema() (map[string]any, error) // JSON Schema for the output
	// Execute runs the tool with the given input and returns a map result.
	Execute(ctx context.Context, input map[string]any) (map[string]any, error)
}
