---
description: "Guidelines for creating a new MCP server using the pkg/mcpserver framework"
applyTo: "cmd/mcp/**"
---

# MCP Server Creation Guidelines

This document describes how to create a new MCP server in this monorepo using the shared `pkg/mcpserver` framework. The reference implementation is `cmd/mcp/go-sdk-server`.

## Architecture Overview

```
pkg/mcpserver/          Shared framework (App, ToolRegistrar, BaseEnv)
internal/domain/        TypedTool interface (business logic)
internal/infrastructure/tools/<toolname>/   Tool implementations
cmd/mcp/<server-name>/  Server entry point (main.go, config.go, Makefile, Dockerfile, .air.toml)
```

## Step-by-Step: Creating a New MCP Server

### 1. Create the server directory

```
cmd/mcp/<server-name>/
```

### 2. Implement your tools

Each tool must implement `domain.TypedTool[TInput, TOutput]` in `internal/infrastructure/tools/<toolname>/`:

```go
package mytool

import (
    "context"
    "talks/internal/domain"
)

type MyToolInput struct {
    Param string `json:"param"`
}

type MyToolOutput struct {
    Result string `json:"result"`
}

type MyTool struct {
    apiKey string
}

var _ domain.TypedTool[MyToolInput, MyToolOutput] = (*MyTool)(nil)

func NewMyTool(apiKey string) *MyTool {
    return &MyTool{apiKey: apiKey}
}

func (t *MyTool) Name() string        { return "my_tool" }
func (t *MyTool) Description() string  { return "Does something useful" }

func (t *MyTool) Call(ctx context.Context, input MyToolInput) (MyToolOutput, error) {
    // implementation
    return MyToolOutput{Result: "ok"}, nil
}
```

### 3. Create `config.go`

Compose `mcpserver.BaseEnv` (provides `APIKey` / `X_API_KEY` and OAuth env vars) with your server-specific environment variables:

```go
package main

import (
    "fmt"
    "os"

    "github.com/joho/godotenv"
    "talks/pkg/mcpserver"
)

type serverEnv struct {
    mcpserver.BaseEnv
    MyServiceAPIKey string // MY_SERVICE_API_KEY
}

func loadServerEnv(envFiles ...string) (*serverEnv, error) {
    _ = godotenv.Load(envFiles...)

    env := &serverEnv{
        BaseEnv:         mcpserver.LoadBaseEnv(),
        MyServiceAPIKey: os.Getenv("MY_SERVICE_API_KEY"),
    }

    if env.MyServiceAPIKey == "" {
        return nil, fmt.Errorf("missing required environment variable %q", "MY_SERVICE_API_KEY")
    }

    return env, nil
}
```

Key rules:
- Always embed `mcpserver.BaseEnv` — it handles `X_API_KEY`, `OAUTH_AUTHORIZATION_SERVER`, and `OAUTH_SCOPES`.
- Call `mcpserver.LoadBaseEnv()` after `godotenv.Load`.
- Validate only your server-specific variables; auth configuration is handled by the framework at runtime.

### 4. Create `main.go`

Wire tools and start the app using the functional options pattern:

```go
package main

import (
    "os"

    "talks/internal/infrastructure/tools/mytool"
    "talks/pkg/logger"
    "talks/pkg/mcpserver"
)

func main() {
    log := logger.GetLogger()

    env, err := loadServerEnv(".env")
    if err != nil {
        log.Error("failed to load config", "error", err)
        os.Exit(1)
    }

    tool := mytool.NewMyTool(env.MyServiceAPIKey)

    opts := []mcpserver.Option{
        mcpserver.WithTools(mcpserver.RegisterTool(tool)),
    }
    if env.APIKey != "" {
        opts = append(opts, mcpserver.WithAPIKey(env.APIKey))
    }
    if env.OAuthAuthorizationServer != "" {
        opts = append(opts, mcpserver.WithOAuth(&mcpserver.OAuthConfig{
            AuthorizationServerURL: env.OAuthAuthorizationServer,
            Scopes:                 env.OAuthScopesList(),
            TokenVerifier: mcpserver.NewJWKSTokenVerifier(mcpserver.JWKSVerifierConfig{
                IssuerURL: env.OAuthAuthorizationServer,
            }),
        }))
    }

    app := mcpserver.NewApp("<server-name>-mcp", "1.0.0", opts...)
    app.Run()
}
```

Key rules:
- `main.go` must stay thin: load config, create tools, wire app, run.
- No business logic in `main.go`.
- Use `mcpserver.NewApp` constructor with functional options (`WithAPIKey`, `WithOAuth`, `WithTools`).
- Auth options are conditional: only add `WithAPIKey` / `WithOAuth` when the env vars are set.
- Multiple tools can be registered by adding entries to `WithTools()`.

### 5. Create `Makefile`

Copy from `cmd/mcp/go-sdk-server/Makefile` and change only `APP_NAME`:

```makefile
APP_NAME     := <server-name>
```

The Makefile provides: `all`, `build`, `run`, `dev`, `lint`, `clean`, `dockerize`, `help`.

