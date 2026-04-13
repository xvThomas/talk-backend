package tools

import (
	"talks/internal/domain"
	"talks/internal/infrastructure/config"
	"talks/internal/infrastructure/tools/openweather"
)

// Tools aggregates all available domain.Tool implementations.
type Tools struct {
	cfg *config.Config
}

// New creates a Tools aggregator backed by the given configuration.
func New(cfg *config.Config) *Tools {
	return &Tools{cfg: cfg}
}

// All returns the list of all registered tools as type-erased Tool handlers.
func (t *Tools) All() []domain.Tool {
	w := openweather.NewCurrentWeatherTool(t.cfg.OpenWeatherMapAPIKey)
	return []domain.Tool{
		domain.Adapt(w /*, w.Parameters*/),
	}
}
