package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/auth"
)

func TestApiKeyAuthMiddleware_ValidKey(t *testing.T) {
	mw := apiKeyAuthMiddleware("secret-key")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("X-API-Key", "secret-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 with valid key, got %d", rr.Code)
	}
}

func TestApiKeyAuthMiddleware_InvalidKey(t *testing.T) {
	mw := apiKeyAuthMiddleware("secret-key")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with invalid key, got %d", rr.Code)
	}
}

func TestApiKeyAuthMiddleware_MissingKey(t *testing.T) {
	mw := apiKeyAuthMiddleware("secret-key")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with missing key, got %d", rr.Code)
	}
}

func TestEitherAuthMiddleware_APIKeyOnly(t *testing.T) {
	// Mock OAuth middleware that always rejects.
	oauthMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})
	}
	apiKeyMW := apiKeyAuthMiddleware("my-key")

	mw := eitherAuthMiddleware(oauthMW, apiKeyMW)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with only API key (no Bearer) → should route to apiKeyMW.
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("X-API-Key", "my-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 with valid API key (no Bearer), got %d", rr.Code)
	}
}

func TestEitherAuthMiddleware_BearerOnly(t *testing.T) {
	// Mock OAuth middleware that always accepts.
	oauthMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
	apiKeyMW := apiKeyAuthMiddleware("my-key")

	mw := eitherAuthMiddleware(oauthMW, apiKeyMW)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with only Bearer (no API key) → should route to oauthMW.
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 with valid Bearer token, got %d", rr.Code)
	}
}

func TestEitherAuthMiddleware_BothPresent_OAuthSucceeds(t *testing.T) {
	// OAuth accepts the request.
	oauthMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
	apiKeyMW := apiKeyAuthMiddleware("my-key")

	mw := eitherAuthMiddleware(oauthMW, apiKeyMW)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	req.Header.Set("X-API-Key", "my-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 when OAuth succeeds, got %d", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Errorf("expected body %q, got %q", "ok", rr.Body.String())
	}
}

func TestEitherAuthMiddleware_BothPresent_OAuthFails_APIKeyFallback(t *testing.T) {
	// OAuth rejects with 401.
	oauthMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})
	}
	apiKeyMW := apiKeyAuthMiddleware("my-key")

	mw := eitherAuthMiddleware(oauthMW, apiKeyMW)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	req.Header.Set("X-API-Key", "my-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 after API key fallback, got %d", rr.Code)
	}
}

func TestEitherAuthMiddleware_BothPresent_BothFail(t *testing.T) {
	// OAuth rejects with 401.
	oauthMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})
	}
	apiKeyMW := apiKeyAuthMiddleware("my-key")

	mw := eitherAuthMiddleware(oauthMW, apiKeyMW)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	req.Header.Set("X-API-Key", "wrong-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when both fail, got %d", rr.Code)
	}
}

func TestBufferedResponseWriter_WriteTo(t *testing.T) {
	buf := &bufferedResponseWriter{header: make(http.Header)}
	buf.Header().Set("X-Custom", "value")
	buf.WriteHeader(http.StatusCreated)
	_, _ = buf.Write([]byte("hello"))

	rr := httptest.NewRecorder()
	buf.writeTo(rr)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}
	if rr.Header().Get("X-Custom") != "value" {
		t.Errorf("expected X-Custom header, got %q", rr.Header().Get("X-Custom"))
	}
	if rr.Body.String() != "hello" {
		t.Errorf("expected body %q, got %q", "hello", rr.Body.String())
	}
}

func TestBufferedResponseWriter_DefaultStatus(t *testing.T) {
	buf := &bufferedResponseWriter{header: make(http.Header)}
	_, _ = buf.Write([]byte("data"))

	if buf.status != http.StatusOK {
		t.Errorf("expected implicit 200 on Write, got %d", buf.status)
	}
}

// --- Step 4: buildAuthMiddleware (4 branches) ---

func fakeVerifier() auth.TokenVerifier {
	return func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		if token == "valid-token" {
			return &auth.TokenInfo{UserID: "user1"}, nil
		}
		return nil, http.ErrAbortHandler
	}
}

func TestBuildAuthMiddleware_NoAuth(t *testing.T) {
	app := NewApp("test", "1.0.0")
	mux := http.NewServeMux()
	mw := app.buildAuthMiddleware("localhost:8080", mux)

	// No-auth middleware should pass requests through.
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 with no auth, got %d", rr.Code)
	}
}

func TestBuildAuthMiddleware_APIKeyOnly(t *testing.T) {
	app := NewApp("test", "1.0.0", WithAPIKey("test-key"))
	mux := http.NewServeMux()
	mw := app.buildAuthMiddleware("localhost:8080", mux)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Valid key.
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("X-API-Key", "test-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 with valid API key, got %d", rr.Code)
	}

	// Invalid key.
	req = httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("X-API-Key", "wrong")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with invalid API key, got %d", rr.Code)
	}
}

