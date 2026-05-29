package mcp

import "context"

// AuthType represents the authentication method for an MCP server.
type AuthType string

const (
	AuthTypeNone   AuthType = "none"
	AuthTypeAPIKey AuthType = "apikey"
	AuthTypeOAuth  AuthType = "oauth"
)

// OAuthConfig holds OAuth 2.0 credentials for an MCP server.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	Scopes       []string
}

// ServerConfig represents a registered MCP server.
type ServerConfig struct {
	ID       string
	Name     string
	URL      string
	AuthType AuthType
	APIKey   string       // populated when AuthType == AuthTypeAPIKey
	OAuth    *OAuthConfig // populated when AuthType == AuthTypeOAuth
}

// Registry provides CRUD operations for MCP server configurations.
type Registry interface {
	Add(ctx context.Context, cfg ServerConfig) error
	Remove(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (ServerConfig, error)
	List(ctx context.Context) ([]ServerConfig, error)
}
