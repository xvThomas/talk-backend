package mcplegacy_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"talks/pkg/mcplegacy"

	"github.com/gorilla/mux"
)

// TestHTTPSSE_SumTool starts a real HTTP server via httptest and sends
// initialize, tools/list, and tools/call(sum(5,6)) against the /rpc endpoint.
func TestHTTPSSE_SumTool(t *testing.T) {
	router := mcplegacy.NewMcpRpcRouter("test-server", "0.0.1", buildTools())
	handler := mcplegacy.NewMcpServerHandler(router, 0, nil)

	muxRouter := mux.NewRouter()
	handler.RegisterRoutes(muxRouter)
	ts := httptest.NewServer(muxRouter)
	defer ts.Close()

	post := func(t *testing.T, id int, method string, params any) mcplegacy.JSONRPCResponse {
		t.Helper()
		body := marshalRequest(t, id, method, params)
		resp, err := http.Post(ts.URL+"/rpc", "application/json", strings.NewReader(body)) //nolint:noctx
		if err != nil {
			t.Fatalf("HTTP POST %s: %v", method, err)
		}
		defer resp.Body.Close()
		var rpcResp mcplegacy.JSONRPCResponse
		if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
			t.Fatalf("decode response for %s: %v", method, err)
		}
		return rpcResp
	}

	// 1. initialize
	initResp := post(t, 1, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"clientInfo":      map[string]any{"name": "test-client", "version": "0.0.1"},
	})
	if initResp.Error != nil {
		t.Fatalf("initialize error: %+v", initResp.Error)
	}
	initJSON, _ := json.Marshal(initResp.Result)
	var initResult struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
		Capabilities struct {
			Tools map[string]any `json:"tools"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(initJSON, &initResult); err != nil {
		t.Fatalf("parse initialize result: %v", err)
	}
	if initResult.ProtocolVersion != "2025-03-26" {
		t.Errorf("expected protocolVersion=2025-03-26, got %q", initResult.ProtocolVersion)
	}
	if initResult.ServerInfo.Name != "test-server" {
		t.Errorf("expected serverInfo.name=test-server, got %q", initResult.ServerInfo.Name)
	}
	if initResult.Capabilities.Tools == nil {
		t.Error("expected capabilities.tools to be present")
	}

	// 2. tools/list
	listResp := post(t, 2, "tools/list", nil)
	if listResp.Error != nil {
		t.Fatalf("tools/list error: %+v", listResp.Error)
	}
	listJSON, _ := json.Marshal(listResp.Result)
	var listResult struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(listJSON, &listResult); err != nil {
		t.Fatalf("parse tools/list result: %v", err)
	}
	if len(listResult.Tools) != 1 || listResult.Tools[0].Name != "sum" {
		t.Errorf("expected tool [sum], got %+v", listResult.Tools)
	}

	// 3. tools/call sum(5,6) -> 11
	callResp := post(t, 3, "tools/call", callRequest{
		Name:      "sum",
		Arguments: map[string]any{"a": 5, "b": 6},
	})
	if callResp.Error != nil {
		t.Fatalf("tools/call error: %+v", callResp.Error)
	}
	callJSON, _ := json.Marshal(callResp.Result)
	_ = callJSON
	payload := parseToolResult(t, callResp.Result)
	if payload["sum"] != float64(11) {
		t.Errorf("expected sum=11, got %v", payload["sum"])
	}
}
