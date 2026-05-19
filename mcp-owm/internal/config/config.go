package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"

	"github.com/xvThomas/LLMClientWrapper/talk-libs/mcpserver"
)

// ServerEnv holds the environment configuration specific to this MCP server.
type ServerEnv struct {
	mcpserver.BaseEnv
	OpenWeatherMapAPIKey string // OPENWEATHERMAP_API_KEY — API key for the weather tool
	FreePlan             bool   // OPENWEATHERMAP_FREE_PLAN — when true (default), pro-only tools are excluded
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
	}

	if env.OpenWeatherMapAPIKey == "" {
		return nil, fmt.Errorf("missing required environment variable %q", "OPENWEATHERMAP_API_KEY")
	}

	return env, nil
}
