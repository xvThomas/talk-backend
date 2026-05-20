package mcpserver

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/xvThomas/LLMClientWrapper/talk-libs/logger"
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
	name    string
	version string
	tools   []ToolRegistrar
	prompts []PromptRegistrar
	apiKey  *string      // optional: X-API-Key header authentication
	oauth   *OAuthConfig // optional: OAuth Bearer token authentication
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

func (a *App) runHTTP(addr string) {
	log := logger.GetLogger()

	serverFactory := func(_ *http.Request) *mcp.Server {
		return a.newServer()
	}

	// Disable localhost DNS rebinding protection when behind a reverse proxy.
	behindProxy := a.oauth != nil && a.oauth.ResourceBaseURL != ""
	sseHandler := mcp.NewSSEHandler(serverFactory, &mcp.SSEOptions{
		DisableLocalhostProtection: behindProxy,
	})
	streamableHandler := mcp.NewStreamableHTTPHandler(serverFactory, &mcp.StreamableHTTPOptions{
		DisableLocalhostProtection: behindProxy,
	})

	mux := http.NewServeMux()
	middleware := a.buildAuthMiddleware(addr, mux)

	mux.Handle("/sse", middleware(sseHandler))
	mux.Handle("/mcp", middleware(streamableHandler))

	// Wrap the mux with a response-capturing request logger for debugging.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpcMethod string
		if r.Body != nil && r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			_ = r.Body.Close()
			if err == nil {
				var envelope struct {
					Method string `json:"method"`
				}
				if json.Unmarshal(body, &envelope) == nil && envelope.Method != "" {
					rpcMethod = envelope.Method
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
			}
		}

		logArgs := []any{"method", r.Method, "path", r.URL.Path,
			"bearer", r.Header.Get("Authorization") != "",
			"apiKey", r.Header.Get("X-API-Key") != ""}
		if rpcMethod != "" {
			logArgs = append(logArgs, "rpc.method", rpcMethod)
		}
		log.Debug("incoming request", logArgs...)

		rw := &statusRecorder{ResponseWriter: w}
		mux.ServeHTTP(rw, r)
		log.Debug("response sent", "method", r.Method, "path", r.URL.Path, "status", rw.status)
	})

	log.Info("HTTP server listening", "addr", addr, "sse", "/sse", "streamable", "/mcp")
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}

// buildAuthMiddleware returns the HTTP middleware to apply based on the
// configured authentication methods. It also registers the OAuth protected
// resource metadata endpoint when OAuth is enabled.
func (a *App) buildAuthMiddleware(addr string, mux *http.ServeMux) func(http.Handler) http.Handler {
	log := logger.GetLogger()

	hasAPIKey := a.apiKey != nil && *a.apiKey != ""
	hasOAuth := a.oauth != nil

	// Resolve the public base URL for OAuth metadata / WWW-Authenticate.
	baseURL := "http://" + addr
	if hasOAuth && a.oauth.ResourceBaseURL != "" {
		baseURL = strings.TrimRight(a.oauth.ResourceBaseURL, "/")
	}

	if hasOAuth {
		a.registerOAuthMetadata(mux, baseURL)
	}

	switch {
	case hasAPIKey && hasOAuth:
		log.Info("auth: API Key + OAuth")
		return eitherAuthMiddleware(
			oauthBearerMiddleware(a.oauth, baseURL),
			apiKeyAuthMiddleware(*a.apiKey),
		)
	case hasOAuth:
		log.Info("auth: OAuth Bearer token")
		return oauthBearerMiddleware(a.oauth, baseURL)
	case hasAPIKey:
		log.Info("auth: API Key")
		return apiKeyAuthMiddleware(*a.apiKey)
	default:
		log.Warn("auth: NONE - server is not secured")
		return func(next http.Handler) http.Handler { return next }
	}
}

// registerOAuthMetadata serves the RFC 9728 Protected Resource Metadata
// at /.well-known/oauth-protected-resource so that OAuth-aware clients can
// discover which Authorization Server to use.
//
// When ASProxy is configured, the authorization_servers entry points to this
// server itself (the proxy) instead of the upstream AS. The proxy endpoints
// inject the audience parameter that upstream AS (e.g. Auth0) requires to
// issue a JWT access token.
func (a *App) registerOAuthMetadata(mux *http.ServeMux, baseURL string) {
	asURL := a.oauth.AuthorizationServerURL
	if a.oauth.ASProxy != nil {
		// Point OAuth clients at the proxy (this server) rather than upstream.
		asURL = baseURL
		registerASProxy(mux, baseURL, a.oauth)
	}

	metadata := &oauthex.ProtectedResourceMetadata{
		Resource:             baseURL + "/mcp",
		AuthorizationServers: []string{asURL},
		ScopesSupported:      a.oauth.Scopes,
	}
	mux.Handle("/.well-known/oauth-protected-resource",
		auth.ProtectedResourceMetadataHandler(metadata))
}

// oauthBearerMiddleware wraps auth.RequireBearerToken from the go-sdk.
func oauthBearerMiddleware(cfg *OAuthConfig, baseURL string) func(http.Handler) http.Handler {
	return auth.RequireBearerToken(cfg.TokenVerifier, &auth.RequireBearerTokenOptions{
		Scopes:              cfg.Scopes,
		ResourceMetadataURL: baseURL + "/.well-known/oauth-protected-resource",
	})
}

// eitherAuthMiddleware tries OAuth first when a Bearer token is present.
// If OAuth fails with 401 and the request also carries a valid X-API-Key,
// the API Key middleware is attempted as a fallback.
func eitherAuthMiddleware(oauthMW, apiKeyMW func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	log := logger.GetLogger()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			hasBearer := len(h) > 7 && strings.EqualFold(h[:7], "bearer ")
			hasAPIKey := r.Header.Get("X-API-Key") != ""

			if !hasBearer {
				apiKeyMW(next).ServeHTTP(w, r)
				return
			}

			// If only Bearer is present (no API Key fallback), go straight to OAuth.
			if !hasAPIKey {
				oauthMW(next).ServeHTTP(w, r)
				return
			}

			// Both are present: try OAuth first with a buffered response.
			buf := &bufferedResponseWriter{header: make(http.Header)}
			oauthMW(next).ServeHTTP(buf, r)
			if buf.status != http.StatusUnauthorized {
				// OAuth succeeded (or returned a non-401 error): commit the response.
				buf.writeTo(w)
				return
			}

			// OAuth failed with 401: fall back to API Key.
			log.Debug("auth fallback: OAuth failed, trying API Key")
			apiKeyMW(next).ServeHTTP(w, r)
		})
	}
}

// bufferedResponseWriter captures an HTTP response in memory so that the
// caller can decide whether to commit it to the real ResponseWriter.
type bufferedResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (b *bufferedResponseWriter) Header() http.Header { return b.header }

func (b *bufferedResponseWriter) WriteHeader(code int) { b.status = code }

func (b *bufferedResponseWriter) Write(data []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.body.Write(data)
}

func (b *bufferedResponseWriter) writeTo(w http.ResponseWriter) {
	for k, vs := range b.header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	if b.status != 0 {
		w.WriteHeader(b.status)
	}
	_, _ = w.Write(b.body.Bytes())
}

// apiKeyAuthMiddleware checks that the X-API-Key header matches the expected key.
func apiKeyAuthMiddleware(expectedKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := r.Header.Get("X-API-Key")
			if subtle.ConstantTimeCompare([]byte(provided), []byte(expectedKey)) != 1 {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}
