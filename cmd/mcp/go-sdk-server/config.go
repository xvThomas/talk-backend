package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"talks/pkg/mcpserver"
)

// serverEnv holds the environment configuration specific to this MCP server.
type serverEnv struct {
	mcpserver.BaseEnv
	OpenWeatherMapAPIKey string // OPENWEATHERMAP_API_KEY — API key for the weather tool
}

// loadServerEnv reads .env files (if present) then loads the environment
// variables required by the go-sdk-server.
// It tries multiple paths so the binary works whether executed from the repo
// root or from the server directory.
// godotenv.Load never overrides already-set environment variables.
func loadServerEnv(envFiles ...string) (*serverEnv, error) {
	_ = godotenv.Load(envFiles...)

	env := &serverEnv{
		BaseEnv:              mcpserver.LoadBaseEnv(),
		OpenWeatherMapAPIKey: os.Getenv("OPENWEATHERMAP_API_KEY"),
	}

	if env.OpenWeatherMapAPIKey == "" {
		return nil, fmt.Errorf("missing required environment variable %q", "OPENWEATHERMAP_API_KEY")
	}

	return env, nil
}
