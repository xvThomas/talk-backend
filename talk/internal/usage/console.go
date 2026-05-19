package usage

import (
	"fmt"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
)

// ConsoleUsageReporter implements domain.UsageReporter by printing token usage
// to the terminal using ANSI color helpers.
// It is the default reporter for the CLI session when CONSOLE_USAGE_REPORTER=true.
// Can be combined with other reporters like LangfuseUsageReporter for dual logging.
type ConsoleUsageReporter struct{}

var _ domain.UsageReporter = (*ConsoleUsageReporter)(nil) // compile-time interface check

// OnAPICall is called after every API call and prints the token usage for that call.
//
// Parameters:
// - e: The APICallEvent containing details about the API call and its token usage.
func (ConsoleUsageReporter) OnAPICall(e domain.APICallEvent) {
	fmt.Printf(
		faint("  ↳ [token] model=%-14s kind=%-12s in=%5d out=%5d cache_read=%5d cache_write=%5d\n"),
		e.Model, string(e.Kind),
		e.Usage.InputTokens, e.Usage.OutputTokens,
		e.Usage.CacheReadTokens, e.Usage.CacheWriteTokens,
	)
}

// OnConversationTurn is called after every conversation turn and prints the token usage for that turn.
//
// Parameters:
// - e: The TurnEvent containing details about the conversation turn and its token usage.
func (ConsoleUsageReporter) OnConversationTurn(e domain.TurnEvent) {
	fmt.Printf(
		faint("  ↳ [turn]  model=%-14s calls=%d  total_in=%5d total_out=%5d cache_read=%5d cache_write=%5d\n"),
		e.Model, e.CallCount,
		e.TotalUsage.InputTokens, e.TotalUsage.OutputTokens,
		e.TotalUsage.CacheReadTokens, e.TotalUsage.CacheWriteTokens,
	)
}

// faint returns the input string with ANSI faint (dim) formatting.
// This is a helper function for consistent terminal output styling.
func faint(s string) string {
	return "\033[2m" + s + "\033[0m"
}
