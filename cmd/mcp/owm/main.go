package main

import (
	"os"

	"talks/internal/infrastructure/tools/openweather"
	"talks/pkg/logger"
	"talks/pkg/mcpserver"
)

func main() {
	log := logger.GetLogger()

	env, err := loadServerEnv(".env")
	if err != nil {
		log.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	weatherTool := openweather.NewCurrentWeatherTool(env.OpenWeatherMapAPIKey)

	opts := []mcpserver.Option{
		mcpserver.WithTools(mcpserver.RegisterTool(weatherTool)),
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

	app := mcpserver.NewApp("owm-mcp", "1.0.0", opts...)
	app.Run()
}
