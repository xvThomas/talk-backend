package domain

import "context"

// ThinkingEffort controls the level of reasoning/thinking requested from the model.
type ThinkingEffort string

const (
	ThinkingOff    ThinkingEffort = "off"
	ThinkingLow    ThinkingEffort = "low"
	ThinkingMedium ThinkingEffort = "medium"
	ThinkingHigh   ThinkingEffort = "high"
)

// CompletionOptions holds optional parameters for a single LLM completion call.
type CompletionOptions struct {
	ThinkingEffort ThinkingEffort
}

// LlmClient is the unified interface for any LLM backend.
type LlmClient interface {
	// Complete sends the conversation to the model and returns the next message
	// together with the token usage for this API call.
	// systemPrompt is passed separately because some providers (Anthropic) treat
	// it outside the message list.
	Complete(ctx context.Context, systemPrompt string, messages []Message, tools []Tool, opts CompletionOptions) (*Message, Usage, error)
}
