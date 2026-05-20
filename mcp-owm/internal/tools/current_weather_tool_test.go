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

func TestCurrentWeatherTool_Metadata(t *testing.T) {
	tool := NewCurrentWeatherTool("key")
	if tool.Name() != "get_current_weather" {
		t.Errorf("unexpected tool name: %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestCurrentWeatherTool_Call_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := weatherResponse{Name: "Paris", ID: 2988507}
		resp.Coord.Lon = 2.3488
		resp.Coord.Lat = 48.8534
		resp.Base = "stations"
		resp.Main.Temp = 18.5
		resp.Main.FeelsLike = 17.8
		resp.Main.TempMin = 16.0
		resp.Main.TempMax = 20.0
		resp.Main.Pressure = 1013
		resp.Main.Humidity = 72
		resp.Weather = []struct {
			ID          int    `json:"id"`
			Main        string `json:"main"`
			Description string `json:"description"`
			Icon        string `json:"icon"`
		}{{ID: 800, Main: "Clear", Description: "clear sky", Icon: "01d"}}
		resp.Visibility = 10000
		resp.Wind.Speed = 3.5
		resp.Wind.Deg = 210
		resp.Clouds.All = 5
		resp.Sys.Country = "FR"
		resp.Sys.Sunrise = 1711341600
		resp.Sys.Sunset = 1711387200
		resp.Timezone = 3600
		resp.Dt = 1711360800
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newCurrentWeatherToolWithBaseURL("testkey", srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), CurrentWeatherToolInput{Lat: 48.8534, Lon: 2.3488})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "Paris" {
		t.Errorf("expected Name %q, got %q", "Paris", result.Name)
	}
	if result.Temp != 18.5 {
		t.Errorf("expected Temp 18.5, got %f", result.Temp)
	}
	if result.FeelsLike != 17.8 {
		t.Errorf("expected FeelsLike 17.8, got %f", result.FeelsLike)
	}
	if result.Humidity != 72 {
		t.Errorf("expected Humidity 72, got %d", result.Humidity)
	}
	if len(result.Weather) != 1 || result.Weather[0].Description != "clear sky" {
		t.Errorf("unexpected Weather: %v", result.Weather)
	}
	if result.Weather[0].Main != "Clear" {
		t.Errorf("expected Weather[0].Main %q, got %q", "Clear", result.Weather[0].Main)
	}
	if result.Visibility != 10000 {
		t.Errorf("expected Visibility 10000, got %d", result.Visibility)
	}
	if result.WindSpeed != 3.5 {
		t.Errorf("expected WindSpeed 3.5, got %f", result.WindSpeed)
	}
	if result.WindDeg != 210 {
		t.Errorf("expected WindDeg 210, got %d", result.WindDeg)
	}
	if result.Cloudiness != 5 {
		t.Errorf("expected Cloudiness 5, got %d", result.Cloudiness)
	}
	if result.Sys.Country != "FR" {
		t.Errorf("expected Sys.Country %q, got %q", "FR", result.Sys.Country)
	}
	if result.Sys.Sunrise != 1711341600 {
		t.Errorf("expected Sys.Sunrise 1711341600, got %d", result.Sys.Sunrise)
	}
	if result.Coord.Lat != 48.8534 {
		t.Errorf("expected Coord.Lat 48.8534, got %f", result.Coord.Lat)
	}
	if result.DateTime != "2024-03-25T10:00:00Z" {
		t.Errorf("expected DateTime %q, got %q", "2024-03-25T10:00:00Z", result.DateTime)
	}
	if result.Precipitation != nil {
		t.Errorf("expected Precipitation to be nil, got %v", result.Precipitation)
	}
}

func TestCurrentWeatherTool_Call_EmptyCity(t *testing.T) {
	tool := NewCurrentWeatherTool("key")
	_, err := tool.Call(context.Background(), CurrentWeatherToolInput{Lat: 0, Lon: 0})
	if err == nil {
		t.Error("expected error for zero coordinates")
	}
}

func TestCurrentWeatherTool_Call_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	tool := newCurrentWeatherToolWithBaseURL("badkey", srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), CurrentWeatherToolInput{Lat: 48.8534, Lon: 2.3488})
	if err == nil {
		t.Error("expected error for non-200 API response")
	}
}

func TestCurrentWeatherTool_Call_WithPrecipitation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload := `{"name":"London","main":{"temp":10.0},"weather":[{"id":500,"main":"Rain","description":"light rain","icon":"10d"}],"rain":{"1h":1.5},"dt":1711360800}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	tool := newCurrentWeatherToolWithBaseURL("testkey", srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), CurrentWeatherToolInput{Lat: 51.5085, Lon: -0.1257})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Precipitation == nil {
		t.Fatal("expected Precipitation to be non-nil")
	}
	if *result.Precipitation != 1.5 {
		t.Errorf("expected Precipitation 1.5, got %f", *result.Precipitation)
	}
	if result.Snow != nil {
		t.Errorf("expected Snow to be nil, got %v", result.Snow)
	}
}

func TestCurrentWeatherTool_Integration(t *testing.T) {
	// Load .env.test from the project root (4 levels up from this package directory).
	// godotenv.Load does not override variables that are already set in the environment.
	projectRoot := testutils.GetProjectRoot()
	_ = godotenv.Load(
		filepath.Join(projectRoot, ".env.test"),
	)

	apiKey := os.Getenv("OPENWEATHERMAP_API_KEY")
	if apiKey == "" {
		t.Skip("OPENWEATHERMAP_API_KEY not set in .env.test, skipping integration test")
	}

	tool := NewCurrentWeatherTool(apiKey)
	result, err := tool.Call(context.Background(), CurrentWeatherToolInput{Lat: 48.8566, Lon: 2.3522})
	if err != nil {
		t.Fatalf("integration call failed: %v", err)
	}
	if result.Name == "" {
		t.Error("expected non-empty city name")
	}
	if len(result.Weather) == 0 {
		t.Error("expected at least one weather condition")
	}
	if result.Coord.Lat == 0 && result.Coord.Lon == 0 {
		t.Error("expected non-zero coordinates for Paris")
	}
	if result.DateTime == "" {
		t.Error("expected non-empty DateTime")
	}
	if result.Sys.Country == "" {
		t.Error("expected non-empty country code")
	}
}
