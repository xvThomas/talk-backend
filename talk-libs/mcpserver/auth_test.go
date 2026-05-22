package mcpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
