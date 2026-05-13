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

	app := &mcpserver.App{
		Name:    "weather-mcp",
		Version: "1.0.0",
		APIKey:  env.APIKey,
		Tools: []mcpserver.ToolRegistrar{
			mcpserver.RegisterTool(weatherTool),
		},
	}
	app.Run()
}
