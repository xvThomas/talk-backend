---
description: "Guidelines for creating a new MCP server using the talk-libs/mcpserver framework"
applyTo: "mcp-*/**"
---

# MCP Server Creation Guidelines

This document describes how to create a new MCP server in this monorepo using the shared `talk-libs/mcpserver` framework. The reference implementation is `mcp-playground/`.

## Architecture Overview

```
talk-libs/mcpserver/       Shared framework (App, ToolRegistrar, BaseEnv)
talk-libs/domain/          TypedTool interface (business logic)
mcp-<name>/                New server module (its own go.mod)
  cmd/main.go              Entry point
  internal/config/         Server-specific configuration
  internal/tools/          Tool implementations (private to this server)
```

## Step-by-Step: Creating a New MCP Server

### 1. Create the module directory

```
mcp-<server-name>/
├── go.mod
├── Makefile
├── Dockerfile
├── cmd/
│   └── main.go
└── internal/
    ├── config/
    │   └── config.go
    └── tools/
        └── my_tool.go
```

### 2. Create `go.mod`

```go
module talks/mcp-<server-name>

go 1.25.0

require (
    talks/talk-libs v0.0.0
    github.com/joho/godotenv v1.5.1
)
```

Then add it to `go.work`:
```go
use (
    ./talk-libs
    ./talk
    ./mcp-owm
    ./mcp-playground
    ./mcp-<server-name>   // ← add
)
```

### 3. Implement your tools

Each tool must implement `domain.TypedTool[TInput, TOutput]` in `internal/tools/`:

```go
package tools

import (
    "context"
    "talks/talk-libs/domain"
)

type MyToolInput struct {
    Param string `json:"param" description:"Parameter description"`
}

type MyToolOutput struct {
    Result string `json:"result" description:"Result description"`
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
    return MyToolOutput{Result: "ok"}, nil
}
```

### 4. Create `internal/config/config.go`

Compose `mcpserver.BaseEnv` with your server-specific environment variables:

```go
package config

import (
    "fmt"
    "os"

    "github.com/joho/godotenv"
    "talks/talk-libs/mcpserver"
)

type ServerEnv struct {
    mcpserver.BaseEnv
    MyServiceAPIKey string
}

func LoadServerEnv(envFiles ...string) (*ServerEnv, error) {
    _ = godotenv.Load(envFiles...)

    env := &ServerEnv{
        BaseEnv:         mcpserver.LoadBaseEnv(),
        MyServiceAPIKey: os.Getenv("MY_SERVICE_API_KEY"),
    }

    if env.MyServiceAPIKey == "" {
        return nil, fmt.Errorf("missing required environment variable %q", "MY_SERVICE_API_KEY")
    }

    return env, nil
}
```

### 5. Create `cmd/main.go`

```go
package main

import (
    "os"

    "talks/mcp-<server-name>/internal/config"
    "talks/mcp-<server-name>/internal/tools"
    "talks/talk-libs/logger"
    "talks/talk-libs/mcpserver"
)

func main() {
    log := logger.GetLogger()

    env, err := config.LoadServerEnv(".env")
    if err != nil {
        log.Error("failed to load config", "error", err)
        os.Exit(1)
    }

    tool := tools.NewMyTool(env.MyServiceAPIKey)

    opts := []mcpserver.Option{
        mcpserver.WithTools(mcpserver.RegisterTool(tool)),
    }
    if env.APIKey != "" {
        opts = append(opts, mcpserver.WithAPIKey(env.APIKey))
    }
    if env.OAuthAuthorizationServer != "" {
        opts = append(opts, mcpserver.WithOAuth(&mcpserver.OAuthConfig{
            AuthorizationServerURL: env.OAuthAuthorizationServer,
            ResourceBaseURL:        env.BaseURL,
            Scopes:                 env.OAuthScopesList(),
            TokenVerifier: mcpserver.NewJWKSTokenVerifier(mcpserver.JWKSVerifierConfig{
                IssuerURL: env.OAuthAuthorizationServer,
                Audience:  env.OAuthAudience,
            }),
        }))
    }

    app := mcpserver.NewApp("<server-name>-mcp", "1.0.0", opts...)
    app.Run()
}
```

### 6. Create `Makefile`

Copy from `mcp-playground/Makefile` and change `APP_NAME`:

```makefile
APP_NAME     := mcp-<server-name>
```

### 7. Create `Dockerfile`

Copy from `mcp-playground/Dockerfile` and adjust the module name.

## Key Rules

- **No cross-dependencies between MCP servers** — each server is fully independent.
- **Tools are internal** to each server — they are NOT importable by other modules.
- `main.go` must stay thin: load config, create tools, wire app, run.
- Auth options are conditional: only add `WithAPIKey` / `WithOAuth` when the env vars are set.

### 8. Create prompts (optional)

Prompts give LLM clients pre-defined instructions on how to use your tools. Create `internal/prompts/prompts.go`:

```go
package prompts

import "github.com/xvThomas/LLMClientWrapper/talk-libs/mcpserver"

var MyPrompt = mcpserver.Prompt{
    Name:        "my_prompt",
    Description: "Explain what this prompt does",
    Arguments: []mcpserver.PromptArgument{
        {Name: "param", Description: "What this parameter is for", Required: true},
    },
    Messages: []mcpserver.PromptMessage{
        {
            Role: "user",
            Text: "Use the my_tool tool with {{param}}. Present the result clearly.",
        },
    },
}
```

Register prompts in `main.go` alongside tools:

```go
opts := []mcpserver.Option{
    mcpserver.WithTools(mcpserver.RegisterTool(myTool)),
    mcpserver.WithPrompts(mcpserver.RegisterPrompt(prompts.MyPrompt)),
}
```

- Use `{{argName}}` placeholders in message text — they are replaced with argument values at runtime.
- A prompt can have multiple messages (e.g. user + assistant for few-shot examples).

### 9. Hot reload with Air

For rapid development, use [Air](https://github.com/air-verse/air) for hot reload. Create `.air.toml` at the server root:

```toml
root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/mcp-<server-name>.exe ./cmd"
bin = "./tmp/mcp-<server-name>.exe"
args_bin = ["--transport", "http", "--addr", "localhost:8080"]
include_ext = ["go", "toml", "env"]
include_dir = ["cmd", "internal"]
exclude_dir = ["tmp", "bin"]
delay = 1000

[log]
time = false

[misc]
clean_on_exit = true
```

Add the `dev` target to your `Makefile`:

```makefile
dev: ## Run the server with hot reload (air) in http mode
	air
```

Add `tmp/` to `.gitignore`. Install Air with:

```bash
go install github.com/air-verse/air@latest
```
