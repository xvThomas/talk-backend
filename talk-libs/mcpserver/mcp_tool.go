package mcpserver

import "context"

// MCPTool is the generic interface for MCP tool implementors.
// TInput is the typed input struct; TOutput is the typed output struct.
type MCPTool[TInput any, TOutput any] interface {
	Name() string
	Description() string
	Call(ctx context.Context, input TInput) (TOutput, error)
}