### 6. Create `.air.toml`

Copy from `cmd/mcp/go-sdk-server/.air.toml`. No changes needed (it uses `main.exe` as binary name).

### 7. Create `Dockerfile`

Copy from `cmd/mcp/go-sdk-server/Dockerfile` and update the binary name:

```dockerfile
FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/<server-name> ./cmd/mcp/<server-name>

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/<server-name> /usr/local/bin/<server-name>

EXPOSE 8080

ENTRYPOINT ["<server-name>", "--transport", "http", "--addr", "0.0.0.0:8080"]
```

### 8. Create a `.env` file (local dev only, never commit)

```env
X_API_KEY=your-dev-secret
MY_SERVICE_API_KEY=your-service-key

# Optional OAuth configuration
# OAUTH_AUTHORIZATION_SERVER=https://my-tenant.auth0.com
# OAUTH_SCOPES=mcp:read,mcp:write
```

## Framework Reference

### `pkg/mcpserver.NewApp`

```go
func NewApp(name, version string, opts ...Option) *App
```

Creates an `App` configured with functional options.

### Options

| Option                   | Description                                            |
|--------------------------|--------------------------------------------------------|
| `WithAPIKey(key)`        | Enables X-API-Key header auth for HTTP transport       |
| `WithOAuth(cfg)`         | Enables OAuth 2.0 Bearer token auth for HTTP transport |
| `WithTools(tools...)`    | Registers tools on the MCP server                      |

### `pkg/mcpserver.OAuthConfig`

| Field                    | Type               | Description                                              |
|--------------------------|--------------------|----------------------------------------------------------|
| `AuthorizationServerURL` | `string`           | Issuer URL of the external Authorization Server          |
| `Scopes`                 | `[]string`         | OAuth scopes required to access MCP endpoints            |
| `TokenVerifier`          | `auth.TokenVerifier` | Validates Bearer tokens (JWKS, introspection, etc.)    |

### `pkg/mcpserver.NewJWKSTokenVerifier`

```go
func NewJWKSTokenVerifier(cfg JWKSVerifierConfig) auth.TokenVerifier
```

Built-in JWKS verifier that fetches public keys from `{IssuerURL}/.well-known/jwks.json`, caches them (default 1h), and validates JWT signature (RS256/384/512), `exp`, `iss`, and optionally `aud`.

| Field        | Type            | Description                                         |
|-------------|-----------------|-----------------------------------------------------|
| `IssuerURL` | `string`        | Base URL of the AS (e.g. `https://tenant.auth0.com`)|
| `Audience`  | `string`        | Expected `aud` claim (optional)                     |
| `HTTPClient`| `*http.Client`  | Custom HTTP client (optional, default 10s timeout)  |
| `CacheTTL`  | `time.Duration` | JWKS cache duration (optional, default 1h)          |

### Authentication Modes

The framework supports four modes based on configuration:
- **None** — no auth options set → warning logged, server is not secured
- **API Key only** — `WithAPIKey` set → `X-API-Key` header checked
- **OAuth only** — `WithOAuth` set → Bearer token validated + metadata endpoint registered
- **Both** — both set → dispatches based on `Authorization: Bearer` header presence

When OAuth is enabled, the server registers `/.well-known/oauth-protected-resource` (RFC 9728) so OAuth-aware clients can discover the Authorization Server.

`App.Run()` handles:
- CLI flags: `--transport stdio|http`, `--addr localhost:8080`
- Stdio transport (for Claude Desktop, VS Code subprocess)
- HTTP transport with SSE (`/sse`) and Streamable HTTP (`/mcp`) endpoints
- Authentication middleware (API Key, OAuth, or both)

### `pkg/mcpserver.RegisterTool`

```go
func RegisterTool[TInput, TOutput any](tool domain.TypedTool[TInput, TOutput]) ToolRegistrar
```

Wraps any `domain.TypedTool` into a `ToolRegistrar` for the MCP SDK.

### `pkg/mcpserver.BaseEnv`

Embed in your `serverEnv` struct. Provides:
- `APIKey` — from `X_API_KEY` env var
- `OAuthAuthorizationServer` — from `OAUTH_AUTHORIZATION_SERVER` env var
- `OAuthScopes` — from `OAUTH_SCOPES` env var (comma-separated)
- `OAuthScopesList()` — helper that returns scopes as `[]string`

Loaded via `LoadBaseEnv()`.

## Checklist for a New Server

- [ ] Tool(s) in `internal/infrastructure/tools/<name>/` implementing `domain.TypedTool`
- [ ] `cmd/mcp/<server-name>/main.go` — thin wiring
- [ ] `cmd/mcp/<server-name>/config.go` — env loading with `mcpserver.BaseEnv` embedding
- [ ] `cmd/mcp/<server-name>/Makefile` — copy, change `APP_NAME`
- [ ] `cmd/mcp/<server-name>/.air.toml` — copy as-is
- [ ] `cmd/mcp/<server-name>/Dockerfile` — copy, change binary name
- [ ] `.env` for local development
- [ ] `go build ./...` passes
- [ ] `make run` starts successfully
