---
applyTo: talk/cmd/cli/**
---

# CLI colour conventions

All terminal output in `talk/cmd/cli/` must use the ANSI helper functions defined
in `colors.go`. Never write raw `\033[…m` escape sequences inline.

## Architecture

The CLI uses an `App` struct (defined in `app.go`) that embeds a `Printer`
interface (defined in `printer.go`). All command functions are methods on `*App`
and must use `a.Printf`, `a.Println`, and `a.Errorf` instead of `fmt.Print*` or
`fmt.Fprint*(os.Stderr, …)`. This enables test code to capture output.

### File layout

| File              | Responsibility                                  |
| ----------------- | ----------------------------------------------- |
| `main.go`         | Cobra setup, `run()` wiring, path helpers       |
| `app.go`          | `App` struct definition                         |
| `printer.go`      | `Printer` interface + `stdPrinter`              |
| `colors.go`       | ANSI constants and helper functions             |
| `commands.go`     | Command dispatcher + `/help`, `/q`, `/prompt`   |
| `cmd_model.go`    | `/model` command                                |
| `cmd_mcp.go`      | `/mcp` sub-commands                             |
| `cmd_session.go`  | `/memory`, `/sessions`, `/session` commands     |
| `reader.go`       | `LineReader` (readline wrapper)                 |
| `history.go`      | Persistent history file                         |

## Helper functions

| Function       | Effect                |
| -------------- | --------------------- |
| `cyan(s)`      | Cyan text             |
| `green(s)`     | Green text            |
| `yellow(s)`    | Yellow text           |
| `red(s)`       | Red text              |
| `faint(s)`     | Dim / greyed-out text |
| `emphasize(s)` | Bold text             |

For bold + colour, concatenate the `bold` constant with the colour constant and
wrap with `reset`, e.g. `bold + colorCyan + s + reset`.

## Colour map

| Element                                    | Style                                    |
| ------------------------------------------ | ---------------------------------------- |
| `Session started.` banner                  | `cyan(bold + "…" + reset)`               |
| Banner sub-lines (commands list)           | `faint(…)`                               |
| `You:` prompt                              | `green(bold + "You" + reset + ":")`      |
| `Assistant:` label                         | `cyan(bold + "Assistant" + reset + ":")` |
| Current model in `/model` list             | `cyan(…)` + `green("← current")`         |
| Provider names in `/model` list            | `faint("(" + provider + ")")`            |
| `Switched to <model>.`                     | `green(model)`                           |
| Error messages                             | `red(…)`                                 |
| `/prompt` separators (`--- … ---`)         | `faint(…)`                               |
| `Session ended.`                           | `faint(…)`                               |
| Section headers (e.g. `Available models:`) | `emphasize(…)`                           |
| Unknown/invalid input warnings             | `yellow(…)`                              |

## Rules

- **All output** goes through `a.Printf`, `a.Println`, or `a.Errorf`. Never call
  `fmt.Print*` or `fmt.Fprint*(os.Stderr, …)` directly in command methods.
- **Errors** use `a.Errorf` with `red(…)` for the label, plain text for
  the error value (the error string itself is not coloured).
- **Never** apply colour to user-provided content (answers from the LLM, prompt
  text, tool output).
- **No external colour library** (e.g. `fatih/color`, `charmbracelet/lipgloss`)
  should be added. The six helpers above are sufficient.
