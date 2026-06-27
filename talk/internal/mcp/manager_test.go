package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/xvThomas/talk-backend/talk/internal/domain"
)

// mcpTool constructs an mcp.Tool for testing.
func mcpTool(name, description string, inputSchema any) mcp.Tool {
	return mcp.Tool{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
	}
}

// stubRegistry is a test double for Registry.
type stubRegistry struct {
	configs []ServerConfig
	err     error
}

func (r *stubRegistry) Add(_ context.Context, _ ServerConfig) error { return nil }
func (r *stubRegistry) Remove(_ context.Context, _ string) error    { return nil }
func (r *stubRegistry) Get(_ context.Context, _ string) (ServerConfig, error) {
	return ServerConfig{}, nil
}
func (r *stubRegistry) List(_ context.Context) ([]ServerConfig, error) {
	return r.configs, r.err
}

func TestNewManager(t *testing.T) {
	reg := &stubRegistry{}
	m := NewManager(reg)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if len(m.Tools()) != 0 {
		t.Errorf("expected 0 tools, got %d", len(m.Tools()))
	}
	if len(m.Statuses()) != 0 {
		t.Errorf("expected 0 statuses, got %d", len(m.Statuses()))
	}
}

func TestManager_ConnectAll_RegistryError(t *testing.T) {
	reg := &stubRegistry{err: fmt.Errorf("db down")}
	m := NewManager(reg)
	m.ConnectAll(context.Background())

	statuses := m.Statuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Error == "" {
		t.Error("expected error in status")
	}
	if len(m.Tools()) != 0 {
		t.Errorf("expected 0 tools on error, got %d", len(m.Tools()))
	}
}

func TestManager_ConnectAll_ConnectionFailure(t *testing.T) {
	// Use an unreachable URL to trigger a connection error.
	reg := &stubRegistry{
		configs: []ServerConfig{
			{ID: "srv-1", Name: "broken", URL: "http://127.0.0.1:1/mcp", AuthType: AuthTypeNone},
		},
	}
	m := NewManager(reg)

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	m.ConnectAll(ctx)

	statuses := m.Statuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Connected {
		t.Error("expected not connected")
	}
	if statuses[0].Error == "" {
		t.Error("expected error message")
	}
	if len(m.Tools()) != 0 {
		t.Errorf("expected 0 tools, got %d", len(m.Tools()))
	}
}

func TestManager_Close_NoSessions(t *testing.T) {
	m := NewManager(&stubRegistry{})
	// Close on empty manager should not panic.
	m.Close()
}

func TestManager_Disconnect_Unknown(t *testing.T) {
	m := NewManager(&stubRegistry{})
	// Disconnect an unknown ID should not panic.
	m.Disconnect("nonexistent")
}

func TestManager_Refresh_Empty(t *testing.T) {
	m := NewManager(&stubRegistry{})
	count := m.Refresh(context.Background())
	if count != 0 {
		t.Errorf("expected 0 tools from refresh, got %d", count)
	}
}

func TestManager_Connect_ConnectionFailure(t *testing.T) {
	reg := &stubRegistry{}
	m := NewManager(reg)

	cfg := ServerConfig{ID: "srv-1", Name: "broken", URL: "http://127.0.0.1:1/mcp", AuthType: AuthTypeNone}
	status, err := m.Connect(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected connection error")
	}
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if status.Connected {
		t.Fatal("expected disconnected status")
	}
	if status.Error == "" {
		t.Fatal("expected status error message")
	}
	if len(m.Statuses()) != 0 {
		t.Fatalf("expected manager statuses to remain empty on connect error, got %d", len(m.Statuses()))
	}
}

