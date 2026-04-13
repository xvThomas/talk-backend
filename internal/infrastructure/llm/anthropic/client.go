package anthropic

import (
	"context"
	"fmt"
	"talks/internal/domain"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicClient implements domain.LlmClient using the Anthropic API.
type AnthropicClient struct {
	sdk     *anthropic.Client
	modelID string
}

var _ domain.LlmClient = (*AnthropicClient)(nil) // ensure AnthropicClient implements domain.LlmClient

// NewAnthropicClient creates an Anthropic Client.
func NewAnthropicClient(apiKey, modelID string) *AnthropicClient {
	sdk := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &AnthropicClient{sdk: &sdk, modelID: modelID}
}

// Complete sends the conversation to Anthropic and returns the assistant response with token usage.
func (c *AnthropicClient) Complete(ctx context.Context, systemPrompt string, messages []domain.Message, tools []domain.Tool) (*domain.Message, domain.Usage, error) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.modelID),
		MaxTokens: 4096,
		Messages:  toSDKMessages(messages),
	}

	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{toSystemPrompt(systemPrompt)}
	}

	if len(tools) > 0 {
		var err error
		params.Tools, err = toSDKTools(tools)
		if err != nil {
			return nil, domain.Usage{}, fmt.Errorf("anthropic completion: %w", err)
		}
	}

	resp, err := c.sdk.Messages.New(ctx, params)
	if err != nil {
		return nil, domain.Usage{}, fmt.Errorf("anthropic completion: %w", err)
	}

	msg, usage := fromSDKResponse(resp)
	return msg, usage, nil
}
