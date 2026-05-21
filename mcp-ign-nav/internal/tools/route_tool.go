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
	AvoidHighways string   `json:"avoidHighways,omitempty" description:"Set to 'true' to avoid highways (autoroutes). Defaults to empty (no avoidance)."`
}

// RouteStep represents a single navigation step (turn-by-turn instruction).
type RouteStep struct {
	Distance    float64 `json:"distance" description:"Distance of this step in meters"`
	Duration    float64 `json:"duration" description:"Duration of this step in seconds"`
	Instruction string  `json:"instruction" description:"Navigation instruction type (e.g. depart, turn, continue, arrive)"`
	Modifier    string  `json:"modifier,omitempty" description:"Instruction modifier (e.g. left, right, straight)"`
	Name        string  `json:"name,omitempty" description:"Road name for this step"`
	RoadNumber  string  `json:"roadNumber,omitempty" description:"Road classification number (e.g. D952, N7, A10)"`
	Toponyme    string  `json:"toponyme,omitempty" description:"Complementary place name or route name"`
}

// RoutePortion represents a portion of the route between two waypoints.
type RoutePortion struct {
	Start    string      `json:"start" description:"Start point of this portion"`
	End      string      `json:"end" description:"End point of this portion"`
	Distance float64     `json:"distance" description:"Distance of this portion in meters"`
	Duration float64     `json:"duration" description:"Duration of this portion in seconds"`
	Steps    []RouteStep `json:"steps" description:"Turn-by-turn navigation steps for this portion"`
}

// GeoJSONGeometry represents a GeoJSON geometry object.
type GeoJSONGeometry struct {
	Type        string      `json:"type" description:"GeoJSON geometry type (e.g. LineString)"`
	Coordinates [][]float64 `json:"coordinates" description:"Array of [longitude, latitude] coordinate pairs"`
}

// RouteToolOutput is the typed output for the route calculation tool.
type RouteToolOutput struct {
	Start        string           `json:"start" description:"Snapped start point"`
	End          string           `json:"end" description:"Snapped end point"`
	Profile      string           `json:"profile" description:"Routing profile used"`
	Optimization string           `json:"optimization" description:"Optimization criterion used"`
	Distance     float64          `json:"distance" description:"Total route distance in meters"`
	Duration     float64          `json:"duration" description:"Total route duration in seconds"`
	Bbox         [4]float64       `json:"bbox" description:"Bounding box [minLon, minLat, maxLon, maxLat]"`
	Geometry     *GeoJSONGeometry `json:"geometry" description:"Route geometry as a GeoJSON LineString"`
	Portions     []RoutePortion   `json:"portions" description:"Route portions between waypoints"`
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
		GetSteps:     "true",
		GetGeometry:  "true",
		GetBbox:      "true",
	}
	if len(input.Intermediates) > 0 {
		body.Intermediates = input.Intermediates
	}
	if input.AvoidHighways == "true" {
		body.Constraints = []routeConstraint{{
			ConstraintType: "banned",
			Key:            "wayType",
			Operator:       "=",
			Value:          "autoroute",
		}}
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
		steps := make([]RouteStep, 0, len(p.Steps))
		for _, s := range p.Steps {
			name := s.Attributes.Name.NomGauche
			if name == "" {
				name = s.Attributes.Name.NomDroite
			}
			steps = append(steps, RouteStep{
				Distance:    s.Distance,
				Duration:    s.Duration,
				Instruction: s.Instruction.Type,
				Modifier:    s.Instruction.Modifier,
				Name:        name,
				RoadNumber:  s.Attributes.Name.CpxNumero,
				Toponyme:    s.Attributes.Name.CpxToponyme,
			})
		}
		portions = append(portions, RoutePortion{
			Start:    p.Start,
			End:      p.End,
			Distance: p.Distance,
			Duration: p.Duration,
			Steps:    steps,
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
		Geometry:     result.Geometry,
		Portions:     portions,
	}, nil
}

// routeConstraint represents a routing constraint for the IGN API.
type routeConstraint struct {
	ConstraintType string `json:"constraintType"`
	Key            string `json:"key"`
	Operator       string `json:"operator"`
	Value          string `json:"value"`
}

// routeRequest is the JSON body sent to the IGN /itineraire endpoint.
type routeRequest struct {
	Start         string            `json:"start"`
	End           string            `json:"end"`
	Resource      string            `json:"resource"`
	Profile       string            `json:"profile"`
	Optimization  string            `json:"optimization"`
	GetSteps      string            `json:"getSteps"`
	GetGeometry   string            `json:"getGeometry"`
	GetBbox       string            `json:"getBbox,omitempty"`
	Intermediates []string          `json:"intermediates,omitempty"`
	Constraints   []routeConstraint `json:"constraints,omitempty"`
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
	Geometry     *GeoJSONGeometry  `json:"geometry"`
	Portions     []routeAPIPortion `json:"portions"`
}

type routeAPIPortion struct {
	Start    string         `json:"start"`
	End      string         `json:"end"`
	Distance float64        `json:"distance"`
	Duration float64        `json:"duration"`
	Steps    []routeAPIStep `json:"steps"`
}

type routeAPIStep struct {
	Distance    float64             `json:"distance"`
	Duration    float64             `json:"duration"`
	Instruction routeAPIInstruction `json:"instruction"`
	Attributes  routeAPIAttributes  `json:"attributes"`
}

type routeAPIInstruction struct {
	Type     string `json:"type"`
	Modifier string `json:"modifier"`
}

type routeAPIAttributes struct {
	Name routeAPIName `json:"name"`
}

type routeAPIName struct {
	NomGauche   string `json:"nom_1_gauche"`
	NomDroite   string `json:"nom_1_droite"`
	CpxNumero   string `json:"cpx_numero"`
	CpxToponyme string `json:"cpx_toponyme"`
}

// routeErrorResponse is the error format from the IGN navigation API.
type routeErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}