func TestBuildAuthMiddleware_OAuthOnly(t *testing.T) {
	app := NewApp("test", "1.0.0", WithOAuth(&OAuthConfig{
		AuthorizationServerURL: "https://auth.example.com",
		Scopes:                 []string{"read"},
		TokenVerifier:          fakeVerifier(),
	}))
	mux := http.NewServeMux()
	mw := app.buildAuthMiddleware("localhost:8080", mux)

	if mw == nil {
		t.Fatal("expected non-nil middleware")
	}

	// Verify that the metadata endpoint was registered.
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for oauth-protected-resource, got %d", rr.Code)
	}
}

func TestBuildAuthMiddleware_APIKeyAndOAuth(t *testing.T) {
	app := NewApp("test", "1.0.0",
		WithAPIKey("dual-key"),
		WithOAuth(&OAuthConfig{
			AuthorizationServerURL: "https://auth.example.com",
			Scopes:                 []string{"read"},
			TokenVerifier:          fakeVerifier(),
		}),
	)
	mux := http.NewServeMux()
	mw := app.buildAuthMiddleware("localhost:8080", mux)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// API key only should work.
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("X-API-Key", "dual-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 with API key in dual mode, got %d", rr.Code)
	}
}

func TestBuildAuthMiddleware_OAuthWithResourceBaseURL(t *testing.T) {
	app := NewApp("test", "1.0.0", WithOAuth(&OAuthConfig{
		AuthorizationServerURL: "https://auth.example.com",
		ResourceBaseURL:        "https://public.example.com",
		Scopes:                 []string{"read"},
		TokenVerifier:          fakeVerifier(),
	}))
	mux := http.NewServeMux()
	_ = app.buildAuthMiddleware("localhost:8080", mux)

	// Verify metadata uses ResourceBaseURL.
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var meta map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &meta); err != nil {
		t.Fatalf("failed to parse metadata JSON: %v", err)
	}
	resource, _ := meta["resource"].(string)
	if resource != "https://public.example.com/mcp" {
		t.Errorf("expected resource %q, got %q", "https://public.example.com/mcp", resource)
	}
}

// --- Step 5: registerOAuthMetadata + registerASProxy ---

func TestRegisterOAuthMetadata_NoProxy(t *testing.T) {
	app := NewApp("test", "1.0.0", WithOAuth(&OAuthConfig{
		AuthorizationServerURL: "https://auth.example.com",
		Scopes:                 []string{"read", "write"},
		TokenVerifier:          fakeVerifier(),
	}))
	mux := http.NewServeMux()
	app.registerOAuthMetadata(mux, "http://localhost:8080")

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var meta map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &meta); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if meta["resource"] != "http://localhost:8080/mcp" {
		t.Errorf("resource = %v, want http://localhost:8080/mcp", meta["resource"])
	}
	servers, _ := meta["authorization_servers"].([]any)
	if len(servers) != 1 || servers[0] != "https://auth.example.com" {
		t.Errorf("authorization_servers = %v, want [https://auth.example.com]", servers)
	}
}

func TestRegisterOAuthMetadata_WithProxy(t *testing.T) {
	app := NewApp("test", "1.0.0", WithOAuth(&OAuthConfig{
		AuthorizationServerURL: "https://auth.example.com",
		Scopes:                 []string{"read"},
		TokenVerifier:          fakeVerifier(),
		ASProxy: &ASProxyConfig{
			Audience: "my-api",
		},
	}))
	mux := http.NewServeMux()
	app.registerOAuthMetadata(mux, "http://localhost:8080")

	// Metadata should point to the proxy (this server) rather than upstream.
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var meta map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &meta); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	servers, _ := meta["authorization_servers"].([]any)
	if len(servers) != 1 || servers[0] != "http://localhost:8080" {
		t.Errorf("authorization_servers should point to proxy, got %v", servers)
	}
}

func TestRegisterASProxy_ASMetadata(t *testing.T) {
	cfg := &OAuthConfig{
		AuthorizationServerURL: "https://auth.example.com",
		ASProxy: &ASProxyConfig{
			Audience: "my-api",
		},
	}
	mux := http.NewServeMux()
	registerASProxy(mux, "http://localhost:8080", cfg)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var meta map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &meta); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if meta["issuer"] != "http://localhost:8080" {
		t.Errorf("issuer = %v, want http://localhost:8080", meta["issuer"])
	}
	if meta["authorization_endpoint"] != "http://localhost:8080/authorize" {
		t.Errorf("authorization_endpoint = %v", meta["authorization_endpoint"])
	}
	if meta["token_endpoint"] != "http://localhost:8080/token" {
		t.Errorf("token_endpoint = %v", meta["token_endpoint"])
	}
}

