package main

import (
	"os"

	"github.com/xvThomas/LLMClientWrapper/mcp-playground/internal/config"
	"github.com/xvThomas/LLMClientWrapper/talk-libs/logger"
	"github.com/xvThomas/LLMClientWrapper/talk-libs/mcpserver"
	"github.com/xvThomas/LLMClientWrapper/talk-libs/version"
)

func main() {
	log := logger.GetLogger()

	env, err := config.LoadServerEnv(".env")
	if err != nil {
		log.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	var opts []mcpserver.Option

	// Register your tools here:
	// opts = append(opts, mcpserver.WithTools(mcpserver.RegisterTool(myTool)))

	if env.APIKey != "" {
		opts = append(opts, mcpserver.WithAPIKey(env.APIKey))
	}
	if env.OAuthAuthorizationServer != "" {
		opts = append(opts, mcpserver.WithOAuth(&mcpserver.OAuthConfig{
			AuthorizationServerURL: env.OAuthAuthorizationServer,
			ResourceBaseURL:        env.BaseURL,
			Scopes:                 env.OAuthScopesList(),
			TokenVerifier: mcpserver.NewJWKSTokenVerifier(mcpserver.JWKSVerifierConfig{
				IssuerURL: env.OAuthAuthorizationServer,
				Audience:  env.OAuthAudience,
			}),
		}))
	}

	app := mcpserver.NewApp("playground-mcp", version.Version, opts...)
	app.Run()
}
