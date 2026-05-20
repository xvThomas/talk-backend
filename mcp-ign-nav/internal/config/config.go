package config

import (
	"github.com/joho/godotenv"
	"github.com/xvThomas/LLMClientWrapper/talk-libs/mcpserver"
)

// ServerEnv holds the environment configuration for the mcp-ign-nav server.
type ServerEnv struct {
	mcpserver.BaseEnv
}

// LoadServerEnv loads environment variables from the given files and returns the server configuration.
func LoadServerEnv(envFiles ...string) (*ServerEnv, error) {
	_ = godotenv.Load(envFiles...)

	env := &ServerEnv{
		BaseEnv: mcpserver.LoadBaseEnv(),
	}

	return env, nil
}
