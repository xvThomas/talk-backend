package mcp_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"talks/internal/domain"
	"talks/pkg/mcp"
	"talks/pkg/mcp/playground"
)

func buildTools() []domain.Tool {
	return []domain.Tool{domain.Adapt(playground.NewSumTool())}
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
	req := mcp.JSONRPCRequest{
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

func decodeResponse(t *testing.T, raw string) mcp.JSONRPCResponse {
	t.Helper()
	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode response %q: %v", raw, err)
	}
	return resp
}
