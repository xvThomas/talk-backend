package openai

import (
	"context"
	"fmt"
	"talks/internal/domain"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// OpenAIClient implements domain.LlmClient using an OpenAI-compatible API.
// It works with OpenAI, Mistral, and any other provider that exposes the
// OpenAI chat completions API.
type OpenAIClient struct {
	sdk     *openai.Client
	modelID string
}

// NewOpenAIClient creates an OpenAI-compatible Client.
// Pass a custom baseURL to target Mistral / Devstral / local endpoints.
func NewOpenAIClient(apiKey, modelID, baseURL string) *OpenAIClient {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	sdk := openai.NewClient(opts...)
	return &OpenAIClient{sdk: &sdk, modelID: modelID}
}

var _ domain.LlmClient = (*OpenAIClient)(nil) // ensure Client implements domain.LlmClient

// Complete sends the conversation to the OpenAI-compatible API and returns the response with token usage.
func (c *OpenAIClient) Complete(ctx context.Context, systemPrompt string, messages []domain.Message, tools []domain.Tool) (*domain.Message, domain.Usage, error) {
	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(c.modelID),
		Messages: toSDKMessages(systemPrompt, messages),
	}

	if len(tools) > 0 {
		var err error
		params.Tools, err = toSDKTools(tools)
		if err != nil {
			return nil, domain.Usage{}, fmt.Errorf("openai completion: %w", err)
		}
	}

	resp, err := c.sdk.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, domain.Usage{}, fmt.Errorf("openai completion: %w", err)
	}

	msg, usage := fromSDKResponse(resp)
	return msg, usage, nil
}
