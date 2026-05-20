package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/time/rate"
)

func TestReverseGeocodingTool_Metadata(t *testing.T) {
	tool := NewReverseGeocodingTool(rate.NewLimiter(rate.Inf, 0))
	if tool.Name() != "reverse_geocode" {
		t.Errorf("unexpected tool name: %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestReverseGeocodingTool_Call_SingleCoordinate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("lon") == "" || r.URL.Query().Get("lat") == "" {
			t.Error("expected lon and lat query parameters")
		}
		if r.URL.Query().Get("index") != "address" {
			t.Errorf("expected index=address, got %q", r.URL.Query().Get("index"))
		}
		resp := featureCollection{
			Type: "FeatureCollection",
			Features: []ignFeature{
				{
					Type: "Feature",
					Properties: ignProperties{
						Label:       "10 Rue de Rivoli 75001 Paris",
						City:        "Paris",
						Postcode:    "75001",
						Street:      "Rue de Rivoli",
						HouseNumber: "10",
						Type:        "housenumber",
						Score:       0.95,
						Distance:    12.5,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newReverseGeocodingToolWithBaseURL(srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), ReverseGeocodingToolInput{
		Coordinates: []Coordinate{{Lon: 2.3522, Lat: 48.8566}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	r := result.Results[0]
	if r.Lon != 2.3522 || r.Lat != 48.8566 {
		t.Errorf("unexpected coordinates in result: (%f, %f)", r.Lon, r.Lat)
	}
	if len(r.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(r.Features))
	}
	f := r.Features[0]
	if f.Label != "10 Rue de Rivoli 75001 Paris" {
		t.Errorf("unexpected label: %q", f.Label)
	}
	if f.City != "Paris" {
		t.Errorf("unexpected city: %q", f.City)
	}
	if f.Postcode != "75001" {
		t.Errorf("unexpected postcode: %q", f.Postcode)
	}
	if f.Street != "Rue de Rivoli" {
		t.Errorf("unexpected street: %q", f.Street)
	}
	if f.HouseNumber != "10" {
		t.Errorf("unexpected housenumber: %q", f.HouseNumber)
	}
	if f.Distance != 12.5 {
		t.Errorf("unexpected distance: %f", f.Distance)
	}
}

func TestReverseGeocodingTool_Call_MultipleCoordinates(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		lon := r.URL.Query().Get("lon")
		var label string
		if lon[:4] == "2.35" {
			label = "Paris"
		} else {
			label = "Lyon"
		}
		resp := featureCollection{
			Type: "FeatureCollection",
			Features: []ignFeature{
				{Type: "Feature", Properties: ignProperties{Label: label, City: label, Type: "municipality", Score: 0.9, Distance: 5}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newReverseGeocodingToolWithBaseURL(srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), ReverseGeocodingToolInput{
		Coordinates: []Coordinate{
			{Lon: 2.3522, Lat: 48.8566},
			{Lon: 4.8357, Lat: 45.7640},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result.Results))
	}
	if result.Results[0].Features[0].Label != "Paris" {
		t.Errorf("expected Paris, got %q", result.Results[0].Features[0].Label)
	}
	if result.Results[1].Features[0].Label != "Lyon" {
		t.Errorf("expected Lyon, got %q", result.Results[1].Features[0].Label)
	}
}

func TestReverseGeocodingTool_Call_EmptyCoordinates(t *testing.T) {
	tool := NewReverseGeocodingTool(rate.NewLimiter(rate.Inf, 0))
	_, err := tool.Call(context.Background(), ReverseGeocodingToolInput{
		Coordinates: nil,
	})
	if err == nil {
		t.Error("expected error for empty coordinates")
	}
}

func TestReverseGeocodingTool_Call_TooManyCoordinates(t *testing.T) {
	tool := NewReverseGeocodingTool(rate.NewLimiter(rate.Inf, 0))
	coords := make([]Coordinate, 11)
	for i := range coords {
		coords[i] = Coordinate{Lon: float64(i), Lat: float64(i)}
	}
	_, err := tool.Call(context.Background(), ReverseGeocodingToolInput{
		Coordinates: coords,
	})
	if err == nil {
		t.Error("expected error for too many coordinates")
	}
}

func TestReverseGeocodingTool_Call_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tool := newReverseGeocodingToolWithBaseURL(srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), ReverseGeocodingToolInput{
		Coordinates: []Coordinate{{Lon: 2.3522, Lat: 48.8566}},
	})
	if err == nil {
		t.Error("expected error for API failure")
	}
}

func TestReverseGeocodingTool_Call_DefaultIndex(t *testing.T) {
	var receivedIndex string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedIndex = r.URL.Query().Get("index")
		resp := featureCollection{Type: "FeatureCollection", Features: []ignFeature{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newReverseGeocodingToolWithBaseURL(srv.URL, srv.Client())
	_, _ = tool.Call(context.Background(), ReverseGeocodingToolInput{
		Coordinates: []Coordinate{{Lon: 2.3522, Lat: 48.8566}},
	})
	if receivedIndex != "address" {
		t.Errorf("expected default index 'address', got %q", receivedIndex)
	}
}

func TestReverseGeocodingTool_Call_CustomLimit(t *testing.T) {
	var receivedLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedLimit = r.URL.Query().Get("limit")
		resp := featureCollection{Type: "FeatureCollection", Features: []ignFeature{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newReverseGeocodingToolWithBaseURL(srv.URL, srv.Client())
	_, _ = tool.Call(context.Background(), ReverseGeocodingToolInput{
		Coordinates: []Coordinate{{Lon: 2.3522, Lat: 48.8566}},
		Limit:       5,
	})
	if receivedLimit != "5" {
		t.Errorf("expected limit=5, got limit=%s", receivedLimit)
	}
}

func TestIntegration_ReverseGeocodingTool_Paris(t *testing.T) {
	tool := NewReverseGeocodingTool(rate.NewLimiter(rate.Inf, 0))
	result, err := tool.Call(context.Background(), ReverseGeocodingToolInput{
		Coordinates: []Coordinate{{Lon: 2.3522, Lat: 48.8566}},
		Limit:       1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	r := result.Results[0]
	if len(r.Features) == 0 {
		t.Fatal("expected at least 1 feature")
	}
	f := r.Features[0]
	if f.Label == "" {
		t.Error("expected non-empty label")
	}
	if f.City == "" {
		t.Error("expected non-empty city")
	}
	if f.Postcode == "" {
		t.Error("expected non-empty postcode")
	}
	t.Logf("Reverse geocoded (2.3522, 48.8566) -> %s", f.Label)
}

func TestIntegration_ReverseGeocodingTool_MultipleCoordinates(t *testing.T) {
	tool := NewReverseGeocodingTool(rate.NewLimiter(rate.Inf, 0))
	result, err := tool.Call(context.Background(), ReverseGeocodingToolInput{
		Coordinates: []Coordinate{
			{Lon: 2.3522, Lat: 48.8566},
			{Lon: 4.8357, Lat: 45.7640},
		},
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result.Results))
	}
	for i, r := range result.Results {
		if len(r.Features) == 0 {
			t.Errorf("result[%d]: expected at least 1 feature", i)
			continue
		}
		t.Logf("result[%d]: %s", i, r.Features[0].Label)
	}
}
