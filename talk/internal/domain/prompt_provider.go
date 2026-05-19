package domain

import "context"

// PromptProvider supplies the system prompt for a conversation.
// Implementations may read from a file, environment variable, database, etc.
type PromptProvider interface {
	SystemPrompt(ctx context.Context) (string, error)
}
