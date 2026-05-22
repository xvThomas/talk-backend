package mcpserver

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRestrictPathMiddleware_AllowedPath(t *testing.T) {
	allowed := map[string]bool{"/mcp": true, "/sse": true}
	handler := restrictPathMiddleware(allowed)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for allowed path, got %d", rr.Code)
	}
}

func TestRestrictPathMiddleware_RejectedPath(t *testing.T) {
	allowed := map[string]bool{"/mcp": true, "/sse": true}
	handler := restrictPathMiddleware(allowed)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	paths := []string{"/.env", "/wp-config.php", "/.git/config", "/docker-compose.yml"}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("path %q: expected 404, got %d", path, rr.Code)
		}
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
		"Content-Security-Policy":   "default-src 'none'",
		"Referrer-Policy":           "no-referrer",
		"Permissions-Policy":        "geolocation=(), microphone=(), camera=()",
	}

	for header, want := range expectedHeaders {
		got := rr.Header().Get(header)
		if got != want {
			t.Errorf("header %q: want %q, got %q", header, want, got)
		}
	}
}

func TestRateLimitMiddleware_AllowsBurst(t *testing.T) {
	limiter := newIPRateLimiter(10, 10)
	handler := rateLimitMiddleware(limiter, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := range 10 {
		req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}
}

func TestRateLimitMiddleware_Returns429AfterBurst(t *testing.T) {
	limiter := newIPRateLimiter(5, 5)
	handler := rateLimitMiddleware(limiter, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust the burst.
	for range 5 {
		req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		req.RemoteAddr = "10.0.0.1:5000"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatal("expected 200 within burst")
		}
	}

	// Next request should be rate limited.
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.RemoteAddr = "10.0.0.1:5000"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after burst, got %d", rr.Code)
	}
}

func TestIPRateLimiter_Cleanup(t *testing.T) {
	limiter := &ipRateLimiter{
		entries: make(map[string]*ipEntry),
		rps:     10,
		burst:   10,
		ttl:     1 * time.Millisecond,
	}

	// Add an entry.
	limiter.get("192.168.1.1")

	// Wait for it to become stale.
	time.Sleep(5 * time.Millisecond)
	limiter.cleanup()

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if len(limiter.entries) != 0 {
		t.Errorf("expected 0 entries after cleanup, got %d", len(limiter.entries))
	}
}

func TestTrustedClientIP_EmptyProxies(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	req.RemoteAddr = "10.0.0.1:1234"

	ip := trustedClientIP(req, nil)
	if ip != "203.0.113.50" {
		t.Errorf("expected X-Forwarded-For IP with empty proxies, got %q", ip)
	}
}

func TestTrustedClientIP_UntrustedProxy(t *testing.T) {
	_, cidr, _ := net.ParseCIDR("172.17.0.1/32")
	trusted := []net.IPNet{*cidr}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	req.RemoteAddr = "192.168.1.100:1234"

	ip := trustedClientIP(req, trusted)
	if ip != "192.168.1.100" {
		t.Errorf("expected RemoteAddr when proxy untrusted, got %q", ip)
	}
}

func TestTrustedClientIP_TrustedProxy(t *testing.T) {
	_, cidr, _ := net.ParseCIDR("172.17.0.0/16")
	trusted := []net.IPNet{*cidr}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	req.RemoteAddr = "172.17.0.5:1234"

	ip := trustedClientIP(req, trusted)
	if ip != "203.0.113.50" {
		t.Errorf("expected X-Forwarded-For IP from trusted proxy, got %q", ip)
	}
}

func TestBuildAllowedPaths_WithoutOAuth(t *testing.T) {
	paths := buildAllowedPaths(false)
	if !paths["/mcp"] || !paths["/sse"] {
		t.Error("expected /mcp and /sse to be allowed")
	}
	if paths["/authorize"] {
		t.Error("expected /authorize to be disallowed without OAuth")
	}
}

func TestBuildAllowedPaths_WithOAuth(t *testing.T) {
	paths := buildAllowedPaths(true)
	expected := []string{"/mcp", "/sse", "/.well-known/oauth-protected-resource", "/authorize", "/token"}
	for _, p := range expected {
		if !paths[p] {
			t.Errorf("expected %q to be allowed with OAuth", p)
		}
	}
}

func TestParseTrustedProxies(t *testing.T) {
	tests := []struct {
		input string
		count int
	}{
		{"", 0},
		{"172.17.0.1", 1},
		{"172.17.0.1, 10.0.0.0/8", 2},
		{"invalid", 0},
		{"172.17.0.1, invalid, 10.0.0.1", 2},
	}

	for _, tt := range tests {
		nets := parseTrustedProxies(tt.input)
		if len(nets) != tt.count {
			t.Errorf("parseTrustedProxies(%q): expected %d nets, got %d", tt.input, tt.count, len(nets))
		}
	}
}
