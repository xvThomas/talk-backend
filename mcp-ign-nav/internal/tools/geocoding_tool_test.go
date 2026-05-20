package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeocodingTool_Metadata(t *testing.T) {
	tool := NewGeocodingTool()
	if tool.Name() != "geocode" {
		t.Errorf("unexpected tool name: %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestGeocodingTool_Call_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "10 rue de rivoli paris" {
			t.Errorf("unexpected query: %q", r.URL.Query().Get("q"))
		}
		if r.URL.Query().Get("index") != "address" {
			t.Errorf("unexpected index: %q", r.URL.Query().Get("index"))
		}
		if r.URL.Query().Get("autocomplete") != "0" {
			t.Errorf("expected autocomplete=0, got %q", r.URL.Query().Get("autocomplete"))
		}
		resp := searchFeatureCollection{
			Type: "FeatureCollection",
			Features: []searchFeature{
				{
					Type:     "Feature",
					Geometry: searchGeometry{Type: "Point", Coordinates: []float64{2.3622, 48.8555}},
					Properties: searchProperties{
						Label:       "10 Rue de Rivoli 75004 Paris",
						City:        "Paris",
						Postcode:    "75004",
						Street:      "Rue de Rivoli",
						HouseNumber: "10",
						Type:        "housenumber",
						Score:       0.95,
						Context:     "75, Paris, Île-de-France",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newGeocodingToolWithBaseURL(srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), GeocodingToolInput{
		Query: "10 rue de rivoli paris",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	r := result.Results[0]
	if r.Label != "10 Rue de Rivoli 75004 Paris" {
		t.Errorf("unexpected label: %q", r.Label)
	}
	if r.City != "Paris" {
		t.Errorf("unexpected city: %q", r.City)
	}
	if r.Lon != 2.3622 {
		t.Errorf("unexpected lon: %f", r.Lon)
	}
	if r.Lat != 48.8555 {
		t.Errorf("unexpected lat: %f", r.Lat)
	}
	if r.Context != "75, Paris, Île-de-France" {
		t.Errorf("unexpected context: %q", r.Context)
	}
}

func TestGeocodingTool_Call_EmptyQuery(t *testing.T) {
	tool := NewGeocodingTool()
	_, err := tool.Call(context.Background(), GeocodingToolInput{Query: ""})
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestGeocodingTool_Call_WithFilters(t *testing.T) {
	var receivedPostcode, receivedCityCode, receivedType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPostcode = r.URL.Query().Get("postcode")
		receivedCityCode = r.URL.Query().Get("citycode")
		receivedType = r.URL.Query().Get("type")
		resp := searchFeatureCollection{Type: "FeatureCollection", Features: []searchFeature{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newGeocodingToolWithBaseURL(srv.URL, srv.Client())
	_, _ = tool.Call(context.Background(), GeocodingToolInput{
		Query:    "rue nationale",
		Postcode: "75013",
		CityCode: "75113",
		Type:     "street",
	})
	if receivedPostcode != "75013" {
		t.Errorf("expected postcode=75013, got %q", receivedPostcode)
	}
	if receivedCityCode != "75113" {
		t.Errorf("expected citycode=75113, got %q", receivedCityCode)
	}
	if receivedType != "street" {
		t.Errorf("expected type=street, got %q", receivedType)
	}
}

func TestGeocodingTool_Call_WithProximity(t *testing.T) {
	var receivedLon, receivedLat string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedLon = r.URL.Query().Get("lon")
		receivedLat = r.URL.Query().Get("lat")
		resp := searchFeatureCollection{Type: "FeatureCollection", Features: []searchFeature{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newGeocodingToolWithBaseURL(srv.URL, srv.Client())
	_, _ = tool.Call(context.Background(), GeocodingToolInput{
		Query: "boulangerie",
		Lon:   2.3522,
		Lat:   48.8566,
	})
	if receivedLon == "" {
		t.Error("expected lon parameter to be set")
	}
	if receivedLat == "" {
		t.Error("expected lat parameter to be set")
	}
}

func TestGeocodingTool_Call_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	tool := newGeocodingToolWithBaseURL(srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), GeocodingToolInput{Query: "test"})
	if err == nil {
		t.Error("expected error for API failure")
	}
}

func TestGeocodingTool_Call_DefaultLimit(t *testing.T) {
	var receivedLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedLimit = r.URL.Query().Get("limit")
		resp := searchFeatureCollection{Type: "FeatureCollection", Features: []searchFeature{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newGeocodingToolWithBaseURL(srv.URL, srv.Client())
	_, _ = tool.Call(context.Background(), GeocodingToolInput{Query: "paris"})
	if receivedLimit != "5" {
		t.Errorf("expected default limit=5, got %q", receivedLimit)
	}
}

func TestIntegration_GeocodingTool_SearchAddress(t *testing.T) {
	tool := NewGeocodingTool()
	result, err := tool.Call(context.Background(), GeocodingToolInput{
		Query: "10 rue de rivoli paris",
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	r := result.Results[0]
	if r.Label == "" {
		t.Error("expected non-empty label")
	}
	if r.Lon == 0 && r.Lat == 0 {
		t.Error("expected non-zero coordinates")
	}
	t.Logf("Geocoded 'rue de rivoli paris' -> %s (%.6f, %.6f)", r.Label, r.Lon, r.Lat)
}

func TestIntegration_GeocodingTool_SearchWithPostcode(t *testing.T) {
	tool := NewGeocodingTool()
	result, err := tool.Call(context.Background(), GeocodingToolInput{
		Query:    "rue nationale",
		Postcode: "75013",
		Limit:    3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	for _, r := range result.Results {
		if r.Postcode != "75013" {
			t.Errorf("expected postcode 75013, got %q (label: %s)", r.Postcode, r.Label)
		}
	}
	t.Logf("Found %d results for 'rue nationale' in 75013", len(result.Results))
}
