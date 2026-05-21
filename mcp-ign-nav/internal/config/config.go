package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/xvThomas/LLMClientWrapper/talk-libs/mcpserver"
)

// ServerEnv holds the environment configuration for the mcp-ign-nav server.
type ServerEnv struct {
	mcpserver.BaseEnv

	// GetGeoJSONGeometry controls whether the route tool returns GeoJSON geometry.
	// Set to "true" to enable. Defaults to false.
	GetGeoJSONGeometry bool
}

// LoadServerEnv loads environment variables from the given files and returns the server configuration.
func LoadServerEnv(envFiles ...string) (*ServerEnv, error) {
	_ = godotenv.Load(envFiles...)

	env := &ServerEnv{
		BaseEnv:            mcpserver.LoadBaseEnv(),
		GetGeoJSONGeometry: strings.EqualFold(os.Getenv("GET_GEOJSON_GEOMETRY"), "true"),
	}

	return env, nil
}
