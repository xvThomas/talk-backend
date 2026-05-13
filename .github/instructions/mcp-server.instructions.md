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

Compose `mcpserver.BaseEnv` (provides `APIKey` / `X_API_KEY`) with your server-specific environment variables:

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
- Always embed `mcpserver.BaseEnv` — it handles `X_API_KEY` for HTTP auth.
- Call `mcpserver.LoadBaseEnv()` after `godotenv.Load`.
- Validate only your server-specific variables; `APIKey` is validated by `mcpserver.App` at runtime.

### 4. Create `main.go`

Wire tools and start the app:

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

    app := &mcpserver.App{
        Name:    "<server-name>-mcp",
        Version: "1.0.0",
        APIKey:  env.APIKey,
        Tools: []mcpserver.ToolRegistrar{
            mcpserver.RegisterTool(tool),
        },
    }
    app.Run()
}
```

Key rules:
- `main.go` must stay thin: load config, create tools, wire app, run.
- No business logic in `main.go`.
- Multiple tools can be registered by adding entries to the `Tools` slice.

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
```

## Framework Reference

### `pkg/mcpserver.App`

| Field     | Type               | Description                                      |
|-----------|--------------------|--------------------------------------------------|
| `Name`    | `string`           | Server name (used in MCP implementation info)    |
| `Version` | `string`           | Server version                                   |
| `Tools`   | `[]ToolRegistrar`  | List of tool registrars                          |
| `APIKey`  | `string`           | Shared secret for HTTP auth (from `BaseEnv`)     |

`App.Run()` handles:
- CLI flags: `--transport stdio|http`, `--addr localhost:8080`
- Stdio transport (for Claude Desktop, VS Code subprocess)
- HTTP transport with SSE (`/sse`) and Streamable HTTP (`/mcp`) endpoints
- `X-API-Key` header authentication middleware

### `pkg/mcpserver.RegisterTool`

```go
func RegisterTool[TInput, TOutput any](tool domain.TypedTool[TInput, TOutput]) ToolRegistrar
```

Wraps any `domain.TypedTool` into a `ToolRegistrar` for the MCP SDK.

### `pkg/mcpserver.BaseEnv`

Embed in your `serverEnv` struct. Provides `APIKey` field loaded from `X_API_KEY` env var via `LoadBaseEnv()`.

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
