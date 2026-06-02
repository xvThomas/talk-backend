package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
