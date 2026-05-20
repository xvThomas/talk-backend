package mcpserver

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// PromptArgument describes a single argument that a prompt accepts.
type PromptArgument struct {
	Name        string
	Description string
	Required    bool
}

// PromptMessage is a single message within a Prompt definition.
type PromptMessage struct {
	Role string // "user" or "assistant"
	Text string
}

// Prompt describes an MCP prompt exposed by the server.
type Prompt struct {
	Name        string
	Description string
	Arguments   []PromptArgument
	Messages    []PromptMessage
}

// PromptRegistrar registers a prompt on an mcp.Server.
// Use RegisterPrompt to create one from a Prompt definition.
type PromptRegistrar struct {
	Name     string
	Register func(s *mcp.Server)
}

// RegisterPrompt returns a PromptRegistrar that adds the given Prompt to an mcp.Server.
func RegisterPrompt(p Prompt) PromptRegistrar {
	return PromptRegistrar{
		Name: p.Name,
		Register: func(s *mcp.Server) {
			args := make([]*mcp.PromptArgument, len(p.Arguments))
			for i, a := range p.Arguments {
				args[i] = &mcp.PromptArgument{
					Name:        a.Name,
					Description: a.Description,
					Required:    a.Required,
				}
			}
			s.AddPrompt(&mcp.Prompt{
				Name:        p.Name,
				Description: p.Description,
				Arguments:   args,
			}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				messages := make([]*mcp.PromptMessage, len(p.Messages))
				for i, m := range p.Messages {
					text := m.Text
					for k, v := range req.Params.Arguments {
						text = strings.ReplaceAll(text, "{{"+k+"}}", v)
					}
					messages[i] = &mcp.PromptMessage{
						Role:    mcp.Role(m.Role),
						Content: &mcp.TextContent{Text: text},
					}
				}
				return &mcp.GetPromptResult{
					Description: p.Description,
					Messages:    messages,
				}, nil
			})
		},
	}
}

// WithPrompts registers prompts on the MCP server.
func WithPrompts(prompts ...PromptRegistrar) Option {
	return func(a *App) { a.prompts = append(a.prompts, prompts...) }
}
