package main

import (
	"log"

	"talks/internal/domain"
	"talks/internal/infrastructure/config"
	"talks/internal/infrastructure/tools/openweather"
	"talks/pkg/mcp/playground"
	"talks/pkg/mcp"
)

func main() {
	cfg, err := config.Load(".env")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	owmKey, _ := cfg.RequireOpenWeatherMapKey()

	server := mcp.NewMcpServer("mcp-openweather-server", "0.1.0", 8080, []domain.Tool{
		domain.Adapt(playground.NewSumTool()),
		domain.Adapt(openweather.NewCurrentWeatherTool(owmKey)),
	}, cfg.McpAllowedOrigins)
	server.Start()
}
