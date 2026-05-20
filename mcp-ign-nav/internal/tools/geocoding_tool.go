package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/xvThomas/LLMClientWrapper/talk-libs/mcpserver"
	"golang.org/x/time/rate"
)

const defaultSearchLimit = 5

// GeocodingToolInput is the typed input for the forward geocoding tool.
type GeocodingToolInput struct {
	Query    string  `json:"q" description:"Search text: address, place name, or location to geocode"`
	Index    string  `json:"index,omitempty" description:"Index to search: address, poi, or parcel. Defaults to address."`
	Limit    int     `json:"limit,omitempty" description:"Maximum number of results (1-50). Defaults to 5."`
	Postcode string  `json:"postcode,omitempty" description:"Filter by postal code (address and poi indexes)"`
	CityCode string  `json:"citycode,omitempty" description:"Filter by INSEE city code (address and poi indexes)"`
	Type     string  `json:"type,omitempty" description:"Filter by type: housenumber, street, locality, or municipality (address index only)"`
	Lon      float64 `json:"lon,omitempty" description:"Longitude to favor nearby results"`
	Lat      float64 `json:"lat,omitempty" description:"Latitude to favor nearby results"`
}

// GeocodingResult represents a single geocoded result from the IGN search API.
type GeocodingResult struct {
	Label       string  `json:"label" description:"Full formatted address or place name"`
	City        string  `json:"city" description:"City name"`
	Postcode    string  `json:"postcode,omitempty" description:"Postal code"`
	Street      string  `json:"street,omitempty" description:"Street name"`
	HouseNumber string  `json:"housenumber,omitempty" description:"House number"`
	Type        string  `json:"type" description:"Result type: housenumber, street, locality, or municipality"`
	Score       float64 `json:"score" description:"Relevance score (0 to 1)"`
	Lon         float64 `json:"lon" description:"Longitude of the result"`
	Lat         float64 `json:"lat" description:"Latitude of the result"`
	Context     string  `json:"context,omitempty" description:"Geographic context (department, region)"`
}

// GeocodingToolOutput is the typed output for the forward geocoding tool.
type GeocodingToolOutput struct {
	Results []GeocodingResult `json:"results" description:"List of matching locations"`
}

// GeocodingTool implements mcpserver.MCPTool for forward geocoding via the IGN Géoplateforme API.
type GeocodingTool struct {
	baseURL string
	http    *http.Client
	limiter *rate.Limiter
}

var _ mcpserver.MCPTool[GeocodingToolInput, GeocodingToolOutput] = (*GeocodingTool)(nil)

// NewGeocodingTool creates a GeocodingTool using the IGN Géoplateforme API.
func NewGeocodingTool(limiter *rate.Limiter) *GeocodingTool {
	return &GeocodingTool{
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: httpClientTimeout},
		limiter: limiter,
	}
}

// newGeocodingToolWithBaseURL creates a GeocodingTool with a custom base URL (for testing).
func newGeocodingToolWithBaseURL(baseURL string, client *http.Client) *GeocodingTool {
	return &GeocodingTool{baseURL: baseURL, http: client, limiter: rate.NewLimiter(rate.Inf, 0)}
}

// Name returns the tool name.
func (t *GeocodingTool) Name() string { return "geocode" }

// Description describes what the tool does.
func (t *GeocodingTool) Description() string {
	return "Search for a French address or place name and return matching locations with their coordinates using the IGN Géoplateforme geocoding API."
}

// Call performs forward geocoding by calling the IGN /search endpoint.
func (t *GeocodingTool) Call(ctx context.Context, input GeocodingToolInput) (GeocodingToolOutput, error) {
	if input.Query == "" {
		return GeocodingToolOutput{}, fmt.Errorf("parameter 'q' (search query) is required")
	}

	index := input.Index
	if index == "" {
		index = defaultIndex
	}

	limit := input.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}

	params := url.Values{}
	params.Set("q", input.Query)
	params.Set("index", index)
	params.Set("limit", strconv.Itoa(limit))
	params.Set("autocomplete", "0")

	if input.Postcode != "" {
		params.Set("postcode", input.Postcode)
	}
	if input.CityCode != "" {
		params.Set("citycode", input.CityCode)
	}
	if input.Type != "" {
		params.Set("type", input.Type)
	}
	if input.Lon != 0 || input.Lat != 0 {
		params.Set("lon", strconv.FormatFloat(input.Lon, 'f', -1, 64))
		params.Set("lat", strconv.FormatFloat(input.Lat, 'f', -1, 64))
	}

	endpoint := fmt.Sprintf("%s/search?%s", t.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return GeocodingToolOutput{}, fmt.Errorf("building request: %w", err)
	}

	if err := t.limiter.Wait(ctx); err != nil {
		return GeocodingToolOutput{}, fmt.Errorf("rate limiter: %w", err)
	}

	resp, err := t.http.Do(req)
	if err != nil {
		return GeocodingToolOutput{}, fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return GeocodingToolOutput{}, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var fc searchFeatureCollection
	if err := json.NewDecoder(resp.Body).Decode(&fc); err != nil {
		return GeocodingToolOutput{}, fmt.Errorf("decoding response: %w", err)
	}

	results := make([]GeocodingResult, 0, len(fc.Features))
	for _, f := range fc.Features {
		var lon, lat float64
		if len(f.Geometry.Coordinates) >= 2 {
			lon = f.Geometry.Coordinates[0]
			lat = f.Geometry.Coordinates[1]
		}
		results = append(results, GeocodingResult{
			Label:       f.Properties.Label,
			City:        f.Properties.City,
			Postcode:    f.Properties.Postcode,
			Street:      f.Properties.Street,
			HouseNumber: f.Properties.HouseNumber,
			Type:        f.Properties.Type,
			Score:       f.Properties.Score,
			Lon:         lon,
			Lat:         lat,
			Context:     f.Properties.Context,
		})
	}

	return GeocodingToolOutput{Results: results}, nil
}

// searchFeatureCollection represents the GeoJSON FeatureCollection returned by the /search endpoint.
type searchFeatureCollection struct {
	Type     string          `json:"type"`
	Features []searchFeature `json:"features"`
}

// searchFeature represents a single GeoJSON Feature from the search response.
type searchFeature struct {
	Type       string           `json:"type"`
	Geometry   searchGeometry   `json:"geometry"`
	Properties searchProperties `json:"properties"`
}

// searchGeometry holds the coordinates of a search result.
type searchGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

// searchProperties holds the properties of a geocoded search result.
type searchProperties struct {
	Label       string  `json:"label"`
	City        string  `json:"city"`
	Postcode    string  `json:"postcode"`
	Street      string  `json:"street"`
	HouseNumber string  `json:"housenumber"`
	Type        string  `json:"type"`
	Score       float64 `json:"score"`
	Context     string  `json:"context"`
}
