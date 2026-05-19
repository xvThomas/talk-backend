package prompt

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileProvider_FileFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.md")
	if err := os.WriteFile(path, []byte("You are a helpful assistant."), 0600); err != nil {
		t.Fatal(err)
	}

	p := NewFileProvider(path)
	got, err := p.SystemPrompt(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "You are a helpful assistant." {
		t.Errorf("unexpected content: %q", got)
	}
}

func TestFileProvider_FileMissing(t *testing.T) {
	p := NewFileProvider("/nonexistent/path/prompt.md")
	_, err := p.SystemPrompt(context.Background())
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
