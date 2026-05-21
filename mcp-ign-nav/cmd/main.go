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

	// Rate limiter for navigation endpoint (5 req/s).
	navLimiter := rate.NewLimiter(rate.Limit(5), 5)

	reverseGeocodeTool := tools.NewReverseGeocodingTool(ignLimiter)
	geocodeTool := tools.NewGeocodingTool(ignLimiter)
	routeTool := tools.NewRouteTool(navLimiter, env.GetGeoJSONGeometry)

	opts := []mcpserver.Option{
		mcpserver.WithTools(
			mcpserver.RegisterTool(reverseGeocodeTool),
			mcpserver.RegisterTool(geocodeTool),
			mcpserver.RegisterTool(routeTool),
		),
	}

	if env.APIKey != "" {
		opts = append(opts, mcpserver.WithAPIKey(env.APIKey))
	}
	if env.OAuthAuthorizationServer != "" {
		oauthCfg := &mcpserver.OAuthConfig{
			AuthorizationServerURL: env.OAuthAuthorizationServer,
			ResourceBaseURL:        env.BaseURL,
			Scopes:                 env.OAuthScopesList(),
			TokenVerifier: mcpserver.NewJWKSTokenVerifier(mcpserver.JWKSVerifierConfig{
				IssuerURL: env.OAuthAuthorizationServer,
				Audience:  env.OAuthAudience,
			}),
		}
		if env.OAuthAudience != "" {
			oauthCfg.ASProxy = &mcpserver.ASProxyConfig{
				Audience:     env.OAuthAudience,
				ClientSecret: env.OAuthClientSecret,
			}
		}
		opts = append(opts, mcpserver.WithOAuth(oauthCfg))
	}

	app := mcpserver.NewApp("ign-nav-mcp", version.Version, opts...)
	app.Run()
}
