package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/xvThomas/LLMClientWrapper/talk-libs/mcpserver"
	"golang.org/x/time/rate"
)

const navigationBaseURL = "https://data.geopf.fr/navigation"

// RouteToolInput is the typed input for the route calculation tool.
type RouteToolInput struct {
	Start         string   `json:"start" description:"Start point as 'longitude,latitude' (WGS84). Example: '2.337306,48.849319'"`
	End           string   `json:"end" description:"End point as 'longitude,latitude' (WGS84). Example: '2.367776,48.852891'"`
	Resource      string   `json:"resource,omitempty" description:"Routing resource to use. Defaults to 'bdtopo-osrm'. Other option: 'bdtopo-pgr'."`
	Profile       string   `json:"profile,omitempty" description:"Routing profile: 'car' or 'pedestrian'. Defaults to 'car'."`
	Optimization  string   `json:"optimization,omitempty" description:"Optimization criterion: 'fastest' or 'shortest'. Defaults to 'fastest'."`
	Intermediates []string `json:"intermediates,omitempty" description:"Ordered list of intermediate waypoints as 'longitude,latitude' strings"`
}

// RoutePortion represents a portion of the route between two waypoints.
type RoutePortion struct {
	Start    string  `json:"start" description:"Start point of this portion"`
	End      string  `json:"end" description:"End point of this portion"`
	Distance float64 `json:"distance" description:"Distance of this portion in meters"`
	Duration float64 `json:"duration" description:"Duration of this portion in seconds"`
}

// RouteToolOutput is the typed output for the route calculation tool.
type RouteToolOutput struct {
	Start        string         `json:"start" description:"Snapped start point"`
	End          string         `json:"end" description:"Snapped end point"`
	Profile      string         `json:"profile" description:"Routing profile used"`
	Optimization string         `json:"optimization" description:"Optimization criterion used"`
	Distance     float64        `json:"distance" description:"Total route distance in meters"`
	Duration     float64        `json:"duration" description:"Total route duration in seconds"`
	Bbox         [4]float64     `json:"bbox" description:"Bounding box [minLon, minLat, maxLon, maxLat]"`
	Portions     []RoutePortion `json:"portions" description:"Route portions between waypoints"`
}

// RouteTool implements mcpserver.MCPTool for route calculation via the IGN Navigation API.
type RouteTool struct {
	baseURL string
	http    *http.Client
	limiter *rate.Limiter
}

var _ mcpserver.MCPTool[RouteToolInput, RouteToolOutput] = (*RouteTool)(nil)

// NewRouteTool creates a RouteTool using the IGN Navigation API.
func NewRouteTool(limiter *rate.Limiter) *RouteTool {
	return &RouteTool{
		baseURL: navigationBaseURL,
		http:    &http.Client{Timeout: httpClientTimeout},
		limiter: limiter,
	}
}

// newRouteToolWithBaseURL creates a RouteTool with a custom base URL (for testing).
func newRouteToolWithBaseURL(baseURL string, client *http.Client) *RouteTool {
	return &RouteTool{baseURL: baseURL, http: client, limiter: rate.NewLimiter(rate.Inf, 0)}
}

// Name returns the tool name.
func (t *RouteTool) Name() string { return "route" }

// Description describes what the tool does.
func (t *RouteTool) Description() string {
	return "Calculate a route between two points in France using the IGN Navigation API. Returns distance, duration, and route portions."
}

// Call performs route calculation by calling the IGN /itineraire endpoint.
func (t *RouteTool) Call(ctx context.Context, input RouteToolInput) (RouteToolOutput, error) {
	if input.Start == "" {
		return RouteToolOutput{}, fmt.Errorf("parameter 'start' is required")
	}
	if input.End == "" {
		return RouteToolOutput{}, fmt.Errorf("parameter 'end' is required")
	}

	resource := input.Resource
	if resource == "" {
		resource = "bdtopo-osrm"
	}
	profile := input.Profile
	if profile == "" {
		profile = "car"
	}
	optimization := input.Optimization
	if optimization == "" {
		optimization = "fastest"
	}

	body := routeRequest{
		Start:        input.Start,
		End:          input.End,
		Resource:     resource,
		Profile:      profile,
		Optimization: optimization,
	}
	if len(input.Intermediates) > 0 {
		body.Intermediates = input.Intermediates
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return RouteToolOutput{}, fmt.Errorf("marshaling request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/itineraire", t.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return RouteToolOutput{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if err := t.limiter.Wait(ctx); err != nil {
		return RouteToolOutput{}, fmt.Errorf("rate limiter: %w", err)
	}

	resp, err := t.http.Do(req)
	if err != nil {
		return RouteToolOutput{}, fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errResp routeErrorResponse
		if decErr := json.NewDecoder(resp.Body).Decode(&errResp); decErr == nil && errResp.Error.Message != "" {
			return RouteToolOutput{}, fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return RouteToolOutput{}, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result routeAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return RouteToolOutput{}, fmt.Errorf("decoding response: %w", err)
	}

	portions := make([]RoutePortion, 0, len(result.Portions))
	for _, p := range result.Portions {
		portions = append(portions, RoutePortion{
			Start:    p.Start,
			End:      p.End,
			Distance: p.Distance,
			Duration: p.Duration,
		})
	}

	var bbox [4]float64
	if len(result.Bbox) == 4 {
		copy(bbox[:], result.Bbox)
	}

	return RouteToolOutput{
		Start:        result.Start,
		End:          result.End,
		Profile:      result.Profile,
		Optimization: result.Optimization,
		Distance:     result.Distance,
		Duration:     result.Duration,
		Bbox:         bbox,
		Portions:     portions,
	}, nil
}

// routeRequest is the JSON body sent to the IGN /itineraire endpoint.
type routeRequest struct {
	Start         string   `json:"start"`
	End           string   `json:"end"`
	Resource      string   `json:"resource"`
	Profile       string   `json:"profile"`
	Optimization  string   `json:"optimization"`
	Intermediates []string `json:"intermediates,omitempty"`
}

// routeAPIResponse is the raw JSON response from the IGN /itineraire endpoint.
type routeAPIResponse struct {
	Start        string            `json:"start"`
	End          string            `json:"end"`
	Profile      string            `json:"profile"`
	Optimization string            `json:"optimization"`
	Distance     float64           `json:"distance"`
	Duration     float64           `json:"duration"`
	Bbox         []float64         `json:"bbox"`
	Portions     []routeAPIPortion `json:"portions"`
}

type routeAPIPortion struct {
	Start    string  `json:"start"`
	End      string  `json:"end"`
	Distance float64 `json:"distance"`
	Duration float64 `json:"duration"`
}

// routeErrorResponse is the error format from the IGN navigation API.
type routeErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}
