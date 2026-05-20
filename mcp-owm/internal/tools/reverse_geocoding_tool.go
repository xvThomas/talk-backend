package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/xvThomas/LLMClientWrapper/mcp-owm/internal/ratelimit"
	"github.com/xvThomas/LLMClientWrapper/talk-libs/mcpserver"
)

// ReverseGeocodingToolInput is the typed input for ReverseGeocodingTool.
type ReverseGeocodingToolInput struct {
	Lat   float64 `json:"lat" description:"Latitude of the location"`
	Lon   float64 `json:"lon" description:"Longitude of the location"`
	Limit int     `json:"limit,omitempty" description:"Optional maximum number of results (1-5). Defaults to 5."`
}

// ReverseGeocodingToolOutput is the typed output for ReverseGeocodingTool.
type ReverseGeocodingToolOutput struct {
	Locations []GeocodingLocation `json:"locations" description:"List of location names for the given coordinates"`
}

// ReverseGeocodingTool implements mcpserver.MCPTool for reverse geocoding via OpenWeatherMap.
type ReverseGeocodingTool struct {
	apiKey  string
	baseURL string
	http    *http.Client
	limiter *ratelimit.Limiter
}

// NewReverseGeocodingTool creates a ReverseGeocodingTool with the given API key.
func NewReverseGeocodingTool(apiKey string, limiter *ratelimit.Limiter) mcpserver.MCPTool[ReverseGeocodingToolInput, ReverseGeocodingToolOutput] {
	return &ReverseGeocodingTool{apiKey: apiKey, baseURL: defaultGeoBaseURL, http: &http.Client{}, limiter: limiter}
}

var _ mcpserver.MCPTool[ReverseGeocodingToolInput, ReverseGeocodingToolOutput] = (*ReverseGeocodingTool)(nil)

// newReverseGeocodingToolWithBaseURL creates a ReverseGeocodingTool with a custom base URL (for testing).
func newReverseGeocodingToolWithBaseURL(apiKey, baseURL string, client *http.Client) *ReverseGeocodingTool {
	return &ReverseGeocodingTool{apiKey: apiKey, baseURL: baseURL, http: client, limiter: ratelimit.Noop()}
}

// Name returns the tool name as expected by the model.
func (t *ReverseGeocodingTool) Name() string { return "reverse_geocode" }

// Description describes what the tool does.
func (t *ReverseGeocodingTool) Description() string {
	return "Convert geographic coordinates (latitude, longitude) into location names. Returns up to 5 nearby location names for the given coordinates."
}

// Call calls the OpenWeatherMap Reverse Geocoding API and returns matching locations.
func (t *ReverseGeocodingTool) Call(ctx context.Context, input ReverseGeocodingToolInput) (ReverseGeocodingToolOutput, error) {
	if input.Lat == 0 && input.Lon == 0 {
		return ReverseGeocodingToolOutput{}, fmt.Errorf("parameters 'lat' and 'lon' must not both be zero")
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 5
	}

	endpoint := fmt.Sprintf("%s/reverse?lat=%f&lon=%f&limit=%d&appid=%s",
		t.baseURL, input.Lat, input.Lon, limit, t.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ReverseGeocodingToolOutput{}, fmt.Errorf("building reverse geocoding request: %w", err)
	}

	if err := t.limiter.Wait(ctx); err != nil {
		return ReverseGeocodingToolOutput{}, fmt.Errorf("rate limiter: %w", err)
	}

	resp, err := t.http.Do(req)
	if err != nil {
		return ReverseGeocodingToolOutput{}, fmt.Errorf("reverse geocoding API request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ReverseGeocodingToolOutput{}, fmt.Errorf("reverse geocoding API returned status %d", resp.StatusCode)
	}

	var results []geocodingResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return ReverseGeocodingToolOutput{}, fmt.Errorf("decoding reverse geocoding response: %w", err)
	}

	out := ReverseGeocodingToolOutput{
		Locations: make([]GeocodingLocation, 0, len(results)),
	}
	for _, r := range results {
		out.Locations = append(out.Locations, GeocodingLocation(r))
	}

	return out, nil
}
