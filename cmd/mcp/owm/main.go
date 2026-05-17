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
	geocodingTool := openweather.NewGeocodingTool(env.OpenWeatherMapAPIKey)
	reverseGeocodingTool := openweather.NewReverseGeocodingTool(env.OpenWeatherMapAPIKey)
	airPollutionTool := openweather.NewAirPollutionTool(env.OpenWeatherMapAPIKey)
	airPollutionForecastTool := openweather.NewAirPollutionForecastTool(env.OpenWeatherMapAPIKey)

	opts := []mcpserver.Option{
		mcpserver.WithTools(mcpserver.RegisterTool(weatherTool)),
		mcpserver.WithTools(mcpserver.RegisterTool(geocodingTool)),
		mcpserver.WithTools(mcpserver.RegisterTool(reverseGeocodingTool)),
		mcpserver.WithTools(mcpserver.RegisterTool(airPollutionTool)),
		mcpserver.WithTools(mcpserver.RegisterTool(airPollutionForecastTool)),
	}

	if env.FreePlan {
		forecastTool := openweather.NewForecast5Days3HoursWeatherTool(env.OpenWeatherMapAPIKey)
		opts = append(opts, mcpserver.WithTools(mcpserver.RegisterTool(forecastTool)))
	} else {
		hourlyForecastTool := openweather.NewHourlyForecastTool(env.OpenWeatherMapAPIKey)
		dailyForecastTool := openweather.NewDailyForecastTool(env.OpenWeatherMapAPIKey)
		opts = append(opts,
			mcpserver.WithTools(mcpserver.RegisterTool(hourlyForecastTool)),
			mcpserver.WithTools(mcpserver.RegisterTool(dailyForecastTool)),
		)
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

	app := mcpserver.NewApp("owm-mcp", "1.0.0", opts...)
	app.Run()
}
