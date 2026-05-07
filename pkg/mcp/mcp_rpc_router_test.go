package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"talks/pkg/mcp"
)

// routeReq is a convenience helper: builds a JSONRPCRequest from (id, method, params)
// and routes it through the given router.
func routeReq(t *testing.T, router mcp.McpRpcRouter, id int, method string, params any) mcp.JSONRPCResponse {
	t.Helper()
	return router.RouteRequest(context.Background(), parseRequest(t, marshalRequest(t, id, method, params)))
}

// ── initialize ────────────────────────────────────────────────────────────────

// TestRouter_Initialize_CapabilitiesPrompts verifies that initialize returns a
// capabilities.prompts field so MCP clients know the server exposes prompts.
func TestRouter_Initialize_CapabilitiesPrompts(t *testing.T) {
	router := mcp.NewMcpRpcRouter("s", "1", buildToolsWithPrompts())
	resp := routeReq(t, router, 1, "initialize", map[string]any{"protocolVersion": "2025-03-26"})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	b, _ := json.Marshal(resp.Result)
	var result struct {
		Capabilities struct {
			Prompts map[string]any `json:"prompts"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.Capabilities.Prompts == nil {
		t.Error("expected capabilities.prompts to be present")
	}
}

// ── prompts/list ──────────────────────────────────────────────────────────────

// TestRouter_PromptsList_Empty verifies that prompts/list returns an empty list
// when no tool implements MCPPromptProvider.
func TestRouter_PromptsList_Empty(t *testing.T) {
	// failingInputSchemaTool embeds domain.Tool but has no Prompts() method.
	router := mcp.NewMcpRpcRouter("s", "1", buildToolsWithSchemaError())
	resp := routeReq(t, router, 1, "prompts/list", nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	b, _ := json.Marshal(resp.Result)
	var result struct {
		Prompts []any `json:"prompts"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Prompts) != 0 {
		t.Errorf("expected 0 prompts, got %d", len(result.Prompts))
	}
}

// TestRouter_PromptsList_WithPrompts verifies that a tool implementing
// MCPPromptProvider has its prompts reflected in the prompts/list response.
func TestRouter_PromptsList_WithPrompts(t *testing.T) {
	router := mcp.NewMcpRpcRouter("s", "1", buildToolsWithPrompts())
	resp := routeReq(t, router, 1, "prompts/list", nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	b, _ := json.Marshal(resp.Result)
	var result struct {
		Prompts []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"prompts"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(result.Prompts))
	}
	if result.Prompts[0].Name != "sum_example" {
		t.Errorf("expected name %q, got %q", "sum_example", result.Prompts[0].Name)
	}
	if result.Prompts[0].Description == "" {
		t.Error("expected non-empty description")
	}
}

// ── prompts/get ───────────────────────────────────────────────────────────────

// TestRouter_PromptsGet_Valid verifies that a known prompt name returns the
// correct description and message list with type="text" content items.
func TestRouter_PromptsGet_Valid(t *testing.T) {
	router := mcp.NewMcpRpcRouter("s", "1", buildToolsWithPrompts())
	resp := routeReq(t, router, 1, "prompts/get", map[string]any{"name": "sum_example"})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	b, _ := json.Marshal(resp.Result)
	var result struct {
		Description string `json:"description"`
		Messages    []struct {
			Role    string `json:"role"`
			Content struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.Description == "" {
		t.Error("expected non-empty description")
	}
	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}
	if result.Messages[0].Role != "user" {
		t.Errorf("expected messages[0].role=%q, got %q", "user", result.Messages[0].Role)
	}
	if result.Messages[0].Content.Type != "text" {
		t.Errorf("expected messages[0].content.type=%q, got %q", "text", result.Messages[0].Content.Type)
	}
	if result.Messages[0].Content.Text == "" {
		t.Error("expected non-empty message text")
	}
	if result.Messages[1].Role != "assistant" {
		t.Errorf("expected messages[1].role=%q, got %q", "assistant", result.Messages[1].Role)
	}
}

// TestRouter_PromptsGet_MissingName verifies that an empty name param returns
// error code -32602.
func TestRouter_PromptsGet_MissingName(t *testing.T) {
	router := mcp.NewMcpRpcRouter("s", "1", buildToolsWithPrompts())
	resp := routeReq(t, router, 1, "prompts/get", map[string]any{})
	if resp.Error == nil {
		t.Fatal("expected error for missing prompt name")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected code -32602, got %d", resp.Error.Code)
	}
}

// TestRouter_PromptsGet_NilParams verifies that nil params returns -32602.
func TestRouter_PromptsGet_NilParams(t *testing.T) {
	router := mcp.NewMcpRpcRouter("s", "1", buildToolsWithPrompts())
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "prompts/get"}
	resp := router.RouteRequest(context.Background(), req)
	if resp.Error == nil {
		t.Fatal("expected error for nil params")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected code -32602, got %d", resp.Error.Code)
	}
}

// TestRouter_PromptsGet_UnknownName verifies that an unknown prompt name
// returns error code -32602.
func TestRouter_PromptsGet_UnknownName(t *testing.T) {
	router := mcp.NewMcpRpcRouter("s", "1", buildToolsWithPrompts())
	resp := routeReq(t, router, 1, "prompts/get", map[string]any{"name": "no_such_prompt"})
	if resp.Error == nil {
		t.Fatal("expected error for unknown prompt name")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected code -32602, got %d", resp.Error.Code)
	}
}

// ── tools/call errors ─────────────────────────────────────────────────────────

// TestRouter_ToolsCall_UnknownTool verifies that calling an unregistered tool
// returns error code -32601.
func TestRouter_ToolsCall_UnknownTool(t *testing.T) {
	router := mcp.NewMcpRpcRouter("s", "1", buildTools())
	resp := routeReq(t, router, 1, "tools/call", callRequest{
		Name:      "no_such_tool",
		Arguments: map[string]any{},
	})
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected code -32601, got %d", resp.Error.Code)
	}
}

