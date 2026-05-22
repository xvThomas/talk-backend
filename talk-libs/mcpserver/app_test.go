package mcpserver

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewApp_Defaults(t *testing.T) {
	app := NewApp("test-server", "1.0.0")

	if app.name != "test-server" {
		t.Errorf("expected name %q, got %q", "test-server", app.name)
	}
	if app.version != "1.0.0" {
		t.Errorf("expected version %q, got %q", "1.0.0", app.version)
	}
	if app.apiKey != nil {
		t.Error("expected nil apiKey by default")
	}
	if app.oauth != nil {
		t.Error("expected nil oauth by default")
	}
	if app.security != nil {
		t.Error("expected nil security by default")
	}
	if len(app.tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(app.tools))
	}
	if len(app.prompts) != 0 {
		t.Errorf("expected 0 prompts, got %d", len(app.prompts))
	}
}

func TestWithAPIKey(t *testing.T) {
	app := NewApp("test", "1.0.0", WithAPIKey("my-secret"))

	if app.apiKey == nil {
		t.Fatal("expected apiKey to be set")
	}
	if *app.apiKey != "my-secret" {
		t.Errorf("expected apiKey %q, got %q", "my-secret", *app.apiKey)
	}
}

func TestWithOAuth(t *testing.T) {
	cfg := &OAuthConfig{
		AuthorizationServerURL: "https://auth.example.com",
		Scopes:                 []string{"read", "write"},
	}
	app := NewApp("test", "1.0.0", WithOAuth(cfg))

	if app.oauth == nil {
		t.Fatal("expected oauth to be set")
	}
	if app.oauth.AuthorizationServerURL != "https://auth.example.com" {
		t.Errorf("expected AS URL %q, got %q", "https://auth.example.com", app.oauth.AuthorizationServerURL)
	}
	if len(app.oauth.Scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(app.oauth.Scopes))
	}
}

func TestWithTools(t *testing.T) {
	t1 := ToolRegistrar{Name: "tool1", Register: func(_ *mcp.Server) {}}
	t2 := ToolRegistrar{Name: "tool2", Register: func(_ *mcp.Server) {}}

	app := NewApp("test", "1.0.0", WithTools(t1, t2))

	if len(app.tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(app.tools))
	}
	if app.tools[0].Name != "tool1" {
		t.Errorf("expected tool[0] name %q, got %q", "tool1", app.tools[0].Name)
	}
	if app.tools[1].Name != "tool2" {
		t.Errorf("expected tool[1] name %q, got %q", "tool2", app.tools[1].Name)
	}
}

func TestWithTools_Append(t *testing.T) {
	t1 := ToolRegistrar{Name: "tool1", Register: func(_ *mcp.Server) {}}
	t2 := ToolRegistrar{Name: "tool2", Register: func(_ *mcp.Server) {}}

	app := NewApp("test", "1.0.0", WithTools(t1), WithTools(t2))

	if len(app.tools) != 2 {
		t.Fatalf("expected 2 tools after appending, got %d", len(app.tools))
	}
}

func TestWithPrompts(t *testing.T) {
	p := PromptRegistrar{Name: "greeting", Register: func(_ *mcp.Server) {}}
	app := NewApp("test", "1.0.0", WithPrompts(p))

	if len(app.prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(app.prompts))
	}
	if app.prompts[0].Name != "greeting" {
		t.Errorf("expected prompt name %q, got %q", "greeting", app.prompts[0].Name)
	}
}

func TestWithHTTPSecurity(t *testing.T) {
	cfg := HTTPSecurityConfig{
		RateLimit:    100,
		RateBurst:    200,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	app := NewApp("test", "1.0.0", WithHTTPSecurity(cfg))

	if app.security == nil {
		t.Fatal("expected security to be set")
	}
	if app.security.RateLimit != 100 {
		t.Errorf("expected RateLimit 100, got %d", app.security.RateLimit)
	}
	if app.security.RateBurst != 200 {
		t.Errorf("expected RateBurst 200, got %d", app.security.RateBurst)
	}
}

func TestNewApp_MultipleOptions(t *testing.T) {
	verifier := func(_ context.Context, _ string, _ *http.Request) (*auth.TokenInfo, error) {
		return &auth.TokenInfo{UserID: "u"}, nil
	}
	app := NewApp("multi", "2.0.0",
		WithAPIKey("key"),
		WithOAuth(&OAuthConfig{
			AuthorizationServerURL: "https://as.test",
			TokenVerifier:          verifier,
		}),
		WithTools(ToolRegistrar{Name: "t1", Register: func(_ *mcp.Server) {}}),
		WithPrompts(PromptRegistrar{Name: "p1", Register: func(_ *mcp.Server) {}}),
		WithHTTPSecurity(HTTPSecurityConfig{RateLimit: 10}),
	)

	if app.name != "multi" || app.version != "2.0.0" {
		t.Error("name/version mismatch")
	}
	if app.apiKey == nil || *app.apiKey != "key" {
		t.Error("apiKey not set correctly")
	}
	if app.oauth == nil {
		t.Error("oauth not set")
	}
	if len(app.tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(app.tools))
	}
	if len(app.prompts) != 1 {
		t.Errorf("expected 1 prompt, got %d", len(app.prompts))
	}
	if app.security == nil || app.security.RateLimit != 10 {
		t.Error("security not set correctly")
	}
}

// mockTool implements MCPTool for testing RegisterTool.
type mockTool struct{}

type mockInput struct {
	Value int `json:"value"`
}

type mockOutput struct {
	Result int `json:"result"`
}

func (m mockTool) Name() string        { return "mock-tool" }
func (m mockTool) Description() string { return "A mock tool for testing" }
func (m mockTool) Call(_ context.Context, input mockInput) (mockOutput, error) {
	return mockOutput{Result: input.Value * 2}, nil
}

func TestRegisterTool(t *testing.T) {
	tr := RegisterTool[mockInput, mockOutput](mockTool{})

	if tr.Name != "mock-tool" {
		t.Errorf("expected name %q, got %q", "mock-tool", tr.Name)
	}
	if tr.Register == nil {
		t.Fatal("expected Register function to be non-nil")
	}

	// Verify it doesn't panic when registering on a real server.
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.1"}, nil)
	tr.Register(s)
}

func TestRegisterPrompt(t *testing.T) {
	p := Prompt{
		Name:        "greet",
		Description: "Greets a user",
		Arguments: []PromptArgument{
			{Name: "name", Description: "User name", Required: true},
		},
		Messages: []PromptMessage{
			{Role: "user", Text: "Hello {{name}}!"},
		},
	}

	pr := RegisterPrompt(p)

	if pr.Name != "greet" {
		t.Errorf("expected name %q, got %q", "greet", pr.Name)
	}
	if pr.Register == nil {
		t.Fatal("expected Register function to be non-nil")
	}

	// Verify it doesn't panic when registering on a real server.
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.1"}, nil)
	pr.Register(s)
}

func TestNewServer(t *testing.T) {
	tr := RegisterTool[mockInput, mockOutput](mockTool{})
	pr := RegisterPrompt(Prompt{
		Name:        "test-prompt",
		Description: "Test",
		Messages:    []PromptMessage{{Role: "user", Text: "hi"}},
	})

	app := NewApp("srv", "1.0.0", WithTools(tr), WithPrompts(pr))
	s := app.newServer()

	if s == nil {
		t.Fatal("expected non-nil server")
	}
}
