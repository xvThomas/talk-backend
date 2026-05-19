package prompt

import "context"

// StaticProvider implements internal.PromptProvider with a fixed string.
type StaticProvider struct {
	text string
}

// NewStaticProvider creates a StaticProvider with the given prompt text.
func NewStaticProvider(text string) *StaticProvider {
	return &StaticProvider{text: text}
}

// SystemPrompt returns the fixed prompt text.
func (p *StaticProvider) SystemPrompt(_ context.Context) (string, error) {
	return p.text, nil
}