// TestRouter_ToolsCall_InvalidParams verifies that malformed params (array
// instead of object) return error code -32602.
func TestRouter_ToolsCall_InvalidParams(t *testing.T) {
	router := mcp.NewMcpRpcRouter("s", "1", buildTools())
	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Method:  "tools/call",
		Params:  json.RawMessage("[1,2,3]"),
	}
	resp := router.RouteRequest(context.Background(), req)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected code -32602, got %d", resp.Error.Code)
	}
}

// ── method not found ──────────────────────────────────────────────────────────

// TestRouter_MethodNotFound verifies that an unrecognised method returns -32601.
func TestRouter_MethodNotFound(t *testing.T) {
	router := mcp.NewMcpRpcRouter("s", "1", buildTools())
	resp := routeReq(t, router, 1, "unknown/method", nil)
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected code -32601, got %d", resp.Error.Code)
	}
}

// ── notifications ─────────────────────────────────────────────────────────────

// TestRouter_NotificationCancelled_NoError verifies that the router handles
// notifications/cancelled without producing a JSON-RPC error.
func TestRouter_NotificationCancelled_NoError(t *testing.T) {
	router := mcp.NewMcpRpcRouter("s", "1", buildTools())
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", Method: "notifications/cancelled"}
	resp := router.RouteRequest(context.Background(), req)
	if resp.Error != nil {
		t.Errorf("unexpected error for notification: %+v", resp.Error)
	}
}

// ── tools/list schema errors ──────────────────────────────────────────────────

