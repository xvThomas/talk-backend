package mcpserver

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/xvThomas/talk-backend/talk-libs/logger"
	"golang.org/x/time/rate"
)

// HTTPSecurityConfig holds the HTTP-level security settings for the server.
type HTTPSecurityConfig struct {
	RateLimit      int           // Max requests per second per IP.
	RateBurst      int           // Burst size per IP.
	ReadTimeout    time.Duration // HTTP read timeout.
	WriteTimeout   time.Duration // HTTP write timeout.
	IdleTimeout    time.Duration // HTTP idle timeout.
	TrustedProxies []net.IPNet   // Trusted proxy CIDRs; empty means trust all.
}

// WithHTTPSecurity configures the HTTP security layer (rate limiting, timeouts,
// trusted proxies). When omitted, default values are used (50 req/s, 50 burst,
// 10s read, 30s write, 60s idle, trust all proxies).
func WithHTTPSecurity(cfg HTTPSecurityConfig) Option {
	return func(a *App) { a.security = &cfg }
}

// --- Restrictive path filter middleware ---

// restrictPathMiddleware returns a middleware that rejects requests to paths
// not in the allowed set with a 404 and logs the attempt.
func restrictPathMiddleware(allowedPaths map[string]bool) func(http.Handler) http.Handler {
	log := logger.GetLogger()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !allowedPaths[r.URL.Path] {
				log.Warn("unknown path rejected", "path", r.URL.Path, "client_ip", clientIP(r))
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --- Security headers middleware ---

// securityHeadersMiddleware adds standard security headers to every response
// and strips the Server header to avoid fingerprinting.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		h.Set("Content-Security-Policy", "default-src 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		h.Del("Server")
		next.ServeHTTP(w, r)
	})
}

// --- Per-IP rate limiter with LRU eviction ---

// ipEntry holds a rate limiter and its last access time.
type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// ipRateLimiter tracks per-IP rate limiters and periodically evicts stale entries.
type ipRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*ipEntry
	rps     rate.Limit
	burst   int
	ttl     time.Duration
}

// newIPRateLimiter creates a per-IP rate limiter. It starts a background
// goroutine that removes entries inactive for longer than ttl.
func newIPRateLimiter(rps int, burst int) *ipRateLimiter {
	l := &ipRateLimiter{
		entries: make(map[string]*ipEntry),
		rps:     rate.Limit(rps),
		burst:   burst,
		ttl:     10 * time.Minute,
	}
	go l.cleanupLoop()
	return l
}

// get returns the rate limiter for the given IP, creating one if needed.
func (l *ipRateLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[ip]
	if !ok {
		entry = &ipEntry{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.entries[ip] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

// cleanupLoop periodically removes IP entries that have been inactive.
func (l *ipRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.cleanup()
	}
}

// cleanup removes entries older than ttl.
func (l *ipRateLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for ip, entry := range l.entries {
		if now.Sub(entry.lastSeen) > l.ttl {
			delete(l.entries, ip)
		}
	}
}

// rateLimitMiddleware returns a middleware that enforces per-IP rate limiting.
// Requests exceeding the limit receive a 429 Too Many Requests response.
func rateLimitMiddleware(limiter *ipRateLimiter, trustedProxies []net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := trustedClientIP(r, trustedProxies)
			if !limiter.get(ip).Allow() {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --- Trusted proxy aware client IP resolution ---

// trustedClientIP returns the client IP address, taking trusted proxies into
// account. When trustedProxies is empty, it behaves identically to clientIP()
// (trusts X-Forwarded-For from any source). When trustedProxies is non-empty,
// forwarded headers are only read if RemoteAddr matches a trusted proxy.
func trustedClientIP(r *http.Request, trustedProxies []net.IPNet) string {
	if len(trustedProxies) == 0 {
		return clientIP(r)
	}

	remoteIP := extractIP(r.RemoteAddr)
	if !isTrustedProxy(remoteIP, trustedProxies) {
		return remoteIP
	}

	return clientIP(r)
}

// extractIP extracts just the IP portion from an address (strips port).
func extractIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// isTrustedProxy checks whether the given IP belongs to any trusted proxy network.
func isTrustedProxy(ipStr string, trustedProxies []net.IPNet) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	// Normalize to 4-byte form so that Contains works with 4-byte IPNet masks.
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	for _, ipNet := range trustedProxies {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// buildAllowedPaths returns the set of paths that the server should serve.
// Paths not in this set will be rejected by restrictPathMiddleware.
func buildAllowedPaths(hasOAuth bool) map[string]bool {
	paths := map[string]bool{
		"/sse": true,
		"/mcp": true,
	}
	if hasOAuth {
		paths["/.well-known/oauth-protected-resource"] = true
		paths["/.well-known/oauth-authorization-server"] = true
		paths["/authorize"] = true
		paths["/token"] = true
		paths["/register"] = true
	}
	return paths
}

// parseTrustedProxies parses a comma-separated list of IPs and CIDRs into a
// slice of net.IPNet. Single IPs are converted to /32 (IPv4) or /128 (IPv6).
func parseTrustedProxies(raw string) []net.IPNet {
	if raw == "" {
		return nil
	}
	var nets []net.IPNet
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			_, ipNet, err := net.ParseCIDR(entry)
			if err == nil {
				nets = append(nets, *ipNet)
			}
		} else {
			ip := net.ParseIP(entry)
			if ip == nil {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				nets = append(nets, net.IPNet{IP: ip4, Mask: net.CIDRMask(32, 32)})
			} else {
				nets = append(nets, net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)})
			}
		}
	}
	return nets
}
