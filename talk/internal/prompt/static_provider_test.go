package prompt

import (
	"context"
	"testing"
)

func TestStaticProvider_ReturnsText(t *testing.T) {
	p := NewStaticProvider("You are a concise assistant.")
	got, err := p.SystemPrompt(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "You are a concise assistant." {
		t.Errorf("unexpected text: %q", got)
	}
}

func TestStaticProvider_EmptyString(t *testing.T) {
	p := NewStaticProvider("")
	got, err := p.SystemPrompt(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}