// TestRouter_ToolsList_InputSchemaError verifies that a tool whose InputSchema
// returns an error causes tools/list to return a -32603 JSON-RPC error.
func TestRouter_ToolsList_InputSchemaError(t *testing.T) {
	router := mcp.NewMcpRpcRouter("s", "1", buildToolsWithSchemaError())
	resp := routeReq(t, router, 1, "tools/list", nil)
	if resp.Error == nil {
		t.Fatal("expected error when tool InputSchema fails")
	}
	if resp.Error.Code != -32603 {
		t.Errorf("expected code -32603, got %d", resp.Error.Code)
	}
}

// TestRouter_ToolsList_OutputSchemaError verifies that a tool whose OutputSchema
// returns an error causes tools/list to return a -32603 JSON-RPC error.
func TestRouter_ToolsList_OutputSchemaError(t *testing.T) {
	router := mcp.NewMcpRpcRouter("s", "1", buildToolsWithOutputSchemaError())
	resp := routeReq(t, router, 1, "tools/list", nil)
	if resp.Error == nil {
		t.Fatal("expected error when tool OutputSchema fails")
	}
	if resp.Error.Code != -32603 {
		t.Errorf("expected code -32603, got %d", resp.Error.Code)
	}
}

// ── tools/call errors ─────────────────────────────────────────────────────────

// TestRouter_ToolsCall_ExecuteError verifies that a tool whose Execute returns
// an error causes tools/call to return a -32603 JSON-RPC error.
func TestRouter_ToolsCall_ExecuteError(t *testing.T) {
	router := mcp.NewMcpRpcRouter("s", "1", buildToolsWithExecuteError())
	resp := routeReq(t, router, 1, "tools/call", callRequest{
		Name:      "sum",
		Arguments: map[string]any{"a": 1, "b": 2},
	})
	if resp.Error == nil {
		t.Fatal("expected error when tool Execute fails")
	}
	if resp.Error.Code != -32603 {
		t.Errorf("expected code -32603, got %d", resp.Error.Code)
	}
}

// TestRouter_ToolsCall_MarshalError verifies that a tool whose Execute returns
// an un-marshalable value causes tools/call to return a -32603 JSON-RPC error.
func TestRouter_ToolsCall_MarshalError(t *testing.T) {
	router := mcp.NewMcpRpcRouter("s", "1", buildToolsWithMarshalError())
	resp := routeReq(t, router, 1, "tools/call", callRequest{
		Name:      "sum",
		Arguments: map[string]any{"a": 1, "b": 2},
	})
	if resp.Error == nil {
		t.Fatal("expected error when tool output cannot be marshaled")
	}
	if resp.Error.Code != -32603 {
		t.Errorf("expected code -32603, got %d", resp.Error.Code)
	}
}

// ── prompts with non-MCPPromptProvider tools ──────────────────────────────────

// TestRouter_PromptsList_NonProvider verifies that a tool which does NOT implement
// MCPPromptProvider (e.g. inline mock without Prompts()) triggers the !ok branch
// in promptsList and is simply skipped.
func TestRouter_PromptsList_NonProvider(t *testing.T) {
	// failingInputSchemaTool embeds domain.Tool but has no Prompts() → !ok=true → continue.
	router := mcp.NewMcpRpcRouter("s", "1", buildToolsWithSchemaError())
	resp := routeReq(t, router, 1, "prompts/list", nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	b, _ := json.Marshal(resp.Result)
	var result struct {
		Prompts []any `json:"prompts"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Prompts) != 0 {
		t.Errorf("expected 0 prompts for non-provider tool, got %d", len(result.Prompts))
	}
}

// TestRouter_PromptsGet_NonProvider verifies that prompts/get with a tool that
// does NOT implement MCPPromptProvider returns an unknown-prompt error.
func TestRouter_PromptsGet_NonProvider(t *testing.T) {
	router := mcp.NewMcpRpcRouter("s", "1", buildToolsWithSchemaError())
	resp := routeReq(t, router, 1, "prompts/get", map[string]any{"name": "sum"})
	if resp.Error == nil {
		t.Fatal("expected error: no prompt provider in tool list")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected code -32602, got %d", resp.Error.Code)
	}
}
