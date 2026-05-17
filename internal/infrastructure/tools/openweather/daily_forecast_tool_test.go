package openweather

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"talks/internal/infrastructure/helpers/testutils"
	"testing"

	"github.com/joho/godotenv"
)

func TestDailyForecastTool_Metadata(t *testing.T) {
	tool := NewDailyForecastTool("key")
	if tool.Name() != "get_daily_forecast" {
		t.Errorf("unexpected tool name: %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestDailyForecastTool_Call_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type dailyResp struct {
			Cod  string `json:"cod"`
			Cnt  int    `json:"cnt"`
			List []struct {
				Dt   int64 `json:"dt"`
				Temp struct {
					Day   float64 `json:"day"`
					Min   float64 `json:"min"`
					Max   float64 `json:"max"`
					Night float64 `json:"night"`
					Eve   float64 `json:"eve"`
					Morn  float64 `json:"morn"`
				} `json:"temp"`
				FeelsLike struct {
					Day   float64 `json:"day"`
					Night float64 `json:"night"`
					Eve   float64 `json:"eve"`
					Morn  float64 `json:"morn"`
				} `json:"feels_like"`
				Pressure int `json:"pressure"`
				Humidity int `json:"humidity"`
				Weather  []struct {
					ID          int    `json:"id"`
					Main        string `json:"main"`
					Description string `json:"description"`
					Icon        string `json:"icon"`
				} `json:"weather"`
				Speed  float64 `json:"speed"`
				Deg    int     `json:"deg"`
				Gust   float64 `json:"gust"`
				Clouds int     `json:"clouds"`
				Pop    float64 `json:"pop"`
				Rain   float64 `json:"rain"`
				Snow   float64 `json:"snow"`
			} `json:"list"`
			City struct {
				ID    int    `json:"id"`
				Name  string `json:"name"`
				Coord struct {
					Lat float64 `json:"lat"`
					Lon float64 `json:"lon"`
				} `json:"coord"`
				Country  string `json:"country"`
				Timezone int    `json:"timezone"`
				Sunrise  int64  `json:"sunrise"`
				Sunset   int64  `json:"sunset"`
			} `json:"city"`
		}

		var resp dailyResp
		resp.Cod = "200"
		resp.Cnt = 2
		resp.City.ID = 2988507
		resp.City.Name = "Paris"
		resp.City.Coord.Lat = 48.8534
		resp.City.Coord.Lon = 2.3488
		resp.City.Country = "FR"
		resp.City.Timezone = 3600
		resp.City.Sunrise = 1711341600
		resp.City.Sunset = 1711387200

		resp.List = make([]struct {
			Dt   int64 `json:"dt"`
			Temp struct {
				Day   float64 `json:"day"`
				Min   float64 `json:"min"`
				Max   float64 `json:"max"`
				Night float64 `json:"night"`
				Eve   float64 `json:"eve"`
				Morn  float64 `json:"morn"`
			} `json:"temp"`
			FeelsLike struct {
				Day   float64 `json:"day"`
				Night float64 `json:"night"`
				Eve   float64 `json:"eve"`
				Morn  float64 `json:"morn"`
			} `json:"feels_like"`
			Pressure int `json:"pressure"`
			Humidity int `json:"humidity"`
			Weather  []struct {
				ID          int    `json:"id"`
				Main        string `json:"main"`
				Description string `json:"description"`
				Icon        string `json:"icon"`
			} `json:"weather"`
			Speed  float64 `json:"speed"`
			Deg    int     `json:"deg"`
			Gust   float64 `json:"gust"`
			Clouds int     `json:"clouds"`
			Pop    float64 `json:"pop"`
			Rain   float64 `json:"rain"`
			Snow   float64 `json:"snow"`
		}, 2)

		resp.List[0].Dt = 1711360800
		resp.List[0].Temp.Day = 18.5
		resp.List[0].Temp.Min = 12.0
		resp.List[0].Temp.Max = 20.0
		resp.List[0].Temp.Night = 13.5
		resp.List[0].Temp.Eve = 17.0
		resp.List[0].Temp.Morn = 12.5
		resp.List[0].FeelsLike.Day = 17.8
		resp.List[0].FeelsLike.Night = 12.5
		resp.List[0].FeelsLike.Eve = 16.0
		resp.List[0].FeelsLike.Morn = 11.5
		resp.List[0].Pressure = 1015
		resp.List[0].Humidity = 72
		resp.List[0].Weather = []struct {
			ID          int    `json:"id"`
			Main        string `json:"main"`
			Description string `json:"description"`
			Icon        string `json:"icon"`
		}{{ID: 800, Main: "Clear", Description: "clear sky", Icon: "01d"}}
		resp.List[0].Speed = 3.5
		resp.List[0].Deg = 210
		resp.List[0].Gust = 5.2
		resp.List[0].Clouds = 5
		resp.List[0].Pop = 0.1

		resp.List[1].Dt = 1711447200
		resp.List[1].Temp.Day = 19.2
		resp.List[1].Temp.Min = 13.0
		resp.List[1].Temp.Max = 21.5
		resp.List[1].Temp.Night = 14.0
		resp.List[1].Temp.Eve = 18.0
		resp.List[1].Temp.Morn = 13.5
		resp.List[1].FeelsLike.Day = 18.5
		resp.List[1].FeelsLike.Night = 13.0
		resp.List[1].FeelsLike.Eve = 17.0
		resp.List[1].FeelsLike.Morn = 12.5
		resp.List[1].Pressure = 1012
		resp.List[1].Humidity = 65
		resp.List[1].Weather = []struct {
			ID          int    `json:"id"`
			Main        string `json:"main"`
			Description string `json:"description"`
			Icon        string `json:"icon"`
		}{{ID: 802, Main: "Clouds", Description: "scattered clouds", Icon: "03d"}}
		resp.List[1].Speed = 4.0
		resp.List[1].Deg = 220
		resp.List[1].Gust = 6.0
		resp.List[1].Clouds = 40
		resp.List[1].Pop = 0.25

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newDailyForecastToolWithBaseURL("testkey", srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), DailyForecastToolInput{Lat: 48.8534, Lon: 2.3488})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.City.Name != "Paris" {
		t.Errorf("expected City.Name %q, got %q", "Paris", result.City.Name)
	}
	if result.City.Country != "FR" {
		t.Errorf("expected City.Country %q, got %q", "FR", result.City.Country)
	}
	if result.City.Coord.Lat != 48.8534 {
		t.Errorf("expected City.Coord.Lat 48.8534, got %f", result.City.Coord.Lat)
	}
	if result.Count != 2 {
		t.Errorf("expected Count 2, got %d", result.Count)
	}
	if len(result.Forecasts) != 2 {
		t.Fatalf("expected 2 forecasts, got %d", len(result.Forecasts))
	}

	f0 := result.Forecasts[0]
	if f0.Temp.Day != 18.5 {
		t.Errorf("expected Temp.Day 18.5, got %f", f0.Temp.Day)
	}
	if f0.Temp.Min != 12.0 {
		t.Errorf("expected Temp.Min 12.0, got %f", f0.Temp.Min)
	}
	if f0.Temp.Max != 20.0 {
		t.Errorf("expected Temp.Max 20.0, got %f", f0.Temp.Max)
	}
	if f0.Temp.Night != 13.5 {
		t.Errorf("expected Temp.Night 13.5, got %f", f0.Temp.Night)
	}
	if f0.FeelsLike.Day != 17.8 {
		t.Errorf("expected FeelsLike.Day 17.8, got %f", f0.FeelsLike.Day)
	}
	if f0.Humidity != 72 {
		t.Errorf("expected Humidity 72, got %d", f0.Humidity)
	}
	if f0.WindSpeed != 3.5 {
		t.Errorf("expected WindSpeed 3.5, got %f", f0.WindSpeed)
	}
	if f0.WindDeg != 210 {
		t.Errorf("expected WindDeg 210, got %d", f0.WindDeg)
	}
	if f0.WindGust != 5.2 {
		t.Errorf("expected WindGust 5.2, got %f", f0.WindGust)
	}
	if f0.Cloudiness != 5 {
		t.Errorf("expected Cloudiness 5, got %d", f0.Cloudiness)
	}
	if f0.Pop != 0.1 {
		t.Errorf("expected Pop 0.1, got %f", f0.Pop)
	}
	if len(f0.Weather) != 1 || f0.Weather[0].Main != "Clear" {
		t.Errorf("unexpected Weather: %v", f0.Weather)
	}
	if f0.Rain != nil {
		t.Errorf("expected Rain to be nil, got %v", f0.Rain)
	}

	f1 := result.Forecasts[1]
	if f1.Temp.Day != 19.2 {
		t.Errorf("expected Temp.Day 19.2, got %f", f1.Temp.Day)
	}
	if f1.Pop != 0.25 {
		t.Errorf("expected Pop 0.25, got %f", f1.Pop)
	}
}

