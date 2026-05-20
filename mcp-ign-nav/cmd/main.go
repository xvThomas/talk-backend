package main

import (
	"os"

	"github.com/xvThomas/LLMClientWrapper/mcp-ign-nav/internal/config"
	"github.com/xvThomas/LLMClientWrapper/mcp-ign-nav/internal/tools"
	"github.com/xvThomas/LLMClientWrapper/talk-libs/logger"
	"github.com/xvThomas/LLMClientWrapper/talk-libs/mcpserver"
	"github.com/xvThomas/LLMClientWrapper/talk-libs/version"
	"golang.org/x/time/rate"
)

func main() {
	log := logger.GetLogger()

	env, err := config.LoadServerEnv(".env")
	if err != nil {
		log.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Shared rate limiter for IGN Géoplateforme endpoints (50 req/s).
	ignLimiter := rate.NewLimiter(rate.Limit(50), 50)

	reverseGeocodeTool := tools.NewReverseGeocodingTool(ignLimiter)
	geocodeTool := tools.NewGeocodingTool(ignLimiter)

	opts := []mcpserver.Option{
		mcpserver.WithTools(
			mcpserver.RegisterTool(reverseGeocodeTool),
			mcpserver.RegisterTool(geocodeTool),
		),
	}

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

	app := mcpserver.NewApp("ign-nav-mcp", version.Version, opts...)
	app.Run()
}
