package router

import (
	"testing"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/config"
)

func TestRouter_UnknownModelReturnsError(t *testing.T) {
	r := NewLLMRouter(&config.Config{})
	_, err := r.Get("unknown-model")
	if err == nil {
		t.Error("expected error for unknown model, got nil")
	}
}

func TestRouter_MissingAPIKeyReturnsError(t *testing.T) {
	r := NewLLMRouter(&config.Config{})
	_, err := r.Get("sonnet-4.6")
	if err == nil {
		t.Error("expected error for missing ANTHROPIC_API_KEY, got nil")
	}
}

func TestRouter_ValidConfigReturnsClient(t *testing.T) {
	r := NewLLMRouter(&config.Config{AnthropicAPIKey: "test-key"})
	client, err := r.Get("sonnet-4.6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client")
	}
}

func TestRouter_OpenAIProviderReturnsClient(t *testing.T) {
	r := NewLLMRouter(&config.Config{OpenAIAPIKey: "test-key"})
	client, err := r.Get("gpt-5.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client")
	}
}

func TestRouter_MistralProviderReturnsClient(t *testing.T) {
	r := NewLLMRouter(&config.Config{MistralAPIKey: "test-key"})
	client, err := r.Get("mistral-small")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client")
	}
}
