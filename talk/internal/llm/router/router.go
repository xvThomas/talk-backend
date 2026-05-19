package router

import (
	"fmt"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/config"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/llm/anthropic"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/llm/openai"
)

// Router builds LlmClient instances for model aliases from configuration.
type Router struct {
	cfg *config.Config
}

// NewLLMRouter creates a Router backed by the given configuration.
func NewLLMRouter(cfg *config.Config) *Router {
	return &Router{cfg: cfg}
}

// Get returns an LlmClient for the given model alias, building it from configuration.
func (r *Router) Get(model domain.Model) (domain.LlmClient, error) {
	d, err := domain.Lookup(model)
	if err != nil {
		return nil, err
	}

	switch d.Provider {
	case domain.ProviderAnthropic:
		key, err := r.cfg.RequireAnthropicKey()
		if err != nil {
			return nil, err
		}
		return anthropic.NewAnthropicClient(key, d.APIModelID), nil

	case domain.ProviderOpenAI:
		key, err := r.cfg.RequireOpenAIKey()
		if err != nil {
			return nil, err
		}
		return openai.NewOpenAIClient(key, d.APIModelID, ""), nil

	case domain.ProviderMistral:
		key, err := r.cfg.RequireMistralKey()
		if err != nil {
			return nil, err
		}
		// Mistral is OpenAI-compatible, but requires a custom base URL.
		return openai.NewOpenAIClient(key, d.APIModelID, "https://api.mistral.ai/v1"), nil

	default:
		return nil, fmt.Errorf("unsupported provider %q", d.Provider)
	}
}
