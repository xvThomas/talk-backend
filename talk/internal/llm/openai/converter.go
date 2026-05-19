package openai

import (
	"encoding/json"
	"fmt"
	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"

	"github.com/openai/openai-go"
)

// toSDKMessages converts domain messages to OpenAI SDK message params.
func toSDKMessages(systemPrompt string, messages []domain.Message) []openai.ChatCompletionMessageParamUnion {
	params := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages)+1)

	if systemPrompt != "" {
		params = append(params, openai.SystemMessage(systemPrompt))
	}

	for _, msg := range messages {
		switch msg.Role {
		case domain.RoleUser:
			params = append(params, openai.UserMessage(msg.Content))
		case domain.RoleAssistant:
			params = append(params, toAssistantParam(msg))
		case domain.RoleTool:
			params = append(params, toToolResultParams(msg)...)
		}
	}
	return params
}

func toAssistantParam(msg domain.Message) openai.ChatCompletionMessageParamUnion {
	if len(msg.ToolCalls) == 0 {
		return openai.AssistantMessage(msg.Content)
	}
	calls := make([]openai.ChatCompletionMessageToolCallParam, 0, len(msg.ToolCalls))
	for _, tc := range msg.ToolCalls {
		raw, _ := json.Marshal(tc.Input)
		// Type field is constant.Function, zero value marshals as "function" automatically.
		calls = append(calls, openai.ChatCompletionMessageToolCallParam{
			ID: tc.ID,
			Function: openai.ChatCompletionMessageToolCallFunctionParam{
				Name:      tc.Name,
				Arguments: string(raw),
			},
		})
	}
	return openai.ChatCompletionMessageParamUnion{
		OfAssistant: &openai.ChatCompletionAssistantMessageParam{
			Content:   openai.ChatCompletionAssistantMessageParamContentUnion{OfString: openai.String(msg.Content)},
			ToolCalls: calls,
		},
	}
}

func toToolResultParams(msg domain.Message) []openai.ChatCompletionMessageParamUnion {
	results := make([]openai.ChatCompletionMessageParamUnion, 0, len(msg.ToolResults))
	for _, tr := range msg.ToolResults {
		results = append(results, openai.ToolMessage(tr.Content, tr.ToolCallID))
	}
	return results
}

// toSDKTools converts domain tools to OpenAI SDK tool definitions.
func toSDKTools(tools []domain.Tool) ([]openai.ChatCompletionToolParam, error) {
	sdkTools := make([]openai.ChatCompletionToolParam, 0, len(tools))
	for _, t := range tools {
		inputSchema, err := t.InputSchema()
		if err != nil {
			return nil, fmt.Errorf("Unable to get InputSchema for tool %s: %w", t.Name(), err)
		}
		// Type field is constant.Function, zero value marshals as "function" automatically.
		sdkTools = append(sdkTools, openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        t.Name(),
				Description: openai.String(t.Description()),
				Parameters:  openai.FunctionParameters(inputSchema),
			},
		})
	}
	return sdkTools, nil
}

// fromSDKResponse converts an OpenAI SDK response to a domain Message and Usage.
func fromSDKResponse(resp *openai.ChatCompletion) (*domain.Message, domain.Usage) {
	usage := domain.Usage{
		InputTokens:     resp.Usage.PromptTokens,
		OutputTokens:    resp.Usage.CompletionTokens,
		CacheReadTokens: resp.Usage.PromptTokensDetails.CachedTokens,
	}
	if len(resp.Choices) == 0 {
		return &domain.Message{Role: domain.RoleAssistant}, usage
	}
	choice := resp.Choices[0].Message
	msg := &domain.Message{
		Role:    domain.RoleAssistant,
		Content: choice.Content,
	}
	for _, tc := range choice.ToolCalls {
		var input map[string]any
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
		msg.ToolCalls = append(msg.ToolCalls, domain.ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}
	return msg, usage
}
