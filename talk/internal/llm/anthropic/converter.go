package anthropic

import (
	"encoding/json"
	"fmt"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"

	"github.com/anthropics/anthropic-sdk-go"
)

// toSDKMessages converts domain messages to Anthropic SDK message params.
// It skips messages that would produce empty content (e.g. tool messages
// reloaded from DB without their ToolResults/ToolCalls) and merges
// consecutive same-role messages to maintain valid alternation.
func toSDKMessages(messages []domain.Message) []anthropic.MessageParam {
	params := make([]anthropic.MessageParam, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case domain.RoleUser:
			if msg.Content == "" {
				continue
			}
			params = append(params, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		case domain.RoleAssistant:
			p := toAssistantParam(msg)
			if len(p.Content) == 0 {
				continue
			}
			params = append(params, p)
		case domain.RoleTool:
			if len(msg.ToolResults) == 0 {
				continue
			}
			params = append(params, toToolResultParam(msg))
		}
	}
	// Anthropic requires strict user/assistant alternation.
	// Merge consecutive same-role messages by keeping only the last one.
	merged := make([]anthropic.MessageParam, 0, len(params))
	for _, p := range params {
		if len(merged) > 0 && merged[len(merged)-1].Role == p.Role {
			merged[len(merged)-1] = p
		} else {
			merged = append(merged, p)
		}
	}
	return merged
}

func toAssistantParam(msg domain.Message) anthropic.MessageParam {
	blocks := make([]anthropic.ContentBlockParamUnion, 0)
	if msg.Content != "" {
		blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
	}
	for _, tc := range msg.ToolCalls {
		// NewToolUseBlock signature: (id, input, name)
		blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, tc.Input, tc.Name))
	}
	return anthropic.NewAssistantMessage(blocks...)
}

func toToolResultParam(msg domain.Message) anthropic.MessageParam {
	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(msg.ToolResults))
	for _, tr := range msg.ToolResults {
		blocks = append(blocks, anthropic.NewToolResultBlock(tr.ToolCallID, tr.Content, false))
	}
	return anthropic.NewUserMessage(blocks...)
}

// toSDKTools converts domain tools to Anthropic SDK tool definitions.
// The last tool is marked with cache_control so that the prompt cache covers
// the system prompt + all tool definitions (Anthropic caches everything up to
// the last cache breakpoint).
func toSDKTools(tools []domain.Tool) ([]anthropic.ToolUnionParam, error) {
	sdkTools := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		// params := t.Parameters()
		params, err := t.InputSchema()
		if err != nil {
			return nil, fmt.Errorf("unable to get InputSchema for tool %s: %w", t.Name(), err)
		}
		props := params["properties"]
		var required []string
		if r, ok := params["required"]; ok {
			if sl, ok := r.([]string); ok {
				required = sl
			}
		}
		sdkTools = append(sdkTools, anthropic.ToolUnionParamOfTool(
			anthropic.ToolInputSchemaParam{
				Properties: props,
				Required:   required,
			},
			t.Name(),
		))
		sdkTools[len(sdkTools)-1].OfTool.Description = anthropic.String(t.Description())
	}
	// Place a cache breakpoint on the last tool so the entire prefix
	// (system prompt + tools) is eligible for prompt caching.
	if len(sdkTools) > 0 {
		sdkTools[len(sdkTools)-1].OfTool.CacheControl = anthropic.NewCacheControlEphemeralParam()
	}
	return sdkTools, nil
}

// fromSDKResponse converts an Anthropic SDK response to a domain Message and Usage.
func fromSDKResponse(resp *anthropic.Message) (*domain.Message, domain.Usage) {
	msg := &domain.Message{Role: domain.RoleAssistant}
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			msg.Content += block.Text
		case "tool_use":
			var input map[string]any
			_ = json.Unmarshal(block.Input, &input)
			msg.ToolCalls = append(msg.ToolCalls, domain.ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: input,
			})
		}
	}
	usage := domain.Usage{
		InputTokens:      resp.Usage.InputTokens,
		OutputTokens:     resp.Usage.OutputTokens,
		CacheReadTokens:  resp.Usage.CacheReadInputTokens,
		CacheWriteTokens: resp.Usage.CacheCreationInputTokens,
	}
	return msg, usage
}

// toSystemPrompt wraps the system prompt string with ephemeral cache control.
func toSystemPrompt(text string) anthropic.TextBlockParam {
	return anthropic.TextBlockParam{
		Text:         text,
		CacheControl: anthropic.NewCacheControlEphemeralParam(),
	}
}
