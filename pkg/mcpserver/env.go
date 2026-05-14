package mcpserver

import (
	"os"
	"strings"
)

// BaseEnv holds the environment configuration common to all MCP servers.
type BaseEnv struct {
	APIKey                   string // X_API_KEY — shared secret for authenticating HTTP clients
	BaseURL                  string // BASE_URL — public-facing URL of this server (e.g. ngrok URL)
	OAuthAuthorizationServer string // OAUTH_AUTHORIZATION_SERVER — issuer URL of the external AS
	OAuthAudience            string // OAUTH_AUDIENCE — expected aud claim in the JWT (API identifier)
	OAuthScopes              string // OAUTH_SCOPES — comma-separated list of required scopes
}

// LoadBaseEnv reads the common MCP server environment variables.
func LoadBaseEnv() BaseEnv {
	return BaseEnv{
		APIKey:                   os.Getenv("X_API_KEY"),
		BaseURL:                  os.Getenv("BASE_URL"),
		OAuthAuthorizationServer: os.Getenv("OAUTH_AUTHORIZATION_SERVER"),
		OAuthAudience:            os.Getenv("OAUTH_AUDIENCE"),
		OAuthScopes:              os.Getenv("OAUTH_SCOPES"),
	}
}

// OAuthScopesList returns the OAuth scopes as a string slice.
func (e BaseEnv) OAuthScopesList() []string {
	if e.OAuthScopes == "" {
		return nil
	}
	parts := strings.Split(e.OAuthScopes, ",")
	scopes := make([]string, 0, len(parts))
	for _, s := range parts {
		s = strings.TrimSpace(s)
		if s != "" {
			scopes = append(scopes, s)
		}
	}
	return scopes
}
