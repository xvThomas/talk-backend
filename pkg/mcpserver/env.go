package mcpserver

import "os"

// BaseEnv holds the environment configuration common to all MCP servers.
type BaseEnv struct {
	APIKey string // X_API_KEY — shared secret for authenticating HTTP clients
}

// LoadBaseEnv reads the common MCP server environment variables.
func LoadBaseEnv() BaseEnv {
	return BaseEnv{
		APIKey: os.Getenv("X_API_KEY"),
	}
}
