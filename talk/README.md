# talk

Interactive CLI for multi-turn conversations with LLM providers (Anthropic, OpenAI, Mistral) through a unified interface. Supports MCP tool servers, session management, and observability via Langfuse.

---

## Quickstart

```bash
# Copy and fill in your API keys
cp .env.example .env
$EDITOR .env

# Build the binary
make build

# Run with default parameters
make run

# Run with a specific model
make run MODEL=sonnet-4.6
```

---

## Available models

| Alias           | Provider  | Notes                 |
| --------------- | --------- | --------------------- |
| `haiku-4.5`     | Anthropic | Fast and cheap        |
| `sonnet-4.6`    | Anthropic | Balanced              |
| `gpt-5.4`       | OpenAI    |                       |
| `mistral-small` | Mistral   | OpenAI-compatible API |

---

## CLI commands

Once in the REPL:

| Command                        | Description               |
| ------------------------------ | ------------------------- |
| `/help`                        | Show available commands   |
| `/model`                       | Switch LLM model          |
| `/memory`                      | Show session history      |
| `/session [list\|new\|remove]`   | Manage sessions           |
| `/prompt`                      | Show system prompt        |
| `/mcp [list\|add\|remove\|refresh]` | Manage MCP servers     |
| `/q`                           | Quit                      |

---

## Environment variables

Copy `.env.example` to `.env` and fill in the relevant keys:

| Variable                 | Required | Default                      | Description                                                                   |
| ------------------------ | -------- | ---------------------------- | ----------------------------------------------------------------------------- |
| `ANTHROPIC_API_KEY`      | yes*     | -                            | Anthropic API key (for `haiku-4.5`, `sonnet-4.6`)                             |
| `OPENAI_API_KEY`         | yes*     | -                            | OpenAI API key (for `gpt-5.4`)                                                |
| `MISTRAL_API_KEY`        | yes*     | -                            | Mistral API key (for `mistral-small`)                                         |                 |
| `TOOLS_MAX_CONCURRENT`   | optional | `4`                          | Maximum concurrent tool executions (`1` = sequential)                          |
| `CONTEXT_FULL_TURNS`     | optional | `-1`                         | Context mode: `-1` full, `0` lean, `N>0` hybrid with last `N` detailed turns  |
| `LANGFUSE_PUBLIC_KEY`    | optional | -                            | Langfuse public key                                                            |
| `LANGFUSE_SECRET_KEY`    | optional | -                            | Langfuse secret key                                                            |
| `LANGFUSE_BASE_URL`      | optional | `https://cloud.langfuse.com` | Langfuse base URL (EU cloud default)                                           |
| `CONSOLE_USAGE_REPORTER` | optional | `true`                       | Enable/disable console usage reporter                                          |

*At least one provider key is required depending on the model you use.

---

## Make targets

| Target        | Description                                         |
| ------------- | --------------------------------------------------- |
| `make build`  | Build the `talk-cli` binary into `bin/`             |
| `make run`    | Run the CLI (`MODEL=haiku-4.5` by default)          |
| `make test`   | Run tests                                           |
| `make cover`  | Run tests with coverage                             |
| `make vet`    | Run `go vet`                                        |
| `make clean`  | Remove build artifacts                              |
| `make all`    | Run vet, test, and build                            |

Override the model and system prompt at runtime:

```bash
make run MODEL=sonnet-4.6 SYSTEM_FILE=./my_prompt.md
```

---

## System prompt

The CLI loads `system_prompt.md` from this directory by default. Override with `--system-file`:

```bash
go run ./cmd/cli --model sonnet-4.6 --system-file /path/to/prompt.md
```

---

## Project structure

```
talk/
├── cmd/cli/                    # Entry point (REPL, commands, printer)
├── internal/
│   ├── config/                 # .env loader
│   ├── domain/                 # Conversation, Message, Model, Store, ToolExecutor
│   ├── helpers/                # Shared utilities
│   ├── llm/                    # LLM providers
│   │   ├── anthropic/          #   Anthropic client
│   │   ├── openai/             #   OpenAI client
│   │   └── router/             #   Model router
│   ├── mcp/                    # MCP server manager
│   ├── memory/                 # InMemory & SQLLite stores
│   ├── prompt/                 # File & static prompt providers
│   └── usage/                  # Console, Langfuse, OTLP reporters
├── system_prompt.md            # Default system prompt
├── .env.example                # Environment variables template
├── Makefile                    # Build & run targets
└── go.mod
```


Your Account is not present in SSC AD
You encountered a problem with the Active Directory, or the DAS Id you typed exists but the KI support hasn't yet added your account in the Citrix users base. In this case, be sure you sent a request to the KI Tools support by creating a ticket