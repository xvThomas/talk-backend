---
applyTo: talk/cmd/cli/**
---

# CLI colour conventions

All terminal output in `src/cmd/` must use the ANSI helper functions defined at
the top of `main.go`. Never write raw `\033[…m` escape sequences inline.

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

- **Errors** written to `os.Stderr` use `red(…)` for the label, plain text for
  the error value (the error string itself is not coloured).
- **Never** apply colour to user-provided content (answers from the LLM, prompt
  text, tool output).
- **No external colour library** (e.g. `fatih/color`, `charmbracelet/lipgloss`)
  should be added. The six helpers above are sufficient.
