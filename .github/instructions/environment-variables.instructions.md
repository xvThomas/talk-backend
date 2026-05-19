---
applyTo: talk/internal/config/loader.go, mcp-*/internal/config/*, .env.example
---

# Environment Variables Documentation Update

When any changes are made to environment variables in `config/loader.go` or `.env.example`, you **MUST** automatically update the "Environment variables" section in `README.md` to maintain documentation synchronization.

## Requirements

1. **Always check the current README.md** - Read the current "Environment variables" section before making changes
2. **Extract all variables from config/loader.go** - Parse the `Config` struct fields that correspond to environment variables
3. **Include default values and descriptions** from:
   - Comments in `config/loader.go`
   - Comments in `.env.example`
   - Default values from parsing functions (e.g., `parseToolsMaxConcurrent`)

## Documentation Format

The README "Environment variables" section should follow this structure:

```markdown
## Environment variables

Copy `.env.example` to `.env` and fill in the relevant keys:

| Variable                 | Required for              | Default | Description                                                  |
| ------------------------ | ------------------------- | ------- | ------------------------------------------------------------ |
| `ANTHROPIC_API_KEY`      | `haiku-4.5`, `sonnet-4.6` | -       | API key for Anthropic models                                 |
| `OPENAI_API_KEY`         | `gpt-5.4`                 | -       | API key for OpenAI models                                    |
| `MISTRAL_API_KEY`        | `mistral-small`           | -       | API key for Mistral models                                   |
| `OPENWEATHERMAP_API_KEY` | weather tool calls        | -       | API key for OpenWeatherMap weather tool                      |
| `TOOLS_MAX_CONCURRENT`   | optional                  | `4`     | Maximum concurrent tool executions (set to 1 for sequential) |
```

## Extraction Rules

1. **API Keys**: Variables ending in `_API_KEY`
   - Required for specific models/features
   - No default value (marked as `-`)

2. **Configuration Variables**: Other variables with defaults
   - Extract default from parsing functions or comments
   - Include description from comments

3. **Model Mapping**: Keep model aliases consistent with the "Available models" section

## Automation Triggers

This instruction is triggered when:

- Adding/removing fields in `Config` struct
- Modifying parsing functions with default values
- Adding/updating comments in `.env.example`
- Any change that affects environment variable behavior

## Consistency Check

Ensure consistency between:

- `config/loader.go` (Config struct fields)
- `.env.example` (documented variables and comments)
- `README.md` (Environment variables table)

**Never skip this update - documentation drift creates poor user experience.**
