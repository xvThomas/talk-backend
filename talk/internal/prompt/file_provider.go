package prompt

import (
	"context"
	"fmt"
	"os"
)

// FileProvider implements internal.PromptProvider by reading a Markdown file.
type FileProvider struct {
	path string
}

// NewFileProvider creates a FileProvider for the given file path.
func NewFileProvider(path string) *FileProvider {
	return &FileProvider{path: path}
}

// SystemPrompt reads and returns the content of the prompt file.
func (p *FileProvider) SystemPrompt(_ context.Context) (string, error) {
	data, err := os.ReadFile(p.path)
	if err != nil {
		return "", fmt.Errorf("reading system prompt file %q: %w", p.path, err)
	}
	return string(data), nil
}