func TestManager_ConnectWrapsServerIdentityInError(t *testing.T) {
	m := NewManager(&stubRegistry{})
	cfg := ServerConfig{ID: "srv-1", Name: "failing-server", URL: "http://127.0.0.1:1/mcp", AuthType: AuthTypeNone}

	_, err := m.connect(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected connect() error")
	}
	if !strings.Contains(err.Error(), "failing-server") {
		t.Fatalf("expected server name in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "http://127.0.0.1:1/mcp") {
		t.Fatalf("expected server URL in error, got: %v", err)
	}
}

func TestBuildHTTPClient_NoAuth(t *testing.T) {
	cfg := ServerConfig{AuthType: AuthTypeNone}
	client := buildHTTPClient(cfg)
	if client != http.DefaultClient {
		t.Error("expected DefaultClient for AuthTypeNone")
	}
}

func TestBuildHTTPClient_APIKeyEmpty(t *testing.T) {
	cfg := ServerConfig{AuthType: AuthTypeAPIKey, APIKey: ""}
	client := buildHTTPClient(cfg)
	if client != http.DefaultClient {
		t.Error("expected DefaultClient when APIKey is empty")
	}
}

func TestBuildHTTPClient_APIKey(t *testing.T) {
	cfg := ServerConfig{AuthType: AuthTypeAPIKey, APIKey: "test-key-123"}
	client := buildHTTPClient(cfg)
	if client == http.DefaultClient {
		t.Fatal("expected custom client for API key auth")
	}

	// Verify the transport injects the header.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-API-Key")
		if got != "test-key-123" {
			t.Errorf("expected X-API-Key %q, got %q", "test-key-123", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
}

func TestToolAdapter_InputSchema_Nil(t *testing.T) {
	adapter := &mcpToolAdapter{
		serverName: "test-server",
		tool:       mcpTool("my-tool", "does things", nil),
	}
	if adapter.Name() != "my-tool" {
		t.Errorf("expected name %q, got %q", "my-tool", adapter.Name())
	}
	if adapter.Description() != "does things" {
		t.Errorf("expected description %q, got %q", "does things", adapter.Description())
	}
	schema, err := adapter.InputSchema()
	if err != nil {
		t.Fatalf("InputSchema error: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("expected type=object, got %v", schema["type"])
	}
}

func TestToolAdapter_OutputSchema(t *testing.T) {
	adapter := &mcpToolAdapter{
		serverName: "test-server",
		tool:       mcpTool("my-tool", "does things", nil),
	}
	schema, err := adapter.OutputSchema()
	if err != nil {
		t.Fatalf("OutputSchema error: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("expected type=object, got %v", schema["type"])
	}
}

func TestToolAdapter_InputSchema_MapDirectAssertion(t *testing.T) {
	adapter := &mcpToolAdapter{
		serverName: "test-server",
		tool: mcpTool("my-tool", "does things", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"city": map[string]any{"type": "string"},
			},
		}),
	}

	schema, err := adapter.InputSchema()
	if err != nil {
		t.Fatalf("InputSchema error: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("expected type=object, got %v", schema["type"])
	}
}

func TestToolAdapter_InputSchema_FallbackMarshalUnmarshal(t *testing.T) {
	type schemaDTO struct {
		Type string `json:"type"`
	}

	adapter := &mcpToolAdapter{
		serverName: "test-server",
		tool:       mcpTool("my-tool", "does things", schemaDTO{Type: "object"}),
	}

	schema, err := adapter.InputSchema()
	if err != nil {
		t.Fatalf("InputSchema fallback error: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("expected type=object, got %v", schema["type"])
	}
}

func TestToolAdapter_ExtractTextContent(t *testing.T) {
	content := []mcp.Content{
		&mcp.TextContent{Text: "line-1"},
		&mcp.TextContent{Text: "line-2"},
	}

	got := extractTextContent(content)
	if got != "line-1\nline-2" {
		t.Fatalf("extractTextContent = %q, want %q", got, "line-1\\nline-2")
	}
}

func TestManager_RebuildToolsExcludingFiltersByServerName(t *testing.T) {
	m := NewManager(&stubRegistry{})
	m.statuses = []ServerStatus{
		{Config: ServerConfig{ID: "srv-1", Name: "alpha"}},
		{Config: ServerConfig{ID: "srv-2", Name: "beta"}},
	}
	m.tools = []domain.Tool{
		&mcpToolAdapter{serverName: "alpha", tool: mcp.Tool{Name: "tool-a"}},
		&mcpToolAdapter{serverName: "beta", tool: mcp.Tool{Name: "tool-b"}},
	}

	m.rebuildToolsExcluding("srv-1")

	if len(m.tools) != 1 {
		t.Fatalf("expected 1 tool after exclude, got %d", len(m.tools))
	}
	if adapter, ok := m.tools[0].(*mcpToolAdapter); !ok || adapter.serverName != "beta" {
		t.Fatalf("remaining tool server = %v, want beta", m.tools[0])
	}
	if len(m.statuses) != 1 || m.statuses[0].Config.ID != "srv-2" {
		t.Fatalf("unexpected statuses after exclude: %+v", m.statuses)
	}
}

func connectInMemorySession(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	return session
}

func TestToolAdapter_Execute_Success(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	server.AddTool(&mcp.Tool{
		Name:        "echo",
		Description: "echoes text",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"msg": map[string]any{"type": "string"}}},
	}, func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Msg string `json:"msg"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, err
		}
		msg := args.Msg
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "echo:" + msg}}}, nil
	})

	session := connectInMemorySession(t, server)
	defer func() { _ = session.Close() }()

	adapter := &mcpToolAdapter{
		serverName: "test-server",
		tool:       mcpTool("echo", "echoes text", nil),
		session:    session,
	}

	got, err := adapter.Execute(context.Background(), map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got["content"] != "echo:hello" {
		t.Fatalf("Execute() content = %v, want %q", got["content"], "echo:hello")
	}
}

func TestToolAdapter_Execute_ToolError(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	server.AddTool(&mcp.Tool{
		Name:        "fail",
		Description: "always fails",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, errors.New("boom")
	})

	session := connectInMemorySession(t, server)
	defer func() { _ = session.Close() }()

	adapter := &mcpToolAdapter{
		serverName: "test-server",
		tool:       mcpTool("fail", "always fails", nil),
		session:    session,
	}

	_, err := adapter.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected Execute() error")
	}
	if !strings.Contains(err.Error(), `calling tool "fail" on server "test-server"`) {
		t.Fatalf("unexpected Execute() error: %v", err)
	}
}
