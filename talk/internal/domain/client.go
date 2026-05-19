package domain

import "context"

// LlmClient is the unified interface for any LLM backend.
type LlmClient interface {
	// Complete sends the conversation to the model and returns the next message
	// together with the token usage for this API call.
	// systemPrompt is passed separately because some providers (Anthropic) treat
	// it outside the message list.
	Complete(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) (*Message, Usage, error)
}