func TestRegisterASProxy_Authorize(t *testing.T) {
	cfg := &OAuthConfig{
		AuthorizationServerURL: "https://auth.example.com",
		ASProxy: &ASProxyConfig{
			Audience: "my-api",
		},
	}
	mux := http.NewServeMux()
	registerASProxy(mux, "http://localhost:8080", cfg)

	req := httptest.NewRequest(http.MethodGet, "/authorize?client_id=abc&scope=openid", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}

	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header")
	}
	// Should redirect to upstream /authorize with audience injected.
	if !contains(loc, "audience=my-api") {
		t.Errorf("expected audience in redirect, got %q", loc)
	}
	if !contains(loc, "offline_access") {
		t.Errorf("expected offline_access scope, got %q", loc)
	}
}

func TestRegisterASProxy_Authorize_ScopeAlreadyHasOfflineAccess(t *testing.T) {
	cfg := &OAuthConfig{
		AuthorizationServerURL: "https://auth.example.com",
		ASProxy: &ASProxyConfig{
			Audience: "my-api",
		},
	}
	mux := http.NewServeMux()
	registerASProxy(mux, "http://localhost:8080", cfg)

	req := httptest.NewRequest(http.MethodGet, "/authorize?client_id=abc&scope=openid+offline_access", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
}

func TestRegisterASProxy_Authorize_CustomUpstreamURL(t *testing.T) {
	cfg := &OAuthConfig{
		AuthorizationServerURL: "https://auth.example.com",
		ASProxy: &ASProxyConfig{
			Audience:             "my-api",
			UpstreamAuthorizeURL: "https://custom.example.com/auth",
		},
	}
	mux := http.NewServeMux()
	registerASProxy(mux, "http://localhost:8080", cfg)

	req := httptest.NewRequest(http.MethodGet, "/authorize?client_id=abc", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	loc := rr.Header().Get("Location")
	if !contains(loc, "custom.example.com/auth") {
		t.Errorf("expected custom upstream URL, got %q", loc)
	}
}

func TestRegisterASProxy_Token(t *testing.T) {
	// Start a fake upstream token endpoint.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"Bearer"}`))
	}))
	defer upstream.Close()

	cfg := &OAuthConfig{
		AuthorizationServerURL: "https://auth.example.com",
		ASProxy: &ASProxyConfig{
			Audience:         "my-api",
			UpstreamTokenURL: upstream.URL,
			ClientSecret:     "secret123",
		},
	}
	mux := http.NewServeMux()
	registerASProxy(mux, "http://localhost:8080", cfg)

	req := httptest.NewRequest(http.MethodPost, "/token", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Body = io.NopCloser(io.Reader(httptest.NewRequest(http.MethodPost, "/", nil).Body))
	// Use a proper form body.
	form := "grant_type=authorization_code&code=abc123&client_id=my-client"
	req = httptest.NewRequest(http.MethodPost, "/token", io.NopCloser(io.Reader(
		&stringReader{s: form},
	)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp["access_token"] != "tok" {
		t.Errorf("expected access_token=tok, got %v", resp["access_token"])
	}
}

func TestRegisterASProxy_Token_MethodNotAllowed(t *testing.T) {
	cfg := &OAuthConfig{
		AuthorizationServerURL: "https://auth.example.com",
		ASProxy: &ASProxyConfig{
			Audience: "my-api",
		},
	}
	mux := http.NewServeMux()
	registerASProxy(mux, "http://localhost:8080", cfg)

	req := httptest.NewRequest(http.MethodGet, "/token", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET /token, got %d", rr.Code)
	}
}

func TestRegisterASProxy_Token_UpstreamError(t *testing.T) {
	// Upstream returns 500.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server_error"}`))
	}))
	defer upstream.Close()

	cfg := &OAuthConfig{
		AuthorizationServerURL: "https://auth.example.com",
		ASProxy: &ASProxyConfig{
			Audience:         "my-api",
			UpstreamTokenURL: upstream.URL,
		},
	}
	mux := http.NewServeMux()
	registerASProxy(mux, "http://localhost:8080", cfg)

	form := "grant_type=authorization_code&code=abc"
	req := httptest.NewRequest(http.MethodPost, "/token", io.NopCloser(io.Reader(
		&stringReader{s: form},
	)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// stringReader implements io.Reader for a string.
type stringReader struct {
	s string
	i int
}

func (r *stringReader) Read(p []byte) (n int, err error) {
	if r.i >= len(r.s) {
		return 0, io.EOF
	}
	n = copy(p, r.s[r.i:])
	r.i += n
	return
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
