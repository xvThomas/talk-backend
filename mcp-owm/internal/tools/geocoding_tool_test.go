package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/xvThomas/LLMClientWrapper/talk-libs/testutils"

	"github.com/joho/godotenv"
)

func TestGeocodingTool_Metadata(t *testing.T) {
	tool := NewGeocodingTool("key")
	if tool.Name() != "geocode" {
		t.Errorf("unexpected tool name: %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestGeocodingTool_Call_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "London" {
			t.Errorf("expected q=London, got q=%s", r.URL.Query().Get("q"))
		}
		resp := []geocodingResponse{
			{Name: "London", Lat: 51.5085, Lon: -0.1257, Country: "GB", State: "England"},
			{Name: "London", Lat: 42.9834, Lon: -81.2330, Country: "CA", State: "Ontario"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newGeocodingToolWithBaseURL("testkey", srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), GeocodingToolInput{City: "London"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Locations) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(result.Locations))
	}
	loc := result.Locations[0]
	if loc.Name != "London" {
		t.Errorf("expected Name %q, got %q", "London", loc.Name)
	}
	if loc.Lat != 51.5085 {
		t.Errorf("expected Lat 51.5085, got %f", loc.Lat)
	}
	if loc.Lon != -0.1257 {
		t.Errorf("expected Lon -0.1257, got %f", loc.Lon)
	}
	if loc.Country != "GB" {
		t.Errorf("expected Country %q, got %q", "GB", loc.Country)
	}
	if loc.State != "England" {
		t.Errorf("expected State %q, got %q", "England", loc.State)
	}
}

func TestGeocodingTool_Call_WithLimit(t *testing.T) {
	var receivedLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]geocodingResponse{{Name: "Paris", Lat: 48.8566, Lon: 2.3522, Country: "FR"}})
	}))
	defer srv.Close()

	tool := newGeocodingToolWithBaseURL("testkey", srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), GeocodingToolInput{City: "Paris", Limit: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedLimit != "1" {
		t.Errorf("expected limit=1, got limit=%s", receivedLimit)
	}
}

func TestGeocodingTool_Call_EmptyCity(t *testing.T) {
	tool := NewGeocodingTool("key")
	_, err := tool.Call(context.Background(), GeocodingToolInput{City: ""})
	if err == nil {
		t.Error("expected error for empty city")
	}
}

func TestGeocodingTool_Call_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	tool := newGeocodingToolWithBaseURL("badkey", srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), GeocodingToolInput{City: "Paris"})
	if err == nil {
		t.Error("expected error for non-200 API response")
	}
}

func TestGeocodingTool_Integration(t *testing.T) {
	projectRoot := testutils.GetProjectRoot()
	_ = godotenv.Load(filepath.Join(projectRoot, ".env.test"))

	apiKey := os.Getenv("OPENWEATHERMAP_API_KEY")
	if apiKey == "" {
		t.Skip("OPENWEATHERMAP_API_KEY not set in .env.test, skipping integration test")
	}

	tool := NewGeocodingTool(apiKey)
	result, err := tool.Call(context.Background(), GeocodingToolInput{City: "Paris"})
	if err != nil {
		t.Fatalf("integration call failed: %v", err)
	}
	if len(result.Locations) == 0 {
		t.Error("expected at least one location")
	}
	if result.Locations[0].Country == "" {
		t.Error("expected non-empty country code")
	}
}
