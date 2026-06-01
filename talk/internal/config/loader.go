package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all configuration loaded from environment variables.
type Config struct {
	// AnthropicAPIKey      string
	// OpenAIAPIKey         string
	// MistralAPIKey        string
	// PoolsideAPIKey       string
	OpenWeatherMapAPIKey string
	ToolsMaxConcurrent   int // Max concurrent tool executions (default: 4)
	ContextFullTurns     int // Context mode selector: -1 full, 0 lean, N hybrid:N

	// Langfuse configuration
	LangfuseSecretKey string // LANGFUSE_SECRET_KEY="sk-lf-..."
	LangfusePublicKey string // LANGFUSE_PUBLIC_KEY="pk-lf-..."
	LangfuseBaseURL   string // LANGFUSE_BASE_URL="https://cloud.langfuse.com" (default)

	// Reporter configuration
	ConsoleUsageReporter bool // CONSOLE_USAGE_REPORTER=true/false (default: true)

	// MCP server configuration
	McpAllowedOrigins []string // MCP_ALLOWED_ORIGINS=http://localhost:3000,http://127.0.0.1:3000 (comma-separated)
}

// Load reads the .env file (if present) then reads environment variables.
func Load(envFile string) (*Config, error) {
	// godotenv.Load does not override already-set env vars.
	_ = godotenv.Load(envFile)

	cfg := &Config{
		// AnthropicAPIKey:      os.Getenv("ANTHROPIC_API_KEY"),
		// OpenAIAPIKey:         os.Getenv("OPENAI_API_KEY"),
		// MistralAPIKey:        os.Getenv("MISTRAL_API_KEY"),
		// PoolsideAPIKey:       os.Getenv("POOLSIDE_API_KEY"),
		OpenWeatherMapAPIKey: os.Getenv("OPENWEATHERMAP_API_KEY"),
		ToolsMaxConcurrent:   parseToolsMaxConcurrent(os.Getenv("TOOLS_MAX_CONCURRENT")),
		ContextFullTurns:     parseContextFullTurns(os.Getenv("CONTEXT_FULL_TURNS")),

		// Langfuse configuration
		LangfuseSecretKey: os.Getenv("LANGFUSE_SECRET_KEY"),
		LangfusePublicKey: os.Getenv("LANGFUSE_PUBLIC_KEY"),
		LangfuseBaseURL:   parseLangfuseBaseURL(os.Getenv("LANGFUSE_BASE_URL")),

		// Reporter configuration
		ConsoleUsageReporter: parseConsoleUsageReporter(os.Getenv("CONSOLE_USAGE_REPORTER")),

		// MCP server configuration
		McpAllowedOrigins: parseMcpAllowedOrigins(os.Getenv("MCP_ALLOWED_ORIGINS")),
	}

	return cfg, nil
}

/*
// RequireAnthropicKey returns the Anthropic API key or an error if missing.
func (c *Config) RequireAnthropicKey() (string, error) {
	return requireKey(c.AnthropicAPIKey, "ANTHROPIC_API_KEY")
}

// RequireOpenAIKey returns the OpenAI API key or an error if missing.
func (c *Config) RequireOpenAIKey() (string, error) {
	return requireKey(c.OpenAIAPIKey, "OPENAI_API_KEY")
}

// RequireMistralKey returns the Mistral API key or an error if missing.
func (c *Config) RequireMistralKey() (string, error) {
	return requireKey(c.MistralAPIKey, "MISTRAL_API_KEY")
}

// RequirePoolsideKey returns the Poolside API key or an error if missing.
func (c *Config) RequirePoolsideKey() (string, error) {
	return requireKey(c.PoolsideAPIKey, "POOLSIDE_API_KEY")
}
	*/

// RequireOpenWeatherMapKey returns the OpenWeatherMap API key or an error if missing.
func (c *Config) RequireOpenWeatherMapKey() (string, error) {
	return requireKey(c.OpenWeatherMapAPIKey, "OPENWEATHERMAP_API_KEY")
}

func requireKey(value, name string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("missing required environment variable %q", name)
	}
	return value, nil
}

// GetKeyValue is a helper function to get an environment variable value or empty string if not set.
func GetKeyValue(name string) string {
	return os.Getenv(name)
}

// GetRequiredKeyValue is a helper function to get a required environment variable value or return error.
func GetRequiredKeyValue(name string) (string, error) {
	value := os.Getenv(name)	
	if value == "" {
		return "", fmt.Errorf("missing required environment variable %q", name)
	}
	return value, nil
}

// parseMcpAllowedOrigins parses MCP_ALLOWED_ORIGINS as a comma-separated list.
// Returns nil if the variable is empty (allow all origins).
func parseMcpAllowedOrigins(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

// parseToolsMaxConcurrent parses TOOLS_MAX_CONCURRENT with fallback to 4.
func parseToolsMaxConcurrent(value string) int {
	if value == "" {
		return 4 // Default value
	}
	if n, err := strconv.Atoi(value); err == nil && n > 0 {
		return n
	}
	return 4 // Fallback on invalid input
}

// parseContextFullTurns parses CONTEXT_FULL_TURNS with fallback to 0 (lean mode).
// Values: -1=full, 0=lean, N>0=hybrid:N.
func parseContextFullTurns(value string) int {
	if value == "" {
		return 0 // Default to lean mode for lower token cost; users can opt into full or hybrid modes.
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < -1 {
		return -1
	}
	return n
}

// parseLangfuseBaseURL parses LANGFUSE_BASE_URL with fallback to default.
func parseLangfuseBaseURL(value string) string {
	if value == "" {
		return "https://cloud.langfuse.com" // Default Langfuse Cloud EU region
	}
	return value
}

// parseConsoleUsageReporter parses CONSOLE_USAGE_REPORTER with fallback to true.
// Accepts: true, false, 1, 0, yes, no (case insensitive)
func parseConsoleUsageReporter(value string) bool {
	if value == "" {
		return true // Default: console reporter enabled for backward compatibility
	}

	switch value {
	case "true", "1", "yes", "True", "TRUE", "Yes", "YES":
		return true
	case "false", "0", "no", "False", "FALSE", "No", "NO":
		return false
	default:
		return true // Default on invalid input
	}
}
