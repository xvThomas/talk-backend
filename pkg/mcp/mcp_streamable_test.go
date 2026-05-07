package mcp_test

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"talks/pkg/mcp"

	"github.com/gorilla/mux"
)

// postMCP is a helper that sends a POST /mcp with the given headers and body.
func postMCP(t *testing.T, ts *httptest.Server, headers map[string]string, id int, method string, params any) *http.Response {
	t.Helper()
	body := marshalRequest(t, id, method, params)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp %s: %v", method, err)
	}
	return resp
}

func decodeJSONBody(t *testing.T, r io.Reader) mcp.JSONRPCResponse {
	t.Helper()
	var rpcResp mcp.JSONRPCResponse
	if err := json.NewDecoder(r).Decode(&rpcResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rpcResp
}

// newStreamableServer creates an httptest.Server with Streamable HTTP routes.
func newStreamableServer(t *testing.T) *httptest.Server {
	t.Helper()
	router := mcp.NewMcpRpcRouter("test-server", "0.0.1", buildTools())
	handler := mcp.NewMcpServerHandler(router, 0, nil)
	muxRouter := mux.NewRouter()
	handler.RegisterRoutes(muxRouter)
	ts := httptest.NewServer(muxRouter)
	t.Cleanup(ts.Close)
	return ts
}

// TestStreamable_Initialize verifies that POST /mcp with initialize creates a
// session (Mcp-Session-Id response header) and returns a valid result.
func TestStreamable_Initialize(t *testing.T) {
	ts := newStreamableServer(t)

	resp := postMCP(t, ts, nil, 1, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("missing Mcp-Session-Id header in initialize response")
	}

	rpcResp := decodeJSONBody(t, resp.Body)
	if rpcResp.Error != nil {
		t.Fatalf("initialize error: %+v", rpcResp.Error)
	}

	resultJSON, _ := json.Marshal(rpcResp.Result)
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if result.ProtocolVersion == "" {
		t.Error("empty protocolVersion")
	}
	if result.ServerInfo.Name != "test-server" {
		t.Errorf("serverInfo.name = %q, want %q", result.ServerInfo.Name, "test-server")
	}
}

// TestStreamable_SumTool exercises the full sequence over Streamable HTTP:
// initialize → tools/list → tools/call(3+4=7).
func TestStreamable_SumTool(t *testing.T) {
	ts := newStreamableServer(t)

	// 1. initialize (no session header needed)
	initResp := postMCP(t, ts, nil, 1, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
	})
	defer initResp.Body.Close()

	if initResp.StatusCode != http.StatusOK {
		t.Fatalf("initialize: expected 200, got %d", initResp.StatusCode)
	}
	sessionID := initResp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("missing Mcp-Session-Id")
	}
	io.Copy(io.Discard, initResp.Body) //nolint:errcheck

	sessionHeaders := map[string]string{
		"Mcp-Session-Id":       sessionID,
		"MCP-Protocol-Version": "2025-03-26",
	}

	// 2. tools/list
	listResp := postMCP(t, ts, sessionHeaders, 2, "tools/list", nil)
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("tools/list: expected 200, got %d", listResp.StatusCode)
	}
	listRPC := decodeJSONBody(t, listResp.Body)
	if listRPC.Error != nil {
		t.Fatalf("tools/list error: %+v", listRPC.Error)
	}
	listJSON, _ := json.Marshal(listRPC.Result)
	var listResult struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(listJSON, &listResult); err != nil {
		t.Fatalf("parse tools/list: %v", err)
	}
	if len(listResult.Tools) == 0 {
		t.Fatal("expected at least 1 tool")
	}

	// 3. tools/call sum(3, 4)
	callResp := postMCP(t, ts, sessionHeaders, 3, "tools/call", callRequest{
		Name:      "sum",
		Arguments: map[string]any{"a": 3, "b": 4},
	})
	defer callResp.Body.Close()

	if callResp.StatusCode != http.StatusOK {
		t.Fatalf("tools/call: expected 200, got %d", callResp.StatusCode)
	}
	callRPC := decodeJSONBody(t, callResp.Body)
	if callRPC.Error != nil {
		t.Fatalf("tools/call error: %+v", callRPC.Error)
	}
	callJSON, _ := json.Marshal(callRPC.Result)
	_ = callJSON
	payload := parseToolResult(t, callRPC.Result)
	if payload["sum"] != float64(7) {
		t.Errorf("sum(3,4) = %v, want 7", payload["sum"])
	}
}

// TestStreamable_NotificationIs202 verifies that a notification (no id)
// returns 202 with no body.
func TestStreamable_NotificationIs202(t *testing.T) {
	ts := newStreamableServer(t)

	// Initialize first to get a session.
	initResp := postMCP(t, ts, nil, 1, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
	})
	sessionID := initResp.Header.Get("Mcp-Session-Id")
	io.Copy(io.Discard, initResp.Body) //nolint:errcheck
	initResp.Body.Close()

	sessionHeaders := map[string]string{
		"Mcp-Session-Id":       sessionID,
		"MCP-Protocol-Version": "2025-03-26",
	}

	// Send a notification (id omitted from JSON).
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range sessionHeaders {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("notification request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("notification: expected 202, got %d", resp.StatusCode)
	}
}