func TestDailyForecastTool_Call_WithCountLimit(t *testing.T) {
	var receivedCnt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCnt = r.URL.Query().Get("cnt")
		payload := `{
			"cod": 200,
			"cnt": 3,
			"list": [
				{"dt": 1711360800, "temp": {"day": 18.0, "min": 12.0, "max": 20.0, "night": 13.0, "eve": 17.0, "morn": 12.0}, "feels_like": {"day": 17.0, "night": 12.0, "eve": 16.0, "morn": 11.0}, "pressure": 1015, "humidity": 60, "weather": [{"id": 800, "main": "Clear", "description": "clear sky"}], "speed": 2.0, "deg": 180, "gust": 3.0, "clouds": 0, "pop": 0.0},
				{"dt": 1711447200, "temp": {"day": 19.0, "min": 13.0, "max": 21.0, "night": 14.0, "eve": 18.0, "morn": 13.0}, "feels_like": {"day": 18.0, "night": 13.0, "eve": 17.0, "morn": 12.0}, "pressure": 1013, "humidity": 55, "weather": [{"id": 800, "main": "Clear", "description": "clear sky"}], "speed": 2.5, "deg": 190, "gust": 3.5, "clouds": 5, "pop": 0.05},
				{"dt": 1711533600, "temp": {"day": 20.0, "min": 14.0, "max": 22.0, "night": 15.0, "eve": 19.0, "morn": 14.0}, "feels_like": {"day": 19.0, "night": 14.0, "eve": 18.0, "morn": 13.0}, "pressure": 1012, "humidity": 50, "weather": [{"id": 801, "main": "Clouds", "description": "few clouds"}], "speed": 3.0, "deg": 200, "gust": 4.0, "clouds": 20, "pop": 0.1}
			],
			"city": {"id": 2988507, "name": "Paris", "coord": {"lat": 48.8534, "lon": 2.3488}, "country": "FR", "timezone": 3600, "sunrise": 1711341600, "sunset": 1711387200}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	tool := newDailyForecastToolWithBaseURL("testkey", srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), DailyForecastToolInput{Lat: 48.8534, Lon: 2.3488, Count: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedCnt != "3" {
		t.Errorf("expected cnt query param %q, got %q", "3", receivedCnt)
	}
	if len(result.Forecasts) != 3 {
		t.Errorf("expected 3 forecasts, got %d", len(result.Forecasts))
	}
}

func TestDailyForecastTool_Call_WithoutCountLimit(t *testing.T) {
	var receivedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		payload := `{"cod": 200, "cnt": 1, "list": [{"dt": 1711360800, "temp": {"day": 18.0, "min": 12.0, "max": 20.0, "night": 13.0, "eve": 17.0, "morn": 12.0}, "feels_like": {"day": 17.0, "night": 12.0, "eve": 16.0, "morn": 11.0}, "pressure": 1015, "humidity": 60, "weather": [{"id": 800, "main": "Clear", "description": "clear sky"}], "speed": 2.0, "deg": 180, "gust": 3.0, "clouds": 0, "pop": 0.0}], "city": {"id": 2988507, "name": "Paris", "coord": {"lat": 48.8534, "lon": 2.3488}, "country": "FR", "timezone": 3600, "sunrise": 1711341600, "sunset": 1711387200}}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	tool := newDailyForecastToolWithBaseURL("testkey", srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), DailyForecastToolInput{Lat: 48.8534, Lon: 2.3488})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(receivedURL, "cnt=") {
		t.Errorf("expected no cnt param when Count is 0, got URL: %s", receivedURL)
	}
}

