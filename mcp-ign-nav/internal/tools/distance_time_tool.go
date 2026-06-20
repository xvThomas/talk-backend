package tools

import (
	"context"
	"net/http"

	"github.com/xvThomas/talk-backend/talk-libs/mcpserver"
	"golang.org/x/time/rate"
)

// DistanceTimeToolInput is the typed input for the distance/duration calculation tool.
type DistanceTimeToolInput struct {
	Start         string   `json:"start" description:"Start point as 'longitude,latitude' (WGS84). Example: '2.337306,48.849319'"`
	End           string   `json:"end" description:"End point as 'longitude,latitude' (WGS84). Example: '2.367776,48.852891'"`
	Profile       string   `json:"profile,omitempty" description:"Routing profile: 'car' or 'pedestrian'. Defaults to 'car'."`
	Optimization  string   `json:"optimization,omitempty" description:"Optimization criterion: 'fastest' or 'shortest'. Defaults to 'fastest'."`
	Intermediates []string `json:"intermediates,omitempty" description:"Ordered list of intermediate waypoints as 'longitude,latitude' strings"`
	AvoidHighways string   `json:"avoidHighways,omitempty" description:"Set to 'true' to avoid highways (autoroutes). Defaults to empty (no avoidance)."`
}

// DistanceTimeToolOutput is the typed output returning only distance and duration.
type DistanceTimeToolOutput struct {
	Start        string  `json:"start" description:"Snapped start point"`
	End          string  `json:"end" description:"Snapped end point"`
	Profile      string  `json:"profile" description:"Routing profile used"`
	Optimization string  `json:"optimization" description:"Optimization criterion used"`
	Distance     float64 `json:"distance" description:"Total route distance in meters"`
	Duration     float64 `json:"duration" description:"Total route duration in seconds"`
}

// DistanceTimeTool implements mcpserver.MCPTool for lightweight distance/duration calculation.
type DistanceTimeTool struct {
	client *routeClient
}

var _ mcpserver.MCPTool[DistanceTimeToolInput, DistanceTimeToolOutput] = (*DistanceTimeTool)(nil)

// NewDistanceTimeTool creates a DistanceTimeTool using the IGN Navigation API.
func NewDistanceTimeTool(limiter *rate.Limiter) *DistanceTimeTool {
	return &DistanceTimeTool{
		client: newRouteClient(navigationBaseURL, &http.Client{Timeout: httpClientTimeout}, limiter),
	}
}

// newDistanceTimeToolWithBaseURL creates a DistanceTimeTool with a custom base URL (for testing).
func newDistanceTimeToolWithBaseURL(baseURL string, httpClient *http.Client) *DistanceTimeTool {
	return &DistanceTimeTool{
		client: newRouteClient(baseURL, httpClient, rate.NewLimiter(rate.Inf, 0)),
	}
}

// Name returns the tool name.
func (t *DistanceTimeTool) Name() string { return "distance_time" }

// Description describes what the tool does.
func (t *DistanceTimeTool) Description() string {
	return "Calculate the distance and travel time between two points in France. Returns only total distance and duration (no steps or geometry). " +
		"Use this tool when the user only needs distance or travel time. " +
		"IMPORTANT: 'start' and 'end' must be coordinates as 'longitude,latitude'. Use the 'geocode' tool first to convert city names or addresses into coordinates."
}

// Call performs distance/duration calculation via the IGN /itineraire endpoint.
func (t *DistanceTimeTool) Call(ctx context.Context, input DistanceTimeToolInput) (DistanceTimeToolOutput, error) {
	result, err := t.client.callRouteAPI(ctx, routeParams{
		Start:         input.Start,
		End:           input.End,
		Profile:       input.Profile,
		Optimization:  input.Optimization,
		Intermediates: input.Intermediates,
		AvoidHighways: input.AvoidHighways,
		GetSteps:      false,
		GetGeometry:   false,
	})
	if err != nil {
		return DistanceTimeToolOutput{}, err
	}

	return DistanceTimeToolOutput{
		Start:        result.Start,
		End:          result.End,
		Profile:      result.Profile,
		Optimization: result.Optimization,
		Distance:     result.Distance,
		Duration:     result.Duration,
	}, nil
}
