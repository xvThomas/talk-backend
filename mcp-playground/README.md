# mcp-playground

A minimal MCP server used as a reference implementation for the `talk-libs/mcpserver` framework.

## Tools

| Tool  | Description                    |
|-------|--------------------------------|
| `sum` | Compute the sum of two integers |

## Run

```bash
make dev
```

## Authentication

This server supports **X-API-Key** and **OAuth 2.0** authentication.  
See [docs/mcp-server-authentication.md](../docs/mcp-server-authentication.md) for details.

## Security

This server includes built-in HTTP security hardening (rate limiting, path filtering, security headers, timeouts).  
See [docs/mcp-server-secured.md](../docs/mcp-server-secured.md) for configuration details.
