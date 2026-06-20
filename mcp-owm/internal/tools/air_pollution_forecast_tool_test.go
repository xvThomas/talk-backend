package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/xvThomas/talk-backend/mcp-owm/internal/ratelimit"

	"github.com/xvThomas/talk-backend/talk-libs/testutils"

	"github.com/joho/godotenv"
)

func TestAirPollutionForecastTool_Metadata(t *testing.T) {
	tool := NewAirPollutionForecastTool("key", ratelimit.Noop())
	if tool.Name() != "get_air_pollution_forecast" {
		t.Errorf("unexpected tool name: %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestAirPollutionForecastTool_Call_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload := `{
			"coord": [48.8534, 2.3488],
			"list": [
				{
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
				},
				{
					"dt": 1711364400,
					"main": {"aqi": 3},
					"components": {
						"co": 280.10,
						"no": 6.12,
						"no2": 45.20,
						"o3": 5.10,
						"so2": 15.00,
						"pm2_5": 14.80,
						"pm10": 16.90,
						"nh3": 0.35
					}
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	tool := newAirPollutionForecastToolWithBaseURL("testkey", srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), AirPollutionForecastToolInput{Lat: 48.8534, Lon: 2.3488})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.Items[0].AQI != 2 {
		t.Errorf("expected first AQI 2, got %d", result.Items[0].AQI)
	}
	if result.Items[1].AQI != 3 {
		t.Errorf("expected second AQI 3, got %d", result.Items[1].AQI)
	}
	if result.Items[0].DateTime == "" {
		t.Error("expected non-empty DateTime on first item")
	}
	if result.Items[0].Components.CO != 270.37 {
		t.Errorf("expected CO 270.37, got %f", result.Items[0].Components.CO)
	}
	if result.Items[1].Components.PM25 != 14.80 {
		t.Errorf("expected PM2.5 14.80, got %f", result.Items[1].Components.PM25)
	}
}

func TestAirPollutionForecastTool_Call_ZeroCoordinates(t *testing.T) {
	tool := NewAirPollutionForecastTool("key", ratelimit.Noop())
	_, err := tool.Call(context.Background(), AirPollutionForecastToolInput{Lat: 0, Lon: 0})
	if err == nil {
		t.Error("expected error for zero coordinates")
	}
}

func TestAirPollutionForecastTool_Call_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	tool := newAirPollutionForecastToolWithBaseURL("badkey", srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), AirPollutionForecastToolInput{Lat: 48.8534, Lon: 2.3488})
	if err == nil {
		t.Error("expected error for non-200 API response")
	}
}

func TestAirPollutionForecastTool_Call_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload := `{"coord": [48.8534, 2.3488], "list": []}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	tool := newAirPollutionForecastToolWithBaseURL("testkey", srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), AirPollutionForecastToolInput{Lat: 48.8534, Lon: 2.3488})
	if err == nil {
		t.Error("expected error for empty list response")
	}
}

func TestAirPollutionForecastTool_Integration(t *testing.T) {
	projectRoot := testutils.GetProjectRoot()
	_ = godotenv.Load(
		filepath.Join(projectRoot, ".env.test"),
	)

	apiKey := os.Getenv("OPENWEATHERMAP_API_KEY")
	if apiKey == "" {
		t.Skip("OPENWEATHERMAP_API_KEY not set in .env.test, skipping integration test")
	}

	tool := NewAirPollutionForecastTool(apiKey, ratelimit.Noop())
	result, err := tool.Call(context.Background(), AirPollutionForecastToolInput{Lat: 48.8566, Lon: 2.3522})
	if err != nil {
		t.Fatalf("integration call failed: %v", err)
	}
	if len(result.Items) == 0 {
		t.Fatal("expected non-empty forecast items")
	}
	// 4 days of hourly data should yield ~96 entries
	if len(result.Items) < 24 {
		t.Errorf("expected at least 24 forecast entries, got %d", len(result.Items))
	}
	for i, item := range result.Items[:3] {
		if item.AQI < 1 || item.AQI > 5 {
			t.Errorf("item[%d] AQI out of range [1-5]: %d", i, item.AQI)
		}
		if item.DateTime == "" {
			t.Errorf("item[%d] expected non-empty DateTime", i)
		}
	}
}
