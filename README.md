# LLMClientWrapper

[![CI](https://github.com/xvThomas/LLMClientWrapper/actions/workflows/ci.yml/badge.svg)](https://github.com/xvThomas/LLMClientWrapper/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/xvThomas/LLMClientWrapper/branch/main/graph/badge.svg)](https://codecov.io/gh/xvThomas/LLMClientWrapper)

A Go monorepo providing a CLI that routes questions to Anthropic, OpenAI or OpenAI-compatible models (Mistral) through a unified interface, plus standalone MCP (Model Context Protocol) tool servers.

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                                  go.work (root)                                      │
├────────────┬────────────┬─────────────────┬─────────────────┬────────────────────────┤
│ talk-libs  │    talk    │    mcp-owm      │   mcp-ign-nav   │    mcp-playground      │
│ (shared)   │   (CLI)    │ (weather MCP)   │  (nav/geo MCP)  │   (empty template)     │
├────────────┼────────────┼─────────────────┼─────────────────┼────────────────────────┤
│ domain/    │ cmd/cli/   │ cmd/            │ cmd/            │ cmd/                   │
│ logger/    │ internal/  │ internal/       │ internal/       │ internal/              │
│ mcpserver/ │            │   config/       │   config/       │   config/              │
│ version/   │            │   tools/        │   tools/        │                        │
└────────────┴────────────┴─────────────────┴─────────────────┴────────────────────────┘
```

The project uses **Go workspaces** (`go.work`) with five independent modules:

| Module           | Path               | Description                                                                       |
| ---------------- | ------------------ | --------------------------------------------------------------------------------- |
| `talk-libs`      | `./talk-libs`      | Shared library: domain types (`TypedTool`), logger, MCP server framework, version |
| `talk`           | [`./talk`](talk/README.md) | Interactive CLI — multi-turn conversations with LLM providers               |
| `mcp-owm`        | [`./mcp-owm`](mcp-owm/README.md) | MCP server exposing OpenWeatherMap tools                           |
| `mcp-ign-nav`    | [`./mcp-ign-nav`](mcp-ign-nav/README.md) | MCP server exposing IGN Géoplateforme tools (geocoding, routing, distance/time) |
| `mcp-playground` | [`./mcp-playground`](mcp-playground/README.md) | Empty MCP server template for experimentation              |

---

## Prerequisites

- Go 1.25+
- `make`
- `golangci-lint` v2.12+ (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`)
- API keys for the providers you want to use (see each module's README)

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

## Make targets (root)

| Target            | Description                                                             |
| ----------------- | ----------------------------------------------------------------------- |
| `make build`      | Build all binaries (`talk`, `mcp-owm`, `mcp-ign-nav`, `mcp-playground`) |
| `make test`       | Run tests for all modules                                               |
| `make cover`      | Run tests with coverage for all modules                                 |
| `make cover-html` | Generate HTML coverage reports for all modules                          |
| `make vet`        | Run `go vet` for all modules                                            |
| `make lint`       | Run `golangci-lint` for all modules                                     |
| `make clean`      | Remove build artifacts                                                  |

Each module also has its own Makefile — see the respective README for details.

---

## Versioning

Versions are resolved automatically:

| Context                | Mechanism                         | Example                          |
| ---------------------- | --------------------------------- | -------------------------------- |
| `make build`           | `git describe --tags` via ldflags | `v1.2.0` or `v1.2.0-3-gdbe6a3e` |
| `make dev` / local run | `runtime/debug.ReadBuildInfo()`   | `dbe6a3ee` (commit hash)         |
| No VCS info            | Fallback                          | `dev`                            |
