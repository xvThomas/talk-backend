package mcplegacy_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"talks/internal/domain"
	"talks/pkg/mcplegacy"
	"talks/pkg/mcplegacy/playground"
)

func buildTools() []domain.Tool {
	return []domain.Tool{domain.Adapt(playground.NewSumTool())}
}

// buildToolsWithPrompts returns the SumTool which implements MCPPromptProvider natively.
func buildToolsWithPrompts() []domain.Tool {
	return []domain.Tool{domain.Adapt(playground.NewSumTool())}
}

// failingInputSchemaTool is a domain.Tool whose InputSchema always returns an error.
// It embeds domain.Tool but does NOT forward Prompts(), so it does NOT satisfy
// domain.MCPPromptProvider — useful to cover the !ok continue branches.
type failingInputSchemaTool struct{ domain.Tool }

func (f *failingInputSchemaTool) InputSchema() (map[string]any, error) {
	return nil, fmt.Errorf("schema error (test)")
}

// buildToolsWithSchemaError returns a list containing a tool that fails InputSchema.
func buildToolsWithSchemaError() []domain.Tool {
	return []domain.Tool{&failingInputSchemaTool{Tool: domain.Adapt(playground.NewSumTool())}}
}

// failingOutputSchemaTool succeeds InputSchema but fails OutputSchema.
// Also does NOT satisfy MCPPromptProvider.
type failingOutputSchemaTool struct{ domain.Tool }

func (f *failingOutputSchemaTool) OutputSchema() (map[string]any, error) {
	return nil, fmt.Errorf("output schema error (test)")
}

// buildToolsWithOutputSchemaError returns a tool whose OutputSchema fails.
func buildToolsWithOutputSchemaError() []domain.Tool {
	return []domain.Tool{&failingOutputSchemaTool{Tool: domain.Adapt(playground.NewSumTool())}}
}

// errorTool is a domain.Tool whose Execute always returns an error.
// Does NOT satisfy MCPPromptProvider.
type errorTool struct{ domain.Tool }

func (e *errorTool) Execute(_ context.Context, _ map[string]any) (map[string]any, error) {
	return nil, fmt.Errorf("execute error (test)")
}

// buildToolsWithExecuteError returns a list where the sum tool's Execute always fails.
func buildToolsWithExecuteError() []domain.Tool {
	return []domain.Tool{&errorTool{Tool: domain.Adapt(playground.NewSumTool())}}
}

// marshalErrorTool returns an un-marshalable value from Execute (channel).
// Does NOT satisfy MCPPromptProvider.
type marshalErrorTool struct{ domain.Tool }

func (m *marshalErrorTool) Execute(_ context.Context, _ map[string]any) (map[string]any, error) {
	return map[string]any{"ch": make(chan int)}, nil // json.Marshal can't handle chan
}

// buildToolsWithMarshalError returns a tool whose output cannot be JSON-marshaled.
func buildToolsWithMarshalError() []domain.Tool {
	return []domain.Tool{&marshalErrorTool{Tool: domain.Adapt(playground.NewSumTool())}}
}

// parseRequest unmarshals a JSON string into a JSONRPCRequest.
func parseRequest(t *testing.T, raw string) mcplegacy.JSONRPCRequest {
	t.Helper()
	var req mcplegacy.JSONRPCRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("parseRequest: %v", err)
	}
	return req
}

type callRequest struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func marshalRequest(t *testing.T, id int, method string, params any) string {
	t.Helper()
	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
		rawParams = b
	}
	req := mcplegacy.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(fmt.Sprintf("%d", id)),
		Method:  method,
		Params:  rawParams,
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return string(b)
}

func decodeResponse(t *testing.T, raw string) mcplegacy.JSONRPCResponse {
	t.Helper()
	var resp mcplegacy.JSONRPCResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode response %q: %v", raw, err)
	}
	return resp
}

// parseToolResult extracts the structured payload from a tools/call result.
// It prefers structuredContent (2025-03-26+) and falls back to JSON-parsing
// content[0].text (2024-11-05 clients).
func parseToolResult(t *testing.T, rpcResult any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(rpcResult)
	var outer struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent map[string]any `json:"structuredContent"`
	}
	if err := json.Unmarshal(b, &outer); err != nil {
		t.Fatalf("unexpected tools/call result format: %s", b)
	}
	if outer.StructuredContent != nil {
		return outer.StructuredContent
	}
	if len(outer.Content) == 0 {
		t.Fatalf("unexpected tools/call result format: %s", b)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(outer.Content[0].Text), &payload); err != nil {
		t.Fatalf("parse tool output text: %v", err)
	}
	return payload
}
