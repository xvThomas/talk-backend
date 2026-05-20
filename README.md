# LLMClientWrapper

[![CI](https://github.com/xvThomas/LLMClientWrapper/actions/workflows/ci.yml/badge.svg)](https://github.com/xvThomas/LLMClientWrapper/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/xvThomas/LLMClientWrapper/branch/main/graph/badge.svg)](https://codecov.io/gh/xvThomas/LLMClientWrapper)

A Go monorepo providing a CLI that routes questions to Anthropic or OpenAI-compatible models (GPT, Mistral, Devstral…) through a unified interface, plus standalone MCP (Model Context Protocol) tool servers.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                          go.work (root)                             │
├────────────┬────────────┬─────────────────┬─────────────────────────┤
│ talk-libs  │    talk    │    mcp-owm      │    mcp-playground       │
│ (shared)   │   (CLI)    │ (weather MCP)   │   (empty template)      │
├────────────┼────────────┼─────────────────┼─────────────────────────┤
│ domain/    │ cmd/cli/   │ cmd/            │ cmd/                    │
│ logger/    │ internal/  │ internal/       │ internal/               │
│ mcpserver/ │            │   config/       │   config/               │
│ version/   │            │   tools/        │                         │
└────────────┴────────────┴─────────────────┴─────────────────────────┘
```

The project uses **Go workspaces** (`go.work`) with four independent modules:

| Module | Path | Description |
|--------|------|-------------|
| `talk-libs` | `./talk-libs` | Shared library: domain types (`TypedTool`), logger, MCP server framework, version |
| `talk` | `./talk` | Interactive CLI — multi-turn conversations with LLM providers |
| `mcp-owm` | `./mcp-owm` | MCP server exposing OpenWeatherMap tools |
| `mcp-playground` | `./mcp-playground` | Empty MCP server template for experimentation |

---

## Prerequisites

- Go 1.25+
- `make`
- `golangci-lint` v2.12+ (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`)
- API keys for the providers you want to use (see [Environment variables](#environment-variables))

---

## Quickstart

```bash
# 1. Clone
git clone https://github.com/xvThomas/LLMClientWrapper.git
cd LLMClientWrapper

# 2. Copy and fill in your API keys
cp .env.example .env
$EDITOR .env

# 3. Build all binaries
make build

# 4. Start an interactive CLI session
cd talk && make run MODEL=sonnet-4.6

# 5. Start the OpenWeather MCP server (hot-reload)
cd mcp-owm && make dev
```

---

## Available models

| Alias | Provider | Notes |
|-------|----------|-------|
| `haiku-4.5` | Anthropic | Fast and cheap |
| `sonnet-4.6` | Anthropic | Balanced |
| `gpt-5.4` | OpenAI | |
| `mistral-small` | Mistral | OpenAI-compatible API |

---

## Environment variables

Copy `.env.example` to `.env` at the root (or in each module directory) and fill in the relevant keys:

| Variable | Required for | Description |
|----------|-------------|-------------|
| `ANTHROPIC_API_KEY` | `haiku-4.5`, `sonnet-4.6` | Anthropic API key |
| `OPENAI_API_KEY` | `gpt-5.4` | OpenAI API key |
| `MISTRAL_API_KEY` | `mistral-small` | Mistral API key |
| `OPENWEATHERMAP_API_KEY` | mcp-owm | OpenWeatherMap API key |
| `TOOLS_MAX_CONCURRENT` | optional | Max concurrent tool executions (default: 4) |
| `X_API_KEY` | optional | Shared secret for MCP HTTP authentication |
| `OAUTH_AUTHORIZATION_SERVER` | optional | OAuth2 AS URL for MCP token validation |

---

## Make targets (root)

| Target | Description |
|--------|-------------|
| `make build` | Build all binaries (`talk`, `mcp-owm`, `mcp-playground`) |
| `make test` | Run tests for all modules |
| `make cover` | Run tests with coverage for all modules |
| `make cover-html` | Generate HTML coverage reports for all modules |
| `make vet` | Run `go vet` for all modules |
| `make lint` | Run `golangci-lint` for all modules |
| `make clean` | Remove build artifacts |

Each module also has its own Makefile with additional targets:

| Target (per module) | Description |
|---------------------|-------------|
| `make build` | Build that module's binary |
| `make run` | Run the binary (CLI or MCP server) |
| `make dev` | Hot-reload with `air` (MCP servers) |
| `make test` | Run module tests |
| `make dockerize` | Build Docker image (MCP servers) |

---

## System prompt

The CLI loads `system_prompt.md` from the `talk/` directory by default. Override at runtime:

```bash
go run ./cmd/cli --model sonnet-4.6 --system-file /path/to/prompt.md
```

---

## Versioning

Versions are resolved automatically:

| Context | Mechanism | Example |
|---------|-----------|---------|
| `make build` | `git describe --tags` via ldflags | `v1.2.0` or `v1.2.0-3-gdbe6a3e` |
| `make dev` / local run | `runtime/debug.ReadBuildInfo()` | `dbe6a3ee` (commit hash) |
| No VCS info | Fallback | `dev` |

---

## Project structure

```
.
├── go.work                         # Go workspace (links all modules)
├── Makefile                        # Root orchestrator
├── .github/
│   └── workflows/ci.yml           # CI: test + lint per module
├── talk-libs/                      # Shared library module
│   ├── domain/                    #   TypedTool interface, Adapt()
│   ├── logger/                    #   Structured colored logging
│   ├── mcpserver/                 #   MCP server framework (App, OAuth, transports)
│   └── version/                   #   Build-time version injection
├── talk/                           # CLI module
│   ├── cmd/cli/                   #   Entry point (REPL)
│   ├── internal/
│   │   ├── domain/                #   Conversation, Message, Model, Store…
│   │   ├── config/                #   .env loader
│   │   ├── llm/                   #   Anthropic, OpenAI, Router
│   │   ├── memory/                #   InMemory & Langfuse stores
│   │   ├── prompt/                #   File & static prompt providers
│   │   └── usage/                 #   Console, Langfuse, OTLP reporters
│   └── system_prompt.md
├── mcp-owm/                        # OpenWeatherMap MCP server module
│   ├── cmd/                       #   Entry point
│   ├── internal/
│   │   ├── config/                #   Server env loader
│   │   ├── tools/                 #   Weather, geocoding, forecast tools
│   │   └── testutils/
│   ├── .air.toml                  #   Hot-reload config
│   └── Dockerfile
└── mcp-playground/                  # Empty MCP server template
    ├── cmd/
    ├── internal/config/
    └── Dockerfile
```
