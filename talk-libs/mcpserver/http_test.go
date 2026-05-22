package mcpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	req.RemoteAddr = "10.0.0.1:1234"

	got := clientIP(req)
	if got != "203.0.113.50" {
		t.Errorf("expected 203.0.113.50, got %q", got)
	}
}

func TestClientIP_XForwardedForMultiple(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18, 150.172.238.178")
	req.RemoteAddr = "10.0.0.1:1234"

	got := clientIP(req)
	if got != "203.0.113.50" {
		t.Errorf("expected first IP 203.0.113.50, got %q", got)
	}
}

func TestClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-Ip", "198.51.100.42")
	req.RemoteAddr = "10.0.0.1:1234"

	got := clientIP(req)
	if got != "198.51.100.42" {
		t.Errorf("expected 198.51.100.42, got %q", got)
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:5555"

	got := clientIP(req)
	if got != "192.168.1.100:5555" {
		t.Errorf("expected RemoteAddr 192.168.1.100:5555, got %q", got)
	}
}

func TestSecurityConfig_Default(t *testing.T) {
	app := &App{}
	cfg := app.securityConfig()

	if cfg.RateLimit != 50 {
		t.Errorf("expected RateLimit 50, got %d", cfg.RateLimit)
	}
	if cfg.RateBurst != 50 {
		t.Errorf("expected RateBurst 50, got %d", cfg.RateBurst)
	}
	if cfg.ReadTimeout != 10*time.Second {
		t.Errorf("expected ReadTimeout 10s, got %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 30*time.Second {
		t.Errorf("expected WriteTimeout 30s, got %v", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 60*time.Second {
		t.Errorf("expected IdleTimeout 60s, got %v", cfg.IdleTimeout)
	}
}

func TestSecurityConfig_Custom(t *testing.T) {
	custom := &HTTPSecurityConfig{
		RateLimit:    100,
		RateBurst:    200,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	app := &App{security: custom}
	cfg := app.securityConfig()

	if cfg.RateLimit != 100 {
		t.Errorf("expected RateLimit 100, got %d", cfg.RateLimit)
	}
	if cfg.RateBurst != 200 {
		t.Errorf("expected RateBurst 200, got %d", cfg.RateBurst)
	}
	if cfg.ReadTimeout != 5*time.Second {
		t.Errorf("expected ReadTimeout 5s, got %v", cfg.ReadTimeout)
	}
}

func TestStatusRecorder_CapturesCode(t *testing.T) {
	rr := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rr}

	sr.WriteHeader(http.StatusNotFound)

	if sr.status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", sr.status)
	}
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected underlying recorder status 404, got %d", rr.Code)
	}
}

func TestStatusRecorder_ImplicitOK(t *testing.T) {
	rr := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rr}

	_, _ = sr.Write([]byte("hello"))

	if sr.status != http.StatusOK {
		t.Errorf("expected implicit status 200, got %d", sr.status)
	}
}

func TestRequestLoggerMiddleware_PassesThrough(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	handler := requestLoggerMiddleware(mux)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Errorf("expected body %q, got %q", "ok", rr.Body.String())
	}
}

func TestRequestLoggerMiddleware_PreservesBody(t *testing.T) {
	// Verify that the request body is still available to the handler after
	// the logger reads it to extract the RPC method.
	mux := http.NewServeMux()
	var gotBody string
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	})

	handler := requestLoggerMiddleware(mux)
	body := `{"jsonrpc":"2.0","method":"tools/list","id":1}`
	req := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if gotBody != body {
		t.Errorf("expected handler to receive original body\n got: %q\nwant: %q", gotBody, body)
	}
}
