package anthropic

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/xvThomas/talk-backend/talk/internal/domain"
)

func TestToSDKMessages_Simple(t *testing.T) {
	msgs := []domain.Message{
		{Role: domain.RoleUser, Content: "hello"},
		{Role: domain.RoleAssistant, Content: "hi there"},
	}
	result := toSDKMessages(msgs)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("expected role user, got %s", result[0].Role)
	}
	if result[1].Role != "assistant" {
		t.Errorf("expected role assistant, got %s", result[1].Role)
	}
}

func TestToSDKMessages_SkipsEmptyContent(t *testing.T) {
	msgs := []domain.Message{
		{Role: domain.RoleUser, Content: ""},
		{Role: domain.RoleUser, Content: "real question"},
	}
	result := toSDKMessages(msgs)
	if len(result) != 1 {
		t.Fatalf("expected 1 message (empty skipped), got %d", len(result))
	}
}

func TestToSDKMessages_MergesConsecutiveSameRole(t *testing.T) {
	msgs := []domain.Message{
		{Role: domain.RoleUser, Content: "part 1"},
		{Role: domain.RoleUser, Content: "part 2"},
		{Role: domain.RoleAssistant, Content: "response"},
	}
	result := toSDKMessages(msgs)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages (merged user), got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("expected merged user role, got %s", result[0].Role)
	}
	if len(result[0].Content) != 2 {
		t.Errorf("expected 2 content blocks in merged user message, got %d", len(result[0].Content))
	}
}

func TestToSDKMessages_ToolMessages(t *testing.T) {
	msgs := []domain.Message{
		{Role: domain.RoleUser, Content: "question"},
		{
			Role: domain.RoleAssistant,
			ToolCalls: []domain.ToolCall{
				{ID: "call-1", Name: "get_weather", Input: map[string]any{"city": "Paris"}},
			},
		},
		{
			Role: domain.RoleTool,
			ToolResults: []domain.ToolResult{
				{ToolCallID: "call-1", Content: `{"temp":"20C"}`},
			},
		},
	}
	result := toSDKMessages(msgs)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	// Tool result is sent as a user message with tool_result blocks.
	if result[2].Role != "user" {
		t.Errorf("expected tool result as user role, got %s", result[2].Role)
	}
}

func TestToSDKMessages_SkipsToolWithoutResults(t *testing.T) {
	msgs := []domain.Message{
		{Role: domain.RoleUser, Content: "hi"},
		{Role: domain.RoleTool, ToolResults: nil},
	}
	result := toSDKMessages(msgs)
	if len(result) != 1 {
		t.Fatalf("expected 1 message (tool skipped), got %d", len(result))
	}
}

func TestToSDKTools(t *testing.T) {
	tools := []domain.Tool{&fakeTool{
		name:        "calculator",
		description: "Does math",
		inputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"expression": map[string]any{"type": "string"},
			},
			"required": []string{"expression"},
		},
	}}

	result, err := toSDKTools(tools)
	if err != nil {
		t.Fatalf("toSDKTools error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	if result[0].OfTool.Name != "calculator" {
		t.Errorf("expected name %q, got %q", "calculator", result[0].OfTool.Name)
	}
	if result[0].OfTool.CacheControl.Type == "" {
		t.Error("expected cache control on last tool")
	}
}

func TestFromSDKResponse_TextOnly(t *testing.T) {
	resp := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{Type: "text", Text: "The answer is 42."},
		},
		Usage: anthropic.Usage{
			InputTokens:  100,
			OutputTokens: 20,
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

func TestFromSDKResponse_ToolUse(t *testing.T) {
	inputJSON := json.RawMessage(`{"city":"London"}`)
	resp := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{Type: "tool_use", ID: "call-abc", Name: "get_weather", Input: inputJSON},
		},
		Usage: anthropic.Usage{
			InputTokens:              50,
			OutputTokens:             10,
			CacheReadInputTokens:     5,
			CacheCreationInputTokens: 3,
		},
	}
	msg, usage := fromSDKResponse(resp)
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ID != "call-abc" {
		t.Errorf("expected ID %q, got %q", "call-abc", msg.ToolCalls[0].ID)
	}
	if msg.ToolCalls[0].Name != "get_weather" {
		t.Errorf("expected name %q, got %q", "get_weather", msg.ToolCalls[0].Name)
	}
	if msg.ToolCalls[0].Input["city"] != "London" {
		t.Errorf("expected city London, got %v", msg.ToolCalls[0].Input["city"])
	}
	if usage.CacheReadTokens != 5 || usage.CacheWriteTokens != 3 {
		t.Errorf("unexpected cache usage: %+v", usage)
	}
}

func TestToSystemPrompt(t *testing.T) {
	p := toSystemPrompt("You are helpful.")
	if p.Text != "You are helpful." {
		t.Errorf("expected text %q, got %q", "You are helpful.", p.Text)
	}
	if p.CacheControl.Type == "" {
		t.Error("expected cache control on system prompt")
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
