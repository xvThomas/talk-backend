package mcp_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"talks/pkg/mcp"
)

// TestStdio_Initialize verifies the initialize response:
//   - protocolVersion is negotiated to the highest supported version
//   - serverInfo (not "server") contains name and version
//   - capabilities.tools is present
func TestStdio_Initialize(t *testing.T) {
	router := mcp.NewMcpRpcRouter("test-server", "0.0.1", buildTools())

	req := marshalRequest(t, 1, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"clientInfo":      map[string]any{"name": "test-client", "version": "0.0.1"},
	})

	var reqMsg mcp.JSONRPCRequest
	if err := json.Unmarshal([]byte(req), &reqMsg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	resp := router.RouteRequest(context.Background(), reqMsg)
	if resp.Error != nil {
		t.Fatalf("initialize error: %+v", resp.Error)
	}

	b, _ := json.Marshal(resp.Result)
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
		Capabilities struct {
			Tools map[string]any `json:"tools"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}

	if result.ProtocolVersion != "2025-03-26" {
		t.Errorf("expected protocolVersion=2025-03-26, got %q", result.ProtocolVersion)
	}
	if result.ServerInfo.Name != "test-server" {
		t.Errorf("expected serverInfo.name=test-server, got %q", result.ServerInfo.Name)
	}
	if result.ServerInfo.Version != "0.0.1" {
		t.Errorf("expected serverInfo.version=0.0.1, got %q", result.ServerInfo.Version)
	}
	if result.Capabilities.Tools == nil {
		t.Error("expected capabilities.tools to be present")
	}
}

// TestStdio_VersionNegotiation verifies that a client requesting an older version
// receives protoV1 (2024-11-05) and a future version is capped at the highest supported.
func TestStdio_VersionNegotiation(t *testing.T) {
	router := mcp.NewMcpRpcRouter("test-server", "0.0.1", buildTools())

	cases := []struct {
		clientVersion string
		wantVersion   string
	}{
		{"2024-11-05", "2024-11-05"},
		{"2025-03-26", "2025-03-26"},
		{"2099-01-01", "2025-03-26"},
		{"", "2024-11-05"},
	}

	for _, tc := range cases {
		t.Run("client="+tc.clientVersion, func(t *testing.T) {
			params := map[string]any{"protocolVersion": tc.clientVersion}
			req := marshalRequest(t, 1, "initialize", params)

			var reqMsg mcp.JSONRPCRequest
			if err := json.Unmarshal([]byte(req), &reqMsg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			resp := router.RouteRequest(context.Background(), reqMsg)
			if resp.Error != nil {
				t.Fatalf("error: %+v", resp.Error)
			}
			b, _ := json.Marshal(resp.Result)
			var result struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			if err := json.Unmarshal(b, &result); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if result.ProtocolVersion != tc.wantVersion {
				t.Errorf("client=%q: want %q, got %q", tc.clientVersion, tc.wantVersion, result.ProtocolVersion)
			}
		})
	}
}

// TestStdio_SumTool sends initialize -> tools/list -> tools/call via the router
// in a simulated stdio line-by-line loop. Validates full sequence.
func TestStdio_SumTool(t *testing.T) {
	router := mcp.NewMcpRpcRouter("test-server", "0.0.1", buildTools())

	inputLines := []string{
		marshalRequest(t, 1, "initialize", map[string]any{
			"protocolVersion": "2025-03-26",
			"clientInfo":      map[string]any{"name": "test-client", "version": "0.0.1"},
		}),
		marshalRequest(t, 2, "tools/list", nil),
		marshalRequest(t, 3, "tools/call", callRequest{
			Name:      "sum",
			Arguments: map[string]any{"a": 3, "b": 4},
		}),
	}

	input := strings.Join(inputLines, "\n") + "\n"
	reader := bufio.NewReader(strings.NewReader(input))
	var output bytes.Buffer

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("read line: %v", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var req mcp.JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}
		resp := router.RouteRequest(t.Context(), req)
		b, _ := json.Marshal(resp)
		output.Write(b)
		output.WriteByte('\n')
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 response lines, got %d:\n%s", len(lines), output.String())
	}

	initResp := decodeResponse(t, lines[0])
	if initResp.Error != nil {
		t.Fatalf("initialize error: %+v", initResp.Error)
	}

	listResp := decodeResponse(t, lines[1])
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

	callResp := decodeResponse(t, lines[2])
	if callResp.Error != nil {
		t.Fatalf("tools/call error: %+v", callResp.Error)
	}
	callJSON, _ := json.Marshal(callResp.Result)
	var callResult struct {
		Content map[string]float64 `json:"content"`
	}
	if err := json.Unmarshal(callJSON, &callResult); err != nil {
		t.Fatalf("parse tools/call result: %v", err)
	}
	if callResult.Content["sum"] != 7 {
		t.Errorf("expected sum=7, got %v", callResult.Content["sum"])
	}
}

// TestStdio_NotificationNoResponse verifies that a notification (nil ID)
// produces no response line in the stdio output.
func TestStdio_NotificationNoResponse(t *testing.T) {
	router := mcp.NewMcpRpcRouter("test-server", "0.0.1", buildTools())

	notification := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	normalReq := marshalRequest(t, 42, "tools/list", nil)

	input := notification + "\n" + normalReq + "\n"
	reader := bufio.NewReader(strings.NewReader(input))
	var output bytes.Buffer

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("read line: %v", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var req mcp.JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		// Mimic McpServer.readStdio: skip response for notifications.
		if req.ID == nil {
			router.RouteRequest(context.Background(), req)
			continue
		}
		resp := router.RouteRequest(context.Background(), req)
		b, _ := json.Marshal(resp)
		output.Write(b)
		output.WriteByte('\n')
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 response (no response for notification), got %d:\n%s", len(lines), output.String())
	}
	resp := decodeResponse(t, lines[0])
	if resp.Error != nil {
		t.Fatalf("tools/list error: %+v", resp.Error)
	}
}