func TestDailyForecastTool_Call_ZeroCoordinates(t *testing.T) {
	tool := NewDailyForecastTool("key")
	_, err := tool.Call(context.Background(), DailyForecastToolInput{Lat: 0, Lon: 0})
	if err == nil {
		t.Error("expected error for zero coordinates")
	}
}

func TestDailyForecastTool_Call_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	tool := newDailyForecastToolWithBaseURL("badkey", srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), DailyForecastToolInput{Lat: 48.8534, Lon: 2.3488})
	if err == nil {
		t.Error("expected error for non-200 API response")
	}
}

func TestDailyForecastTool_Call_WithPrecipitation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload := `{
			"cod": 200,
			"cnt": 1,
			"list": [{
				"dt": 1711360800,
				"temp": {"day": 10.0, "min": 7.0, "max": 12.0, "night": 8.0, "eve": 9.5, "morn": 7.5},
				"feels_like": {"day": 8.5, "night": 6.5, "eve": 8.0, "morn": 6.0},
				"pressure": 1010,
				"humidity": 90,
				"weather": [{"id": 500, "main": "Rain", "description": "light rain", "icon": "10d"}],
				"speed": 5.0,
				"deg": 180,
				"gust": 7.5,
				"clouds": 80,
				"pop": 0.85,
				"rain": 2.5
			}],
			"city": {"id": 2643743, "name": "London", "coord": {"lat": 51.5085, "lon": -0.1257}, "country": "GB", "timezone": 0, "sunrise": 1711341600, "sunset": 1711387200}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	tool := newDailyForecastToolWithBaseURL("testkey", srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), DailyForecastToolInput{Lat: 51.5085, Lon: -0.1257})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Forecasts) != 1 {
		t.Fatalf("expected 1 forecast, got %d", len(result.Forecasts))
	}
	f := result.Forecasts[0]
	if f.Rain == nil {
		t.Fatal("expected Rain to be non-nil")
	}
	if *f.Rain != 2.5 {
		t.Errorf("expected Rain 2.5, got %f", *f.Rain)
	}
	if f.Pop != 0.85 {
		t.Errorf("expected Pop 0.85, got %f", f.Pop)
	}
	if f.Snow != nil {
		t.Errorf("expected Snow to be nil, got %v", f.Snow)
	}
}

func TestDailyForecastTool_Integration(t *testing.T) {
	projectRoot := testutils.GetProjectRoot()
	_ = godotenv.Load(
		filepath.Join(projectRoot, ".env.test"),
	)

	apiKey := os.Getenv("OPENWEATHERMAP_API_KEY")
	if apiKey == "" {
		t.Skip("OPENWEATHERMAP_API_KEY not set in .env.test, skipping integration test")
	}

	if v := os.Getenv("OPENWEATHERMAP_FREE_PLAN"); v == "" || !strings.EqualFold(v, "false") {
		t.Skip("16-day daily forecast requires a paid OWM subscription (set OPENWEATHERMAP_FREE_PLAN=false to run)")
	}

	tool := NewDailyForecastTool(apiKey)
	result, err := tool.Call(context.Background(), DailyForecastToolInput{Lat: 48.8566, Lon: 2.3522})
	if err != nil {
		if strings.Contains(err.Error(), "status 401") {
			t.Skip("16-day daily forecast requires a paid OWM subscription, skipping")
		}
		t.Fatalf("integration call failed: %v", err)
	}
	if result.City.Name == "" {
		t.Error("expected non-empty city name")
	}
	if result.City.Country == "" {
		t.Error("expected non-empty country code")
	}
	if result.City.Coord.Lat == 0 && result.City.Coord.Lon == 0 {
		t.Error("expected non-zero coordinates for Paris")
	}
	if result.Count == 0 {
		t.Error("expected non-zero forecast count")
	}
	if len(result.Forecasts) == 0 {
		t.Error("expected at least one forecast entry")
	}
	if len(result.Forecasts) > 0 {
		f := result.Forecasts[0]
		if f.DateTime == "" {
			t.Error("expected non-empty DateTime")
		}
		if f.Temp.Max < f.Temp.Min {
			t.Errorf("expected Temp.Max >= Temp.Min, got max=%f min=%f", f.Temp.Max, f.Temp.Min)
		}
		if f.Humidity < 0 || f.Humidity > 100 {
			t.Errorf("humidity out of range: %d", f.Humidity)
		}
		if len(f.Weather) == 0 {
			t.Error("expected at least one weather condition")
		}
	}
}
