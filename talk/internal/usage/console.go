package usage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
)

// ConsoleUsageReporter implements domain.MessageEventHandler by printing token usage
// to the terminal using ANSI color helpers.
// It is the default reporter for the CLI session when CONSOLE_USAGE_REPORTER=true.
// Can be combined with other reporters like LangfuseUsageReporter for dual logging.
type ConsoleUsageReporter struct{}

var _ domain.MessageEventHandler = (*ConsoleUsageReporter)(nil) // compile-time interface check

// HandleMessageEvent is called for every message event and prints usage for assistant LLM calls
// and tool invocation details for tool calls.
//
// Parameters:
// - e: The MessageEvent containing details about the call and its token usage.
func (ConsoleUsageReporter) HandleMessageEvent(_ context.Context, e domain.MessageEvent) error {
	switch e.Kind {
	case domain.CallKindToolCall:
		if len(e.Message.ToolCalls) == 0 {
			return nil
		}
		tc := e.Message.ToolCalls[0]
		inputJSON, _ := json.Marshal(tc.Input)
		fmt.Printf(
			faint("  ↳   [tool call] tool=%s args=%s\n"),
			tc.Name, string(inputJSON),
		)

	case domain.CallKindInitial, domain.CallKindToolResult:
		if e.Message.Role != domain.RoleAssistant {
			return nil
		}
		fmt.Printf(
			faint("  ↳ [api call] model=%-14s kind=%-12s in=%5d out=%5d cache_read=%5d cache_write=%5d\n"),
			e.Model.Name, string(e.Kind),
			e.Usage.InputTokens, e.Usage.OutputTokens,
			e.Usage.CacheReadTokens, e.Usage.CacheWriteTokens,
		)
	}

	return nil
}

// HandleTurnEvent is called after every conversation turn and prints aggregated usage.
//
// Parameters:
// - e: The TurnEvent containing details about the conversation turn and its token usage.
func (ConsoleUsageReporter) HandleTurnEvent(_ context.Context, e domain.TurnEvent) error {
	fmt.Printf(
		faint("  ↳ [turn]  model=%-14s calls=%d  total_in=%5d total_out=%5d cache_read=%5d cache_write=%5d\n"),
		e.Model.Name, e.CallCount,
		e.TotalUsage.InputTokens, e.TotalUsage.OutputTokens,
		e.TotalUsage.CacheReadTokens, e.TotalUsage.CacheWriteTokens,
	)

	return nil
}

// faint returns the input string with ANSI faint (dim) formatting.
// This is a helper function for consistent terminal output styling.
func faint(s string) string {
	return "\033[2m" + s + "\033[0m"
}
