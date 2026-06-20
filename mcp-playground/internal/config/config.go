package config

import (
	"github.com/joho/godotenv"

	"github.com/xvThomas/talk-backend/talk-libs/mcpserver"
)

// ServerEnv holds the environment configuration specific to this MCP server.
type ServerEnv struct {
	mcpserver.BaseEnv
}

// LoadServerEnv reads .env files (if present) then loads the environment
// variables required by the playground server.
// godotenv.Load never overrides already-set environment variables.
func LoadServerEnv(envFiles ...string) (*ServerEnv, error) {
	_ = godotenv.Load(envFiles...)

	env := &ServerEnv{
		BaseEnv: mcpserver.LoadBaseEnv(),
	}

	return env, nil
}
