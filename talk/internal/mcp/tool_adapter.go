package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
)

// mcpToolAdapter adapts an MCP remote tool into the domain.Tool interface.
type mcpToolAdapter struct {
	serverName string
	tool       mcp.Tool
	session    *mcp.ClientSession
}

func (a *mcpToolAdapter) Name() string {
	return a.tool.Name
}

func (a *mcpToolAdapter) Description() string {
	return a.tool.Description
}

func (a *mcpToolAdapter) InputSchema() (map[string]any, error) {
	if a.tool.InputSchema == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}, nil
	}
	// InputSchema is typed as `any` in the MCP SDK. After JSON decoding from
	// ListTools it is typically already a map[string]any — try a direct assertion
	// to avoid a marshal/unmarshal round-trip.
	if m, ok := a.tool.InputSchema.(map[string]any); ok {
		return m, nil
	}
	// Fallback for non-map types (e.g. a typed struct passed in tests).
	b, err := json.Marshal(a.tool.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("marshalling input schema: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("unmarshalling input schema: %w", err)
	}
	return m, nil
}

func (a *mcpToolAdapter) OutputSchema() (map[string]any, error) {
	// MCP tools do not expose an output schema; return a generic one.
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"content": map[string]any{"type": "string"},
		},
	}, nil
}

func (a *mcpToolAdapter) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	result, err := a.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      a.tool.Name,
		Arguments: input,
	})
	if err != nil {
		return nil, fmt.Errorf("calling tool %q on server %q: %w", a.tool.Name, a.serverName, err)
	}
	if result.IsError {
		return nil, fmt.Errorf("tool %q returned error: %s", a.tool.Name, extractTextContent(result.Content))
	}
	return map[string]any{"content": extractTextContent(result.Content)}, nil
}

// extractTextContent concatenates text content from an MCP tool result.
func extractTextContent(content []mcp.Content) string {
	var text string
	for _, c := range content {
		if tc, ok := c.(*mcp.TextContent); ok {
			if text != "" {
				text += "\n"
			}
			text += tc.Text
		}
	}
	return text
}

var _ domain.Tool = (*mcpToolAdapter)(nil)
