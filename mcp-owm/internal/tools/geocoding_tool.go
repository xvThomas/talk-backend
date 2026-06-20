package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/xvThomas/talk-backend/mcp-owm/internal/ratelimit"
	"github.com/xvThomas/talk-backend/talk-libs/mcpserver"
)

const defaultGeoBaseURL = "http://api.openweathermap.org/geo/1.0"

// GeocodingToolInput is the typed input for GeocodingTool.
type GeocodingToolInput struct {
	City  string `json:"city" description:"City name, optionally followed by state code (US only) and country code, comma-separated (e.g. 'London', 'London,GB', 'Portland,OR,US')"`
	Limit int    `json:"limit,omitempty" description:"Optional maximum number of results (1-5). Defaults to 5."`
}

// GeocodingLocation represents a single geocoding result.
type GeocodingLocation struct {
	Name    string  `json:"name" description:"Name of the found location"`
	Lat     float64 `json:"lat" description:"Latitude"`
	Lon     float64 `json:"lon" description:"Longitude"`
	Country string  `json:"country" description:"Country code (ISO 3166)"`
	State   string  `json:"state,omitempty" description:"State (where available)"`
}

// GeocodingToolOutput is the typed output for GeocodingTool.
type GeocodingToolOutput struct {
	Locations []GeocodingLocation `json:"locations" description:"List of matching locations with coordinates"`
}

// GeocodingTool implements mcpserver.MCPTool for direct geocoding via OpenWeatherMap.
type GeocodingTool struct {
	apiKey  string
	baseURL string
	http    *http.Client
	limiter *ratelimit.Limiter
}

// NewGeocodingTool creates a GeocodingTool with the given API key.
func NewGeocodingTool(apiKey string, limiter *ratelimit.Limiter) mcpserver.MCPTool[GeocodingToolInput, GeocodingToolOutput] {
	return &GeocodingTool{apiKey: apiKey, baseURL: defaultGeoBaseURL, http: &http.Client{}, limiter: limiter}
}

var _ mcpserver.MCPTool[GeocodingToolInput, GeocodingToolOutput] = (*GeocodingTool)(nil)

// newGeocodingToolWithBaseURL creates a GeocodingTool with a custom base URL (for testing).
func newGeocodingToolWithBaseURL(apiKey, baseURL string, client *http.Client) *GeocodingTool {
	return &GeocodingTool{apiKey: apiKey, baseURL: baseURL, http: client, limiter: ratelimit.Noop()}
}

// Name returns the tool name as expected by the model.
func (t *GeocodingTool) Name() string { return "geocode" }

// Description describes what the tool does.
func (t *GeocodingTool) Description() string {
	return "Convert a city name into geographic coordinates (latitude, longitude). Supports city name with optional state code and country code. Returns up to 5 matching locations."
}

// Call calls the OpenWeatherMap Geocoding API and returns matching locations.
func (t *GeocodingTool) Call(ctx context.Context, input GeocodingToolInput) (GeocodingToolOutput, error) {
	if input.City == "" {
		return GeocodingToolOutput{}, fmt.Errorf("parameter 'city' must be a non-empty string")
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 5
	}

	endpoint := fmt.Sprintf("%s/direct?q=%s&limit=%d&appid=%s",
		t.baseURL, url.QueryEscape(input.City), limit, t.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return GeocodingToolOutput{}, fmt.Errorf("building geocoding request: %w", err)
	}

	if err := t.limiter.Wait(ctx); err != nil {
		return GeocodingToolOutput{}, fmt.Errorf("rate limiter: %w", err)
	}

	resp, err := t.http.Do(req)
	if err != nil {
		return GeocodingToolOutput{}, fmt.Errorf("geocoding API request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return GeocodingToolOutput{}, fmt.Errorf("geocoding API returned status %d", resp.StatusCode)
	}

	var results []geocodingResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return GeocodingToolOutput{}, fmt.Errorf("decoding geocoding response: %w", err)
	}

	out := GeocodingToolOutput{
		Locations: make([]GeocodingLocation, 0, len(results)),
	}
	for _, r := range results {
		out.Locations = append(out.Locations, GeocodingLocation(r))
	}

	return out, nil
}

type geocodingResponse struct {
	Name    string  `json:"name"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Country string  `json:"country"`
	State   string  `json:"state"`
}
