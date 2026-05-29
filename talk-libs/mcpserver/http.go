package mcpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xvThomas/LLMClientWrapper/talk-libs/logger"
)

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

	handler := a.buildHTTPHandler(addr, mux)

	// Resolve security settings for server timeouts.
	sec := a.securityConfig()

	log.Info("HTTP server listening", "addr", addr, "sse", "/sse", "streamable", "/mcp")
	log.Info("HTTP security", "rate_limit", sec.RateLimit, "rate_burst", sec.RateBurst,
		"read_timeout", sec.ReadTimeout, "write_timeout", sec.WriteTimeout,
		"idle_timeout", sec.IdleTimeout, "trusted_proxies", len(sec.TrustedProxies))

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  sec.ReadTimeout,
		WriteTimeout: sec.WriteTimeout,
		IdleTimeout:  sec.IdleTimeout,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}

// buildHTTPHandler constructs the full middleware-wrapped HTTP handler chain.
// Separated from runHTTP for testability.
func (a *App) buildHTTPHandler(_ string, mux *http.ServeMux) http.Handler {
	sec := a.securityConfig()

	// Build the allowed-path set for the restrictive path filter.
	allowedPaths := buildAllowedPaths(a.oauth != nil)

	// Wrap the mux with a response-capturing request logger for debugging.
	handler := requestLoggerMiddleware(mux)

	// Apply security middleware chain (outermost first):
	// rate limit -> security headers -> path filter -> handler
	handler = restrictPathMiddleware(allowedPaths)(handler)
	handler = securityHeadersMiddleware(handler)
	ipLimiter := newIPRateLimiter(sec.RateLimit, sec.RateBurst)
	handler = rateLimitMiddleware(ipLimiter, sec.TrustedProxies)(handler)

	return handler
}

// securityConfig returns the effective HTTPSecurityConfig, using defaults for
// any unset values.
func (a *App) securityConfig() HTTPSecurityConfig {
	if a.security != nil {
		return *a.security
	}
	return HTTPSecurityConfig{
		RateLimit:    50,
		RateBurst:    50,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// requestLoggerMiddleware wraps a mux with response-capturing request logging
// for debugging. It extracts the JSON-RPC method from POST bodies.
func requestLoggerMiddleware(mux *http.ServeMux) http.Handler {
	log := logger.GetLogger()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			"client_ip", clientIP(r),
			"host", r.Host,
			"user_agent", r.Header.Get("User-Agent"),
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
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
// It also implements http.Flusher so that SSE streams can flush properly.
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

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// clientIP returns the best-effort client IP address from the request,
// checking proxy headers before falling back to RemoteAddr.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if i := strings.IndexByte(fwd, ','); i > 0 {
			return strings.TrimSpace(fwd[:i])
		}
		return fwd
	}
	if ip := r.Header.Get("X-Real-Ip"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}
