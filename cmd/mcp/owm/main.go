package main

import (
	"log"

	"talks/internal/domain"
	"talks/internal/infrastructure/config"
	"talks/internal/infrastructure/tools/openweather"
	"talks/pkg/mcp"
	"talks/pkg/mcp/playground"
)

func main() {

	/*
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to get current file path")
	}
	// From internal/helpers/testutils/ go up 4 levels to project root
	dir := filepath.Dir(filename)
	*/

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
