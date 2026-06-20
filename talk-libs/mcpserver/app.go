package mcpserver

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xvThomas/talk-backend/talk-libs/logger"
)

// ToolRegistrar registers a tool on an mcp.Server.
// Use RegisterTool to create one from an MCPTool.
type ToolRegistrar struct {
	Name     string
	Register func(s *mcp.Server)
}

// RegisterTool returns a ToolRegistrar that adds the given MCPTool to an mcp.Server.
func RegisterTool[TInput, TOutput any](tool MCPTool[TInput, TOutput]) ToolRegistrar {
	return ToolRegistrar{
		Name: tool.Name(),
		Register: func(s *mcp.Server) {
			mcp.AddTool(s, &mcp.Tool{
				Name:        tool.Name(),
				Description: tool.Description(),
			}, func(ctx context.Context, _ *mcp.CallToolRequest, args TInput) (*mcp.CallToolResult, TOutput, error) {
				out, err := tool.Call(ctx, args)
				return nil, out, err
			})
		},
	}
}

// ASProxyConfig enables the Authorization Server proxy mode. When set, the MCP
// server exposes its own OAuth endpoints that inject the required audience
// before forwarding to the upstream Authorization Server. This is necessary
// for OAuth clients (e.g. Claude.ai) that do not send the audience parameter
// in authorization requests (Auth0 requires it to issue a JWT access token).
type ASProxyConfig struct {
	// Audience is the audience value to inject into upstream authorize requests
	// (e.g. "owm-mcp"). Required.
	Audience string

	// UpstreamAuthorizeURL is the upstream authorization endpoint. When empty,
	// defaults to AuthorizationServerURL + "/authorize".
	UpstreamAuthorizeURL string

	// UpstreamTokenURL is the upstream token endpoint. When empty, defaults to
	// AuthorizationServerURL + "/oauth/token".
	UpstreamTokenURL string

	// ClientSecret is the OAuth client secret to inject into token proxy
	// requests. Optional: only needed for confidential clients whose secret
	// is not sent by the OAuth client itself.
	ClientSecret string
}

// OAuthConfig holds the OAuth 2.0 configuration for the server acting as a
// Resource Server. The Authorization Server (e.g. Auth0, Keycloak) is external.
type OAuthConfig struct {
	// AuthorizationServerURL is the issuer URL of the external Authorization
	// Server (e.g. "https://my-tenant.auth0.com").
	AuthorizationServerURL string

	// ResourceBaseURL is the public-facing base URL of this server
	// (e.g. "https://xxxx.ngrok-free.app"). Used in the OAuth metadata
	// and WWW-Authenticate header. When empty, falls back to http://{addr}.
	ResourceBaseURL string

	// Scopes lists the OAuth scopes required to access the MCP endpoints.
	Scopes []string

	// TokenVerifier validates the Bearer token from incoming requests.
	// The caller must provide this function. Typical implementations verify
	// the JWT signature against the AS's JWKS endpoint or call the AS's
	// token introspection endpoint.
	TokenVerifier auth.TokenVerifier

	// ASProxy enables the Authorization Server proxy mode. When non-nil, the
	// server exposes /authorize, /token, and
	// /.well-known/oauth-authorization-server endpoints that proxy to the
	// upstream AS while injecting the required audience parameter.
	ASProxy *ASProxyConfig
}

// Option configures an App. Use the With* functions to create options.
type Option func(*App)

// WithAPIKey enables X-API-Key header authentication.
func WithAPIKey(key string) Option {
	return func(a *App) { a.apiKey = &key }
}

// WithOAuth enables OAuth 2.0 Bearer token authentication.
func WithOAuth(cfg *OAuthConfig) Option {
	return func(a *App) { a.oauth = cfg }
}

// WithTools registers tools on the MCP server.
func WithTools(tools ...ToolRegistrar) Option {
	return func(a *App) { a.tools = append(a.tools, tools...) }
}

// App is a reusable MCP server runner that handles CLI flags, transport
// routing (stdio / HTTP), and server creation.
//
// Create with NewApp and configure with functional options:
//
//	app := mcpserver.NewApp("my-mcp", "1.0.0",
//	    mcpserver.WithAPIKey(env.APIKey),
//	    mcpserver.WithTools(mcpserver.RegisterTool(myTool)),
//	)
//	app.Run()
type App struct {
	name     string
	version  string
	tools    []ToolRegistrar
	prompts  []PromptRegistrar
	apiKey   *string             // optional: X-API-Key header authentication
	oauth    *OAuthConfig        // optional: OAuth Bearer token authentication
	security *HTTPSecurityConfig // optional: HTTP security settings
}

// NewApp creates an App configured with the given options.
func NewApp(name, version string, opts ...Option) *App {
	a := &App{name: name, version: version}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Run parses CLI flags and starts the server using the selected transport.
func (a *App) Run() {
	log := logger.GetLogger()

	transport := flag.String("transport", "stdio", "transport to use: stdio | http")
	addr := flag.String("addr", "localhost:8080", "address to listen on (HTTP transport)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s --transport stdio\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --transport http --addr localhost:8080\n", os.Args[0])
	}
	flag.Parse()

	log.Info("MCP Server", "name", a.name, "version", a.version)

	toolNames := make([]string, len(a.tools))
	for i, t := range a.tools {
		toolNames[i] = t.Name
	}
	log.Info("Available tools", "count", len(toolNames), "tools", toolNames)

	promptNames := make([]string, len(a.prompts))
	for i, p := range a.prompts {
		promptNames[i] = p.Name
	}
	if len(promptNames) > 0 {
		log.Info("Available prompts", "count", len(promptNames), "prompts", promptNames)
	}

	switch *transport {
	case "stdio":
		a.runStdio()
	case "http":
		a.runHTTP(*addr)
	default:
		log.Error("unknown transport", "transport", *transport)
		flag.Usage()
		os.Exit(1)
	}
}

func (a *App) newServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: a.name, Version: a.version}, nil)
	for _, t := range a.tools {
		t.Register(s)
	}
	for _, p := range a.prompts {
		p.Register(s)
	}
	return s
}

func (a *App) runStdio() {
	log := logger.GetLogger()
	s := a.newServer()
	log.Info("Stdio server running")
	if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Error("stdio server failed", "error", err)
		os.Exit(1)
	}
}
