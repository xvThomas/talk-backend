package main

import (
	"os"

	"github.com/xvThomas/LLMClientWrapper/mcp-owm/internal/config"
	"github.com/xvThomas/LLMClientWrapper/mcp-owm/internal/prompts"
	"github.com/xvThomas/LLMClientWrapper/mcp-owm/internal/ratelimit"
	"github.com/xvThomas/LLMClientWrapper/mcp-owm/internal/tools"
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

	limiter := ratelimit.NewLimiter(env.RateLimitPerMinute)

	weatherTool := tools.NewCurrentWeatherTool(env.OpenWeatherMapAPIKey, limiter)
	geocodingTool := tools.NewGeocodingTool(env.OpenWeatherMapAPIKey, limiter)
	reverseGeocodingTool := tools.NewReverseGeocodingTool(env.OpenWeatherMapAPIKey, limiter)
	airPollutionTool := tools.NewAirPollutionTool(env.OpenWeatherMapAPIKey, limiter)
	airPollutionForecastTool := tools.NewAirPollutionForecastTool(env.OpenWeatherMapAPIKey, limiter)

	opts := []mcpserver.Option{
		mcpserver.WithTools(mcpserver.RegisterTool(weatherTool)),
		mcpserver.WithTools(mcpserver.RegisterTool(geocodingTool)),
		mcpserver.WithTools(mcpserver.RegisterTool(reverseGeocodingTool)),
		mcpserver.WithTools(mcpserver.RegisterTool(airPollutionTool)),
		mcpserver.WithTools(mcpserver.RegisterTool(airPollutionForecastTool)),
		mcpserver.WithPrompts(
			mcpserver.RegisterPrompt(prompts.CurrentWeather),
			mcpserver.RegisterPrompt(prompts.CurrentAir),
			mcpserver.RegisterPrompt(prompts.ForecastAir),
		),
	}

	if env.FreePlan {
		forecastTool := tools.NewForecast5Days3HoursWeatherTool(env.OpenWeatherMapAPIKey, limiter)
		opts = append(opts,
			mcpserver.WithTools(mcpserver.RegisterTool(forecastTool)),
			mcpserver.WithPrompts(mcpserver.RegisterPrompt(prompts.ForecastWeather)),
		)
	} else {
		hourlyForecastTool := tools.NewHourlyForecastTool(env.OpenWeatherMapAPIKey, limiter)
		dailyForecastTool := tools.NewDailyForecastTool(env.OpenWeatherMapAPIKey, limiter)
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

	app := mcpserver.NewApp("owm-mcp", version.Version, opts...)
	app.Run()
}
