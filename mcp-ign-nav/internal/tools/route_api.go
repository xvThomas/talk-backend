package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/time/rate"
)

// routeParams holds the common input parameters for a route API call.
type routeParams struct {
	Start         string
	End           string
	Resource      string
	Profile       string
	Optimization  string
	Intermediates []string
	AvoidHighways string
	GetSteps      bool
	GetGeometry   bool
}

// routeClient encapsulates the HTTP client and rate limiter for IGN API calls.
type routeClient struct {
	baseURL string
	http    *http.Client
	limiter *rate.Limiter
}

// newRouteClient creates a routeClient with the given base URL and limiter.
func newRouteClient(baseURL string, httpClient *http.Client, limiter *rate.Limiter) *routeClient {
	return &routeClient{baseURL: baseURL, http: httpClient, limiter: limiter}
}

// callRouteAPI calls the IGN /itineraire endpoint with the given parameters.
func (c *routeClient) callRouteAPI(ctx context.Context, params routeParams) (*routeAPIResponse, error) {
	if params.Start == "" {
		return nil, fmt.Errorf("parameter 'start' is required")
	}
	if params.End == "" {
		return nil, fmt.Errorf("parameter 'end' is required")
	}

	resource := params.Resource
	if resource == "" {
		resource = "bdtopo-osrm"
	}
	profile := params.Profile
	if profile == "" {
		profile = "car"
	}
	optimization := params.Optimization
	if optimization == "" {
		optimization = "fastest"
	}

	body := routeRequest{
		Start:        params.Start,
		End:          params.End,
		Resource:     resource,
		Profile:      profile,
		Optimization: optimization,
	}
	if params.GetSteps {
		body.GetSteps = "true"
		body.GetBbox = "true"
		body.GeometryFormat = "geojson"
	}
	if params.GetGeometry {
		body.GetGeometry = "true"
		body.GeometryFormat = "geojson"
	}
	if len(params.Intermediates) > 0 {
		body.Intermediates = params.Intermediates
	}
	if params.AvoidHighways == "true" {
		body.Constraints = []routeConstraint{{
			ConstraintType: "banned",
			Key:            "wayType",
			Operator:       "=",
			Value:          "autoroute",
		}}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/itineraire", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errResp routeErrorResponse
		if decErr := json.NewDecoder(resp.Body).Decode(&errResp); decErr == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result routeAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
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
	Start          string            `json:"start"`
	End            string            `json:"end"`
	Resource       string            `json:"resource"`
	Profile        string            `json:"profile"`
	Optimization   string            `json:"optimization"`
	GetSteps       string            `json:"getSteps,omitempty"`
	GetGeometry    string            `json:"getGeometry,omitempty"`
	GeometryFormat string            `json:"geometryFormat,omitempty"`
	GetBbox        string            `json:"getBbox,omitempty"`
	Intermediates  []string          `json:"intermediates,omitempty"`
	Constraints    []routeConstraint `json:"constraints,omitempty"`
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
	Geometry    *GeoJSONGeometry    `json:"geometry"`
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
