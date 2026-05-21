package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/time/rate"
)

func TestRouteTool_Metadata(t *testing.T) {
	tool := NewRouteTool(rate.NewLimiter(rate.Inf, 0))
	if tool.Name() != "route" {
		t.Errorf("unexpected tool name: %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestRouteTool_Call_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content-type: %q", r.Header.Get("Content-Type"))
		}

		body, _ := io.ReadAll(r.Body)
		var req routeRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if req.Start != "2.337306,48.849319" {
			t.Errorf("unexpected start: %q", req.Start)
		}
		if req.End != "2.367776,48.852891" {
			t.Errorf("unexpected end: %q", req.End)
		}
		if req.Resource != "bdtopo-osrm" {
			t.Errorf("unexpected resource: %q", req.Resource)
		}
		if req.Profile != "car" {
			t.Errorf("unexpected profile: %q", req.Profile)
		}
		if req.Optimization != "fastest" {
			t.Errorf("unexpected optimization: %q", req.Optimization)
		}
		if req.GetSteps != "true" {
			t.Errorf("expected getSteps 'true', got %q", req.GetSteps)
		}

		resp := routeAPIResponse{
			Start:        "2.337325,48.84932",
			End:          "2.367842,48.85278",
			Profile:      "car",
			Optimization: "fastest",
			Distance:     2562.9,
			Duration:     581.1,
			Bbox:         []float64{2.337325, 48.848823, 2.367842, 48.85278},
			Portions: []routeAPIPortion{
				{
					Start:    "2.337325,48.84932",
					End:      "2.367842,48.85278",
					Distance: 2562.9,
					Duration: 581.1,
					Steps: []routeAPIStep{
						{
							Distance:    2.5,
							Duration:    0.3,
							Instruction: routeAPIInstruction{Type: "depart"},
							Attributes:  routeAPIAttributes{Name: routeAPIName{NomGauche: "R DE TOURNON"}},
						},
						{
							Distance:    106.8,
							Duration:    30.4,
							Instruction: routeAPIInstruction{Type: "turn", Modifier: "left"},
							Attributes:  routeAPIAttributes{Name: routeAPIName{NomGauche: "R DE VAUGIRARD", CpxNumero: "D952", CpxToponyme: "Route Nationale"}},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newRouteToolWithBaseURL(srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), RouteToolInput{
		Start: "2.337306,48.849319",
		End:   "2.367776,48.852891",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Distance != 2562.9 {
		t.Errorf("unexpected distance: %f", result.Distance)
	}
	if result.Duration != 581.1 {
		t.Errorf("unexpected duration: %f", result.Duration)
	}
	if result.Profile != "car" {
		t.Errorf("unexpected profile: %q", result.Profile)
	}
	if result.Optimization != "fastest" {
		t.Errorf("unexpected optimization: %q", result.Optimization)
	}
	if len(result.Portions) != 1 {
		t.Fatalf("expected 1 portion, got %d", len(result.Portions))
	}
	if result.Portions[0].Distance != 2562.9 {
		t.Errorf("unexpected portion distance: %f", result.Portions[0].Distance)
	}
	if len(result.Portions[0].Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result.Portions[0].Steps))
	}
	if result.Portions[0].Steps[0].Instruction != "depart" {
		t.Errorf("unexpected step instruction: %q", result.Portions[0].Steps[0].Instruction)
	}
	if result.Portions[0].Steps[0].Name != "R DE TOURNON" {
		t.Errorf("unexpected step name: %q", result.Portions[0].Steps[0].Name)
	}
	if result.Portions[0].Steps[1].Modifier != "left" {
		t.Errorf("unexpected step modifier: %q", result.Portions[0].Steps[1].Modifier)
	}
	if result.Portions[0].Steps[1].RoadNumber != "D952" {
		t.Errorf("unexpected step road number: %q", result.Portions[0].Steps[1].RoadNumber)
	}
	if result.Portions[0].Steps[1].Toponyme != "Route Nationale" {
		t.Errorf("unexpected step toponyme: %q", result.Portions[0].Steps[1].Toponyme)
	}
}

func TestRouteTool_Call_WithIntermediates(t *testing.T) {
	var receivedReq routeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedReq)

		resp := routeAPIResponse{
			Start:        "2.337325,48.84932",
			End:          "2.367842,48.85278",
			Profile:      "car",
			Optimization: "fastest",
			Distance:     2563.0,
			Duration:     581.1,
			Bbox:         []float64{2.337325, 48.848823, 2.367842, 48.85278},
			Portions: []routeAPIPortion{
				{Start: "2.337325,48.84932", End: "2.349861,48.84976", Distance: 1159.4, Duration: 280.7},
				{Start: "2.349861,48.84976", End: "2.367842,48.85278", Distance: 1403.6, Duration: 300.4},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newRouteToolWithBaseURL(srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), RouteToolInput{
		Start:         "2.337306,48.849319",
		End:           "2.367776,48.852891",
		Intermediates: []string{"2.350000,48.850000"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(receivedReq.Intermediates) != 1 || receivedReq.Intermediates[0] != "2.350000,48.850000" {
		t.Errorf("unexpected intermediates in request: %v", receivedReq.Intermediates)
	}
	if len(result.Portions) != 2 {
		t.Fatalf("expected 2 portions, got %d", len(result.Portions))
	}
}

func TestRouteTool_Call_MissingStart(t *testing.T) {
	tool := NewRouteTool(rate.NewLimiter(rate.Inf, 0))
	_, err := tool.Call(context.Background(), RouteToolInput{End: "2.367776,48.852891"})
	if err == nil {
		t.Error("expected error for missing start")
	}
}

func TestRouteTool_Call_MissingEnd(t *testing.T) {
	tool := NewRouteTool(rate.NewLimiter(rate.Inf, 0))
	_, err := tool.Call(context.Background(), RouteToolInput{Start: "2.337306,48.849319"})
	if err == nil {
		t.Error("expected error for missing end")
	}
}

func TestRouteTool_Call_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		resp := routeErrorResponse{}
		resp.Error.Message = "Parameter 'profile' is invalid"
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newRouteToolWithBaseURL(srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), RouteToolInput{
		Start: "2.337306,48.849319",
		End:   "2.367776,48.852891",
	})
	if err == nil {
		t.Error("expected error for API error response")
	}
}

func TestRouteTool_Call_Defaults(t *testing.T) {
	var receivedReq routeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedReq)

		resp := routeAPIResponse{
			Start: "2.337325,48.84932", End: "2.367842,48.85278",
			Profile: "car", Optimization: "fastest",
			Distance: 100, Duration: 50, Bbox: []float64{0, 0, 1, 1},
			Portions: []routeAPIPortion{{Start: "a", End: "b", Distance: 100, Duration: 50}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newRouteToolWithBaseURL(srv.URL, srv.Client())
	_, _ = tool.Call(context.Background(), RouteToolInput{
		Start: "2.337306,48.849319",
		End:   "2.367776,48.852891",
	})

	if receivedReq.Resource != "bdtopo-osrm" {
		t.Errorf("expected default resource 'bdtopo-osrm', got %q", receivedReq.Resource)
	}
	if receivedReq.Profile != "car" {
		t.Errorf("expected default profile 'car', got %q", receivedReq.Profile)
	}
	if receivedReq.Optimization != "fastest" {
		t.Errorf("expected default optimization 'fastest', got %q", receivedReq.Optimization)
	}
}
