package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/xvThomas/LLMClientWrapper/mcp-owm/internal/ratelimit"

	"github.com/xvThomas/LLMClientWrapper/talk-libs/testutils"

	"github.com/joho/godotenv"
)

func TestReverseGeocodingTool_Metadata(t *testing.T) {
	tool := NewReverseGeocodingTool("key", ratelimit.Noop())
	if tool.Name() != "reverse_geocode" {
		t.Errorf("unexpected tool name: %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestReverseGeocodingTool_Call_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("lat") == "" || r.URL.Query().Get("lon") == "" {
			t.Error("expected lat and lon query parameters")
		}
		resp := []geocodingResponse{
			{Name: "City of London", Lat: 51.5128, Lon: -0.0918, Country: "GB", State: "England"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newReverseGeocodingToolWithBaseURL("testkey", srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), ReverseGeocodingToolInput{Lat: 51.5098, Lon: -0.1180})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Locations) != 1 {
		t.Fatalf("expected 1 location, got %d", len(result.Locations))
	}
	loc := result.Locations[0]
	if loc.Name != "City of London" {
		t.Errorf("expected Name %q, got %q", "City of London", loc.Name)
	}
	if loc.Country != "GB" {
		t.Errorf("expected Country %q, got %q", "GB", loc.Country)
	}
	if loc.State != "England" {
		t.Errorf("expected State %q, got %q", "England", loc.State)
	}
}

func TestReverseGeocodingTool_Call_WithLimit(t *testing.T) {
	var receivedLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]geocodingResponse{{Name: "Paris", Lat: 48.8566, Lon: 2.3522, Country: "FR"}})
	}))
	defer srv.Close()

	tool := newReverseGeocodingToolWithBaseURL("testkey", srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), ReverseGeocodingToolInput{Lat: 48.8566, Lon: 2.3522, Limit: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedLimit != "1" {
		t.Errorf("expected limit=1, got limit=%s", receivedLimit)
	}
}

func TestReverseGeocodingTool_Call_ZeroCoordinates(t *testing.T) {
	tool := NewReverseGeocodingTool("key", ratelimit.Noop())
	_, err := tool.Call(context.Background(), ReverseGeocodingToolInput{Lat: 0, Lon: 0})
	if err == nil {
		t.Error("expected error for zero coordinates")
	}
}

func TestReverseGeocodingTool_Call_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	tool := newReverseGeocodingToolWithBaseURL("badkey", srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), ReverseGeocodingToolInput{Lat: 48.8566, Lon: 2.3522})
	if err == nil {
		t.Error("expected error for non-200 API response")
	}
}

func TestReverseGeocodingTool_Integration(t *testing.T) {
	projectRoot := testutils.GetProjectRoot()
	_ = godotenv.Load(filepath.Join(projectRoot, ".env.test"))

	apiKey := os.Getenv("OPENWEATHERMAP_API_KEY")
	if apiKey == "" {
		t.Skip("OPENWEATHERMAP_API_KEY not set in .env.test, skipping integration test")
	}

	tool := NewReverseGeocodingTool(apiKey, ratelimit.Noop())
	result, err := tool.Call(context.Background(), ReverseGeocodingToolInput{Lat: 48.8566, Lon: 2.3522})
	if err != nil {
		t.Fatalf("integration call failed: %v", err)
	}
	if len(result.Locations) == 0 {
		t.Error("expected at least one location")
	}
	if result.Locations[0].Name == "" {
		t.Error("expected non-empty location name")
	}
}
