---
description: "Project layout conventions for the talks monorepo"
applyTo: "**/*.go,**/go.mod,Makefile"
---

# Project Layout

This is a Go workspaces monorepo with multiple independent modules coordinated by `go.work`.

## Module structure

```
go.work                 Workspace file linking all modules
talk-libs/              Shared library module (talks/talk-libs)
  domain/               Core business types and interfaces. No external dependencies.
  logger/               Colored structured logging.
  mcpserver/            MCP server framework (App, ToolRegistrar, OAuth, etc.).
  version/              Build-time injected version string.
talk/                   CLI/web application module (talks/talk)
  cmd/cli/              CLI entry point.
  internal/             Private implementation details.
    config/             Environment variable loading.
    llm/                LLM provider adapters (anthropic/, openai/, router/).
    memory/             MessageStore implementations (inmemory/, langfuse/).
    prompt/             PromptProvider implementations.
    usage/              UsageReporter implementations.
mcp-owm/               OpenWeather MCP server module (talks/mcp-owm)
  cmd/                  Server entry point.
  internal/config/      Server-specific configuration.
  internal/tools/       OpenWeather tool implementations.
mcp-playground/         Template MCP server module (talks/mcp-playground)
  cmd/                  Server entry point (no tools registered).
  internal/config/      Server-specific configuration.
```

## Rules

- **Each module** has its own `go.mod`, `Makefile`, and is independently buildable.
- **Shared types and frameworks** go in `talk-libs/`. This module has zero application logic.
- **New MCP servers** should be created as new top-level modules following `mcp-playground/` as a template.
- **No cross-dependencies between applications** — `talk`, `mcp-owm`, and `mcp-playground` never import each other. They only depend on `talk-libs`.
- **`domain/` must have zero external library dependencies** (except `swaggest/jsonschema-go` for schema reflection).
- **`main.go` must stay thin**: import, wire, run — no business logic.
- **Dockerfiles** use the workspace root as build context to resolve `go.work` dependencies.
- **Versioning**: each module is tagged independently (`talk/v1.0.0`, `mcp-owm/v0.8.0`, etc.).
