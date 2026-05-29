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

func TestDistanceTimeTool_Metadata(t *testing.T) {
	tool := NewDistanceTimeTool(rate.NewLimiter(rate.Inf, 0))
	if tool.Name() != "distance_time" {
		t.Errorf("unexpected tool name: %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestDistanceTimeTool_Call_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		var req routeRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if req.Start != "1.909251,47.902964" {
			t.Errorf("unexpected start: %q", req.Start)
		}
		if req.End != "3.057256,50.62925" {
			t.Errorf("unexpected end: %q", req.End)
		}
		// Should NOT request steps or geometry.
		if req.GetSteps != "" {
			t.Errorf("expected no getSteps, got %q", req.GetSteps)
		}
		if req.GetGeometry != "" {
			t.Errorf("expected no getGeometry, got %q", req.GetGeometry)
		}

		resp := routeAPIResponse{
			Start:        "1.909251,47.902964",
			End:          "3.057256,50.62925",
			Profile:      "car",
			Optimization: "fastest",
			Distance:     296000.0,
			Duration:     10800.0,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newDistanceTimeToolWithBaseURL(srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), DistanceTimeToolInput{
		Start: "1.909251,47.902964",
		End:   "3.057256,50.62925",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Distance != 296000.0 {
		t.Errorf("unexpected distance: %f", result.Distance)
	}
	if result.Duration != 10800.0 {
		t.Errorf("unexpected duration: %f", result.Duration)
	}
	if result.Profile != "car" {
		t.Errorf("unexpected profile: %q", result.Profile)
	}
}

func TestDistanceTimeTool_Call_MissingStart(t *testing.T) {
	tool := NewDistanceTimeTool(rate.NewLimiter(rate.Inf, 0))
	_, err := tool.Call(context.Background(), DistanceTimeToolInput{End: "3.057256,50.62925"})
	if err == nil {
		t.Error("expected error for missing start")
	}
}

func TestDistanceTimeTool_Call_MissingEnd(t *testing.T) {
	tool := NewDistanceTimeTool(rate.NewLimiter(rate.Inf, 0))
	_, err := tool.Call(context.Background(), DistanceTimeToolInput{Start: "1.909251,47.902964"})
	if err == nil {
		t.Error("expected error for missing end")
	}
}

func TestDistanceTimeTool_Call_WithAvoidHighways(t *testing.T) {
	var receivedReq routeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedReq)

		resp := routeAPIResponse{
			Start: "1.909251,47.902964", End: "3.057256,50.62925",
			Profile: "car", Optimization: "fastest",
			Distance: 320000.0, Duration: 14400.0,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newDistanceTimeToolWithBaseURL(srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), DistanceTimeToolInput{
		Start:         "1.909251,47.902964",
		End:           "3.057256,50.62925",
		AvoidHighways: "true",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(receivedReq.Constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(receivedReq.Constraints))
	}
	if receivedReq.Constraints[0].Value != "autoroute" {
		t.Errorf("unexpected constraint value: %q", receivedReq.Constraints[0].Value)
	}
}
