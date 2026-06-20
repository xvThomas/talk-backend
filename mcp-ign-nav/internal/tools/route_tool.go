package tools

import (
	"context"
	"fmt"
	"net/http"

	"github.com/xvThomas/talk-backend/talk-libs/mcpserver"
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
	Start       string  `json:"start" description:"Start point of this step as 'longitude,latitude'"`
	End         string  `json:"end" description:"End point of this step as 'longitude,latitude'"`
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
	client      *routeClient
	getGeometry bool
}

var _ mcpserver.MCPTool[RouteToolInput, RouteToolOutput] = (*RouteTool)(nil)

// NewRouteTool creates a RouteTool using the IGN Navigation API.
// When getGeometry is true, the route GeoJSON geometry is requested and returned.
func NewRouteTool(limiter *rate.Limiter, getGeometry bool) *RouteTool {
	return &RouteTool{
		client:      newRouteClient(navigationBaseURL, &http.Client{Timeout: httpClientTimeout}, limiter),
		getGeometry: getGeometry,
	}
}

// newRouteToolWithBaseURL creates a RouteTool with a custom base URL (for testing).
func newRouteToolWithBaseURL(baseURL string, httpClient *http.Client, getGeometry bool) *RouteTool {
	return &RouteTool{
		client:      newRouteClient(baseURL, httpClient, rate.NewLimiter(rate.Inf, 0)),
		getGeometry: getGeometry,
	}
}

// Name returns the tool name.
func (t *RouteTool) Name() string { return "route" }

// Description describes what the tool does.
func (t *RouteTool) Description() string {
	return "Calculate a route between two points in France using the IGN Navigation API. Returns distance, duration, and route portions and detailed navigation steps. " +
		"IMPORTANT: 'start' and 'end' must be coordinates as 'longitude,latitude'. Use the 'geocode' tool first to convert city names or addresses into coordinates."
}

// Call performs route calculation by calling the IGN /itineraire endpoint.
func (t *RouteTool) Call(ctx context.Context, input RouteToolInput) (RouteToolOutput, error) {
	result, err := t.client.callRouteAPI(ctx, routeParams{
		Start:         input.Start,
		End:           input.End,
		Resource:      input.Resource,
		Profile:       input.Profile,
		Optimization:  input.Optimization,
		Intermediates: input.Intermediates,
		AvoidHighways: input.AvoidHighways,
		GetSteps:      true,
		GetGeometry:   t.getGeometry,
	})
	if err != nil {
		return RouteToolOutput{}, err
	}

	portions := make([]RoutePortion, 0, len(result.Portions))
	for _, p := range result.Portions {
		steps := make([]RouteStep, 0, len(p.Steps))
		for _, s := range p.Steps {
			name := s.Attributes.Name.NomGauche
			if name == "" {
				name = s.Attributes.Name.NomDroite
			}
			var stepStart, stepEnd string
			if s.Geometry != nil && len(s.Geometry.Coordinates) > 0 {
				first := s.Geometry.Coordinates[0]
				if len(first) >= 2 {
					stepStart = fmt.Sprintf("%g,%g", first[0], first[1])
				}
				last := s.Geometry.Coordinates[len(s.Geometry.Coordinates)-1]
				if len(last) >= 2 {
					stepEnd = fmt.Sprintf("%g,%g", last[0], last[1])
				}
			}
			steps = append(steps, RouteStep{
				Start:       stepStart,
				End:         stepEnd,
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

	var geometry *GeoJSONGeometry
	if t.getGeometry {
		geometry = result.Geometry
	}

	return RouteToolOutput{
		Start:        result.Start,
		End:          result.End,
		Profile:      result.Profile,
		Optimization: result.Optimization,
		Distance:     result.Distance,
		Duration:     result.Duration,
		Bbox:         bbox,
		Geometry:     geometry,
		Portions:     portions,
	}, nil
}
