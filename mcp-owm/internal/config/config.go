package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"

	"github.com/xvThomas/LLMClientWrapper/talk-libs/mcpserver"
)

const defaultRateLimitPerMinute = 60

// ServerEnv holds the environment configuration specific to this MCP server.
type ServerEnv struct {
	mcpserver.BaseEnv
	OpenWeatherMapAPIKey string // OPENWEATHERMAP_API_KEY — API key for the weather tool
	FreePlan             bool   // OPENWEATHERMAP_FREE_PLAN — when true (default), pro-only tools are excluded
	RateLimitPerMinute   int    // OPENWEATHERMAP_RATE_LIMIT_PER_MINUTE — max API calls per minute (default: 60)
}

// LoadServerEnv reads .env files (if present) then loads the environment
// variables required by the owm server.
// godotenv.Load never overrides already-set environment variables.
func LoadServerEnv(envFiles ...string) (*ServerEnv, error) {
	_ = godotenv.Load(envFiles...)

	freePlan := true
	if v := os.Getenv("OPENWEATHERMAP_FREE_PLAN"); v != "" {
		freePlan = !strings.EqualFold(v, "false")
	}

	env := &ServerEnv{
		BaseEnv:              mcpserver.LoadBaseEnv(),
		OpenWeatherMapAPIKey: os.Getenv("OPENWEATHERMAP_API_KEY"),
		FreePlan:             freePlan,
		RateLimitPerMinute:   parseIntEnv("OPENWEATHERMAP_RATE_LIMIT_PER_MINUTE", defaultRateLimitPerMinute),
	}

	if env.OpenWeatherMapAPIKey == "" {
		return nil, fmt.Errorf("missing required environment variable %q", "OPENWEATHERMAP_API_KEY")
	}

	return env, nil
}

func parseIntEnv(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultVal
	}
	return n
}
