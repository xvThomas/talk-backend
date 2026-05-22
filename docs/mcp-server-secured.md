# MCP Server Security Hardening

This document describes the HTTP-level security features built into the `talk-libs/mcpserver` framework, protecting all MCP servers (`mcp-owm`, `mcp-ign-nav`, `mcp-playground`) against common threats observed on public-facing deployments.

## Security Strategy

The framework implements **defense in depth** with multiple layers:

1. **Restrictive path filter** — rejects scanning probes early
2. **Security headers** — prevents browser-based attacks and fingerprinting
3. **Per-IP rate limiting** — absorbs automated scanners and brute-force attempts
4. **HTTP timeouts** — protects against Slowloris and resource exhaustion
5. **Trusted proxy awareness** — ensures correct client IP resolution for rate limiting
6. **Authentication** — API Key and/or OAuth 2.0 Bearer token (see [mcp-server-authentication.md](mcp-server-authentication.md))

These features activate automatically in **HTTP transport mode** and have no effect in stdio mode.

## Features

### Restrictive Path Filter

Only explicitly registered paths are served. All other paths receive a `404 Not Found` response and are logged:

```
WARN unknown path rejected  path=/.env  client_ip=203.0.113.50
```

**Allowed paths:**

| Path | Always |
|------|--------|
| `/sse` | ✓ |
| `/mcp` | ✓ |
| `/.well-known/oauth-protected-resource` | OAuth only |
| `/.well-known/oauth-authorization-server` | OAuth only |
| `/authorize` | OAuth only |
| `/token` | OAuth only |
| `/register` | OAuth only |

Any request to paths like `/.env`, `/.git/config`, `/wp-config.php`, `/docker-compose.yml`, etc. is immediately rejected without reaching any business logic.

### Security Headers

Every response includes the following headers to harden the server against common web attacks:

| Header | Value | Purpose |
|--------|-------|---------|
| `X-Content-Type-Options` | `nosniff` | Prevents MIME-type sniffing |
| `X-Frame-Options` | `DENY` | Prevents clickjacking |
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains` | Enforces HTTPS (2 years) |
| `Content-Security-Policy` | `default-src 'none'` | Blocks all external resources |
| `Referrer-Policy` | `no-referrer` | Prevents referrer leakage |
| `Permissions-Policy` | `geolocation=(), microphone=(), camera=()` | Disables browser APIs |

The `Server` header is also removed to prevent fingerprinting.

### Per-IP Rate Limiting

Each client IP is rate-limited independently using a token bucket algorithm:

- **Rate** — maximum sustained requests per second (default: 50)
- **Burst** — maximum burst capacity (default: 50)
- Exceeding the limit returns `429 Too Many Requests`

The rate limiter uses LRU-style eviction: entries inactive for more than 10 minutes are automatically cleaned up every 5 minutes to prevent unbounded memory growth from scanning IPs.

### HTTP Timeouts

The server uses `http.Server` with configurable timeouts to protect against slow clients (Slowloris attacks) and resource exhaustion:

| Timeout | Default | Protection |
|---------|---------|------------|
| Read | 10s | Limits time to read request headers + body |
| Write | 30s | Limits time to write the response |
| Idle | 60s | Closes idle keep-alive connections |

### Trusted Proxies

When deployed behind a reverse proxy (Traefik, Caddy, Nginx), the server needs to know which `X-Forwarded-For` headers to trust for accurate client IP resolution.

**Behavior:**

- `HTTP_TRUSTED_PROXIES` **empty** (default) — trusts `X-Forwarded-For` from any source (backward compatible, suitable when the server is only reachable via the proxy)
- `HTTP_TRUSTED_PROXIES` **set** — only reads `X-Forwarded-For` when the direct connection comes from a listed IP/CIDR; otherwise uses the TCP connection IP

This prevents attackers from spoofing their IP to bypass rate limiting when the server port is accidentally exposed directly.

## Environment Variables

All variables are optional and read from the shared `BaseEnv` (loaded by each server's `config.LoadServerEnv()`):

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_RATE_LIMIT` | `50` | Maximum requests per second per client IP |
| `HTTP_RATE_BURST` | `50` | Maximum burst capacity per client IP |
| `HTTP_READ_TIMEOUT` | `10` | HTTP read timeout in seconds |
| `HTTP_WRITE_TIMEOUT` | `30` | HTTP write timeout in seconds |
| `HTTP_IDLE_TIMEOUT` | `60` | HTTP idle connection timeout in seconds |
| `HTTP_TRUSTED_PROXIES` | *(empty)* | Comma-separated trusted proxy IPs or CIDRs (e.g. `172.17.0.1,10.0.0.0/8`) |

### Example `.env`

```env
# HTTP Security (all optional, shown with defaults)
HTTP_RATE_LIMIT=50
HTTP_RATE_BURST=50
HTTP_READ_TIMEOUT=10
HTTP_WRITE_TIMEOUT=30
HTTP_IDLE_TIMEOUT=60

# Set when you know your reverse proxy IP (recommended for production)
# HTTP_TRUSTED_PROXIES=172.17.0.1
```

## Architecture

```
Incoming request
    │
    ▼
┌─────────────────────┐
│  Rate Limit (per-IP)│ → 429 Too Many Requests
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Security Headers   │ → adds protective headers
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Path Filter        │ → 404 for unknown paths
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Request Logger     │ → logs method, path, IP, UA
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Auth Middleware    │ → 401 Unauthorized
│  (API Key / OAuth)  │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  MCP Handler        │ → business logic
│  (/sse or /mcp)     │
└─────────────────────┘
```

## Deployment Recommendations

1. **Always set authentication** — configure at least `X_API_KEY` or OAuth for production deployments
2. **Set `HTTP_TRUSTED_PROXIES`** when you know your reverse proxy IP — this prevents IP spoofing via `X-Forwarded-For`
3. **Lower rate limits** for sensitive endpoints if needed — the current implementation applies the same limit to all paths
4. **Monitor logs** for `unknown path rejected` warnings — high volumes indicate active scanning
5. **Keep defaults** unless you have a specific reason to change them — they are tuned for typical MCP server workloads

## Related Documentation

- [Authentication](mcp-server-authentication.md) — API Key and OAuth 2.0 configuration