// TestStreamable_ForbiddenOrigin verifies that browser requests from
// disallowed origins receive 403.
func TestStreamable_ForbiddenOrigin(t *testing.T) {
	router := mcp.NewMcpRpcRouter("test-server", "0.0.1", buildTools())
	handler := mcp.NewMcpServerHandler(router, 0, []string{"http://allowed.example.com"})
	muxRouter := mux.NewRouter()
	handler.RegisterRoutes(muxRouter)
	ts := httptest.NewServer(muxRouter)
	t.Cleanup(ts.Close)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/mcp",
		strings.NewReader(marshalRequest(t, 1, "initialize", map[string]any{
			"protocolVersion": "2025-03-26",
		})))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.example.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for disallowed origin, got %d", resp.StatusCode)
	}
}

// TestStreamable_MissingProtocolVersion verifies that non-initialize requests
// without MCP-Protocol-Version return 400.
func TestStreamable_MissingProtocolVersion(t *testing.T) {
	ts := newStreamableServer(t)

	// Initialize to get a session.
	initResp := postMCP(t, ts, nil, 1, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
	})
	sessionID := initResp.Header.Get("Mcp-Session-Id")
	io.Copy(io.Discard, initResp.Body) //nolint:errcheck
	initResp.Body.Close()

	// tools/list WITHOUT MCP-Protocol-Version header.
	resp := postMCP(t, ts, map[string]string{"Mcp-Session-Id": sessionID}, 2, "tools/list", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing MCP-Protocol-Version, got %d", resp.StatusCode)
	}
}

// TestStreamable_DeleteSession verifies that DELETE /mcp terminates a session.
func TestStreamable_DeleteSession(t *testing.T) {
	ts := newStreamableServer(t)

	initResp := postMCP(t, ts, nil, 1, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
	})
	sessionID := initResp.Header.Get("Mcp-Session-Id")
	io.Copy(io.Discard, initResp.Body) //nolint:errcheck
	initResp.Body.Close()

	// DELETE the session.
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodDelete, ts.URL+"/mcp", nil)
	req.Header.Set("Mcp-Session-Id", sessionID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /mcp: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("DELETE session: expected 200, got %d", resp.StatusCode)
	}

	// A subsequent request with that session ID must return 404.
	resp2 := postMCP(t, ts, map[string]string{
		"Mcp-Session-Id":       sessionID,
		"MCP-Protocol-Version": "2025-03-26",
	}, 2, "tools/list", nil)
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("after delete: expected 404, got %d", resp2.StatusCode)
	}
}

// TestStreamable_SSEGetMCP opens a GET /mcp SSE stream and verifies the server
// can push responses on it when the client POST includes Accept: text/event-stream.
func TestStreamable_SSEGetMCP(t *testing.T) {
	router := mcp.NewMcpRpcRouter("test-server", "0.0.1", buildTools())
	handler := mcp.NewMcpServerHandler(router, 0, nil)
	muxRouter := mux.NewRouter()
	handler.RegisterRoutes(muxRouter)
	ts := httptest.NewServer(muxRouter)
	t.Cleanup(ts.Close)

	// 1. Initialize
	initResp := postMCP(t, ts, nil, 1, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
	})
	sessionID := initResp.Header.Get("Mcp-Session-Id")
	io.Copy(io.Discard, initResp.Body) //nolint:errcheck
	initResp.Body.Close()

	// 2. Open GET /mcp SSE stream in a goroutine.
	sseReady := make(chan struct{})
	sseLines := make(chan string, 8)
	go func() {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+"/mcp", nil)
		req.Header.Set("Mcp-Session-Id", sessionID)
		req.Header.Set("MCP-Protocol-Version", "2025-03-26")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		close(sseReady)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data:") {
				sseLines <- strings.TrimPrefix(line, "data:")
			}
		}
	}()

	<-sseReady // wait for SSE connection to open

	// 3. POST tools/list with Accept: text/event-stream so the response is pushed.
	sessionHeaders := map[string]string{
		"Mcp-Session-Id":       sessionID,
		"MCP-Protocol-Version": "2025-03-26",
		"Accept":               "text/event-stream",
	}
	listResp := postMCP(t, ts, sessionHeaders, 2, "tools/list", nil)
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusAccepted {
		t.Fatalf("tools/list with Accept:SSE: expected 202, got %d", listResp.StatusCode)
	}

	// 4. The SSE channel must deliver the response.
	select {
	case dataLine := <-sseLines:
		var rpcResp mcp.JSONRPCResponse
		if err := json.Unmarshal([]byte(strings.TrimSpace(dataLine)), &rpcResp); err != nil {
			t.Fatalf("parse SSE data: %v", err)
		}
		if rpcResp.Error != nil {
			t.Errorf("tools/list via SSE: error %+v", rpcResp.Error)
		}
	case <-t.Context().Done():
		t.Fatal("timed out waiting for SSE message")
	}
}
