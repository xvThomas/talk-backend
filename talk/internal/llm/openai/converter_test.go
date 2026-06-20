package openai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/openai/openai-go"

	"github.com/xvThomas/talk-backend/talk/internal/domain"
)

func TestToSDKMessages_WithSystemPrompt(t *testing.T) {
	msgs := []domain.Message{
		{Role: domain.RoleUser, Content: "hello"},
	}
	result := toSDKMessages("You are helpful.", msgs)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages (system+user), got %d", len(result))
	}
}

func TestToSDKMessages_NoSystemPrompt(t *testing.T) {
	msgs := []domain.Message{
		{Role: domain.RoleUser, Content: "hello"},
		{Role: domain.RoleAssistant, Content: "hi"},
	}
	result := toSDKMessages("", msgs)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
}

func TestToSDKMessages_ToolCalls(t *testing.T) {
	msgs := []domain.Message{
		{Role: domain.RoleUser, Content: "question"},
		{
			Role:    domain.RoleAssistant,
			Content: "",
			ToolCalls: []domain.ToolCall{
				{ID: "call-1", Name: "calc", Input: map[string]any{"expr": "1+1"}},
			},
		},
		{
			Role:        domain.RoleTool,
			ToolResults: []domain.ToolResult{{ToolCallID: "call-1", Content: "2"}},
		},
	}
	result := toSDKMessages("", msgs)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	// Assistant with tool calls.
	if result[1].OfAssistant == nil {
		t.Fatal("expected assistant message with OfAssistant set")
	}
	if len(result[1].OfAssistant.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result[1].OfAssistant.ToolCalls))
	}
	if result[1].OfAssistant.ToolCalls[0].Function.Name != "calc" {
		t.Errorf("expected tool name %q, got %q", "calc", result[1].OfAssistant.ToolCalls[0].Function.Name)
	}
	// Tool result.
	if result[2].OfTool == nil {
		t.Fatal("expected tool message with OfTool set")
	}
}

func TestToSDKTools(t *testing.T) {
	tools := []domain.Tool{&fakeTool{
		name:        "weather",
		description: "Get weather",
		inputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"city": map[string]any{"type": "string"},
			},
		},
	}}

	result, err := toSDKTools(tools)
	if err != nil {
		t.Fatalf("toSDKTools error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	if result[0].Function.Name != "weather" {
		t.Errorf("expected name %q, got %q", "weather", result[0].Function.Name)
	}
}

func TestFromSDKResponse_TextOnly(t *testing.T) {
	resp := &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{
			{Message: openai.ChatCompletionMessage{Content: "The answer is 42."}},
		},
		Usage: openai.CompletionUsage{
			PromptTokens:     100,
			CompletionTokens: 20,
		},
	}
	msg, usage := fromSDKResponse(resp)
	if msg.Role != domain.RoleAssistant {
		t.Errorf("expected assistant role, got %s", msg.Role)
	}
	if msg.Content != "The answer is 42." {
		t.Errorf("unexpected content: %q", msg.Content)
	}
	if usage.InputTokens != 100 || usage.OutputTokens != 20 {
		t.Errorf("unexpected usage: %+v", usage)
	}
}

func TestFromSDKResponse_EmptyChoices(t *testing.T) {
	resp := &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{},
		Usage:   openai.CompletionUsage{PromptTokens: 50},
	}
	msg, usage := fromSDKResponse(resp)
	if msg.Role != domain.RoleAssistant {
		t.Errorf("expected assistant role, got %s", msg.Role)
	}
	if msg.Content != "" {
		t.Errorf("expected empty content, got %q", msg.Content)
	}
	if usage.InputTokens != 50 {
		t.Errorf("expected 50 input tokens, got %d", usage.InputTokens)
	}
}

func TestFromSDKResponse_ToolCalls(t *testing.T) {
	resp := &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Content: "",
					ToolCalls: []openai.ChatCompletionMessageToolCall{
						{
							ID: "call-xyz",
							Function: openai.ChatCompletionMessageToolCallFunction{
								Name:      "get_weather",
								Arguments: `{"city":"Tokyo"}`,
							},
						},
					},
				},
			},
		},
		Usage: openai.CompletionUsage{PromptTokens: 30, CompletionTokens: 10},
	}
	msg, _ := fromSDKResponse(resp)
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ID != "call-xyz" {
		t.Errorf("expected ID %q, got %q", "call-xyz", msg.ToolCalls[0].ID)
	}
	if msg.ToolCalls[0].Name != "get_weather" {
		t.Errorf("expected name %q, got %q", "get_weather", msg.ToolCalls[0].Name)
	}
	if msg.ToolCalls[0].Input["city"] != "Tokyo" {
		t.Errorf("expected city Tokyo, got %v", msg.ToolCalls[0].Input["city"])
	}
}

func TestFromSDKResponse_CacheTokens(t *testing.T) {
	resp := &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{
			{Message: openai.ChatCompletionMessage{Content: "cached"}},
		},
		Usage: openai.CompletionUsage{
			PromptTokens:     200,
			CompletionTokens: 50,
			PromptTokensDetails: openai.CompletionUsagePromptTokensDetails{
				CachedTokens: 150,
			},
		},
	}
	_, usage := fromSDKResponse(resp)
	if usage.CacheReadTokens != 150 {
		t.Errorf("expected 150 cache read tokens, got %d", usage.CacheReadTokens)
	}
}

func TestToAssistantParam_WithToolCalls(t *testing.T) {
	msg := domain.Message{
		Role:    domain.RoleAssistant,
		Content: "I'll call a tool",
		ToolCalls: []domain.ToolCall{
			{ID: "c1", Name: "search", Input: map[string]any{"q": "test"}},
		},
	}
	param := toAssistantParam(msg)
	if param.OfAssistant == nil {
		t.Fatal("expected OfAssistant to be set")
	}
	if len(param.OfAssistant.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(param.OfAssistant.ToolCalls))
	}

	// Verify the arguments are valid JSON.
	var input map[string]any
	if err := json.Unmarshal([]byte(param.OfAssistant.ToolCalls[0].Function.Arguments), &input); err != nil {
		t.Fatalf("invalid arguments JSON: %v", err)
	}
	if input["q"] != "test" {
		t.Errorf("expected q=test, got %v", input["q"])
	}
}

func TestToToolResultParams(t *testing.T) {
	msg := domain.Message{
		Role: domain.RoleTool,
		ToolResults: []domain.ToolResult{
			{ToolCallID: "call-1", Content: "result 1"},
			{ToolCallID: "call-2", Content: "result 2"},
		},
	}
	params := toToolResultParams(msg)
	if len(params) != 2 {
		t.Fatalf("expected 2 tool result params, got %d", len(params))
	}
}

// fakeTool implements domain.Tool for testing.
type fakeTool struct {
	name        string
	description string
	inputSchema map[string]any
}

func (f *fakeTool) Name() string                         { return f.name }
func (f *fakeTool) Description() string                  { return f.description }
func (f *fakeTool) InputSchema() (map[string]any, error) { return f.inputSchema, nil }
func (f *fakeTool) OutputSchema() (map[string]any, error) {
	return map[string]any{"type": "object"}, nil
}
func (f *fakeTool) Execute(_ context.Context, _ map[string]any) (map[string]any, error) {
	return nil, nil
}
