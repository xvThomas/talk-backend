package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/xvThomas/LLMClientWrapper/mcp-owm/internal/ratelimit"

	"github.com/xvThomas/LLMClientWrapper/talk-libs/testutils"

	"github.com/joho/godotenv"
)

func TestAirPollutionTool_Metadata(t *testing.T) {
	tool := NewAirPollutionTool("key", ratelimit.Noop())
	if tool.Name() != "get_current_air_pollution" {
		t.Errorf("unexpected tool name: %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestAirPollutionTool_Call_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload := `{
			"coord": [48.8534, 2.3488],
			"list": [{
				"dt": 1711360800,
				"main": {"aqi": 2},
				"components": {
					"co": 270.37,
					"no": 5.87,
					"no2": 43.18,
					"o3": 4.78,
					"so2": 14.54,
					"pm2_5": 13.45,
					"pm10": 15.52,
					"nh3": 0.29
				}
			}]
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	tool := newAirPollutionToolWithBaseURL("testkey", srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), AirPollutionToolInput{Lat: 48.8534, Lon: 2.3488})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AQI != 2 {
		t.Errorf("expected AQI 2, got %d", result.AQI)
	}
	if result.DateTime == "" {
		t.Error("expected non-empty DateTime")
	}
	if result.Components.CO != 270.37 {
		t.Errorf("expected CO 270.37, got %f", result.Components.CO)
	}
	if result.Components.NO != 5.87 {
		t.Errorf("expected NO 5.87, got %f", result.Components.NO)
	}
	if result.Components.NO2 != 43.18 {
		t.Errorf("expected NO2 43.18, got %f", result.Components.NO2)
	}
	if result.Components.O3 != 4.78 {
		t.Errorf("expected O3 4.78, got %f", result.Components.O3)
	}
	if result.Components.SO2 != 14.54 {
		t.Errorf("expected SO2 14.54, got %f", result.Components.SO2)
	}
	if result.Components.PM25 != 13.45 {
		t.Errorf("expected PM2.5 13.45, got %f", result.Components.PM25)
	}
	if result.Components.PM10 != 15.52 {
		t.Errorf("expected PM10 15.52, got %f", result.Components.PM10)
	}
	if result.Components.NH3 != 0.29 {
		t.Errorf("expected NH3 0.29, got %f", result.Components.NH3)
	}
}

func TestAirPollutionTool_Call_ZeroCoordinates(t *testing.T) {
	tool := NewAirPollutionTool("key", ratelimit.Noop())
	_, err := tool.Call(context.Background(), AirPollutionToolInput{Lat: 0, Lon: 0})
	if err == nil {
		t.Error("expected error for zero coordinates")
	}
}

func TestAirPollutionTool_Call_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	tool := newAirPollutionToolWithBaseURL("badkey", srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), AirPollutionToolInput{Lat: 48.8534, Lon: 2.3488})
	if err == nil {
		t.Error("expected error for non-200 API response")
	}
}

func TestAirPollutionTool_Call_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload := `{"coord": [48.8534, 2.3488], "list": []}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	tool := newAirPollutionToolWithBaseURL("testkey", srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), AirPollutionToolInput{Lat: 48.8534, Lon: 2.3488})
	if err == nil {
		t.Error("expected error for empty list response")
	}
}

func TestAirPollutionTool_Integration(t *testing.T) {
	projectRoot := testutils.GetProjectRoot()
	_ = godotenv.Load(
		filepath.Join(projectRoot, ".env.test"),
	)

	apiKey := os.Getenv("OPENWEATHERMAP_API_KEY")
	if apiKey == "" {
		t.Skip("OPENWEATHERMAP_API_KEY not set in .env.test, skipping integration test")
	}

	tool := NewAirPollutionTool(apiKey, ratelimit.Noop())
	result, err := tool.Call(context.Background(), AirPollutionToolInput{Lat: 48.8566, Lon: 2.3522})
	if err != nil {
		t.Fatalf("integration call failed: %v", err)
	}
	if result.AQI < 1 || result.AQI > 5 {
		t.Errorf("AQI out of range [1-5]: %d", result.AQI)
	}
	if result.DateTime == "" {
		t.Error("expected non-empty DateTime")
	}
	if result.Components.CO <= 0 {
		t.Errorf("expected positive CO value, got %f", result.Components.CO)
	}
	if result.Components.PM25 < 0 {
		t.Errorf("expected non-negative PM2.5 value, got %f", result.Components.PM25)
	}
}
