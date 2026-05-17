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

func TestHourlyForecastTool_Metadata(t *testing.T) {
	tool := NewHourlyForecastTool("key")
	if tool.Name() != "get_hourly_forecast" {
		t.Errorf("unexpected tool name: %q", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestHourlyForecastTool_Call_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type hourlyResp struct {
			Cod  string `json:"cod"`
			Cnt  int    `json:"cnt"`
			List []struct {
				Dt   int64 `json:"dt"`
				Main struct {
					Temp      float64 `json:"temp"`
					FeelsLike float64 `json:"feels_like"`
					TempMin   float64 `json:"temp_min"`
					TempMax   float64 `json:"temp_max"`
					Pressure  int     `json:"pressure"`
					Humidity  int     `json:"humidity"`
					SeaLevel  int     `json:"sea_level"`
					GrndLevel int     `json:"grnd_level"`
				} `json:"main"`
				Weather []struct {
					ID          int    `json:"id"`
					Main        string `json:"main"`
					Description string `json:"description"`
					Icon        string `json:"icon"`
				} `json:"weather"`
				Clouds struct {
					All int `json:"all"`
				} `json:"clouds"`
				Wind struct {
					Speed float64 `json:"speed"`
					Deg   int     `json:"deg"`
					Gust  float64 `json:"gust"`
				} `json:"wind"`
				Rain *struct {
					OneH float64 `json:"1h"`
				} `json:"rain"`
				Snow *struct {
					OneH float64 `json:"1h"`
				} `json:"snow"`
				Visibility int     `json:"visibility"`
				Pop        float64 `json:"pop"`
				Sys        struct {
					Pod string `json:"pod"`
				} `json:"sys"`
				DtTxt string `json:"dt_txt"`
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

		var resp hourlyResp
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
			Main struct {
				Temp      float64 `json:"temp"`
				FeelsLike float64 `json:"feels_like"`
				TempMin   float64 `json:"temp_min"`
				TempMax   float64 `json:"temp_max"`
				Pressure  int     `json:"pressure"`
				Humidity  int     `json:"humidity"`
				SeaLevel  int     `json:"sea_level"`
				GrndLevel int     `json:"grnd_level"`
			} `json:"main"`
			Weather []struct {
				ID          int    `json:"id"`
				Main        string `json:"main"`
				Description string `json:"description"`
				Icon        string `json:"icon"`
			} `json:"weather"`
			Clouds struct {
				All int `json:"all"`
			} `json:"clouds"`
			Wind struct {
				Speed float64 `json:"speed"`
				Deg   int     `json:"deg"`
				Gust  float64 `json:"gust"`
			} `json:"wind"`
			Rain *struct {
				OneH float64 `json:"1h"`
			} `json:"rain"`
			Snow *struct {
				OneH float64 `json:"1h"`
			} `json:"snow"`
			Visibility int     `json:"visibility"`
			Pop        float64 `json:"pop"`
			Sys        struct {
				Pod string `json:"pod"`
			} `json:"sys"`
			DtTxt string `json:"dt_txt"`
		}, 2)

		resp.List[0].Dt = 1711360800
		resp.List[0].Main.Temp = 18.5
		resp.List[0].Main.FeelsLike = 17.8
		resp.List[0].Main.TempMin = 18.0
		resp.List[0].Main.TempMax = 19.0
		resp.List[0].Main.Pressure = 1015
		resp.List[0].Main.Humidity = 72
		resp.List[0].Main.SeaLevel = 1015
		resp.List[0].Main.GrndLevel = 1010
		resp.List[0].Weather = []struct {
			ID          int    `json:"id"`
			Main        string `json:"main"`
			Description string `json:"description"`
			Icon        string `json:"icon"`
		}{{ID: 800, Main: "Clear", Description: "clear sky", Icon: "01d"}}
		resp.List[0].Clouds.All = 5
		resp.List[0].Wind.Speed = 3.5
		resp.List[0].Wind.Deg = 210
		resp.List[0].Wind.Gust = 5.2
		resp.List[0].Visibility = 10000
		resp.List[0].Pop = 0.1
		resp.List[0].DtTxt = "2024-03-25 10:00:00"

		resp.List[1].Dt = 1711364400
		resp.List[1].Main.Temp = 19.2
		resp.List[1].Main.FeelsLike = 18.5
		resp.List[1].Main.TempMin = 18.5
		resp.List[1].Main.TempMax = 19.5
		resp.List[1].Main.Pressure = 1014
		resp.List[1].Main.Humidity = 68
		resp.List[1].Weather = []struct {
			ID          int    `json:"id"`
			Main        string `json:"main"`
			Description string `json:"description"`
			Icon        string `json:"icon"`
		}{{ID: 801, Main: "Clouds", Description: "few clouds", Icon: "02d"}}
		resp.List[1].Clouds.All = 20
		resp.List[1].Wind.Speed = 4.0
		resp.List[1].Wind.Deg = 220
		resp.List[1].Visibility = 10000
		resp.List[1].Pop = 0.25
		resp.List[1].DtTxt = "2024-03-25 11:00:00"

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tool := newHourlyForecastToolWithBaseURL("testkey", srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), HourlyForecastToolInput{Lat: 48.8534, Lon: 2.3488})
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
	if f0.Temp != 18.5 {
		t.Errorf("expected Temp 18.5, got %f", f0.Temp)
	}
	if f0.FeelsLike != 17.8 {
		t.Errorf("expected FeelsLike 17.8, got %f", f0.FeelsLike)
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
	if f0.Visibility != 10000 {
		t.Errorf("expected Visibility 10000, got %d", f0.Visibility)
	}
	if f0.Pop != 0.1 {
		t.Errorf("expected Pop 0.1, got %f", f0.Pop)
	}
	if len(f0.Weather) != 1 || f0.Weather[0].Main != "Clear" {
		t.Errorf("unexpected Weather: %v", f0.Weather)
	}
	if f0.Precipitation != nil {
		t.Errorf("expected Precipitation to be nil, got %v", f0.Precipitation)
	}

	f1 := result.Forecasts[1]
	if f1.Temp != 19.2 {
		t.Errorf("expected Temp 19.2, got %f", f1.Temp)
	}
	if f1.Pop != 0.25 {
		t.Errorf("expected Pop 0.25, got %f", f1.Pop)
	}
}

func TestHourlyForecastTool_Call_WithCountLimit(t *testing.T) {
	var receivedCnt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCnt = r.URL.Query().Get("cnt")
		payload := `{
			"cod": "200",
			"cnt": 3,
			"list": [
				{"dt": 1711360800, "main": {"temp": 18.0}, "weather": [{"id": 800, "main": "Clear", "description": "clear sky"}], "clouds": {"all": 0}, "wind": {"speed": 2.0, "deg": 180}, "visibility": 10000, "pop": 0.0},
				{"dt": 1711364400, "main": {"temp": 18.5}, "weather": [{"id": 800, "main": "Clear", "description": "clear sky"}], "clouds": {"all": 0}, "wind": {"speed": 2.2, "deg": 185}, "visibility": 10000, "pop": 0.0},
				{"dt": 1711368000, "main": {"temp": 19.0}, "weather": [{"id": 801, "main": "Clouds", "description": "few clouds"}], "clouds": {"all": 20}, "wind": {"speed": 2.5, "deg": 190}, "visibility": 10000, "pop": 0.05}
			],
			"city": {"id": 2988507, "name": "Paris", "coord": {"lat": 48.8534, "lon": 2.3488}, "country": "FR", "timezone": 3600, "sunrise": 1711341600, "sunset": 1711387200}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	tool := newHourlyForecastToolWithBaseURL("testkey", srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), HourlyForecastToolInput{Lat: 48.8534, Lon: 2.3488, Count: 3})
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

func TestHourlyForecastTool_Call_WithoutCountLimit(t *testing.T) {
	var receivedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		payload := `{"cod": "200", "cnt": 1, "list": [{"dt": 1711360800, "main": {"temp": 18.0}, "weather": [{"id": 800, "main": "Clear", "description": "clear sky"}], "clouds": {"all": 0}, "wind": {"speed": 2.0, "deg": 180}, "visibility": 10000, "pop": 0.0}], "city": {"id": 2988507, "name": "Paris", "coord": {"lat": 48.8534, "lon": 2.3488}, "country": "FR", "timezone": 3600, "sunrise": 1711341600, "sunset": 1711387200}}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	tool := newHourlyForecastToolWithBaseURL("testkey", srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), HourlyForecastToolInput{Lat: 48.8534, Lon: 2.3488})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(receivedURL, "cnt=") {
		t.Errorf("expected no cnt param when Count is 0, got URL: %s", receivedURL)
	}
}

func TestHourlyForecastTool_Call_ZeroCoordinates(t *testing.T) {
	tool := NewHourlyForecastTool("key")
	_, err := tool.Call(context.Background(), HourlyForecastToolInput{Lat: 0, Lon: 0})
	if err == nil {
		t.Error("expected error for zero coordinates")
	}
}

func TestHourlyForecastTool_Call_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	tool := newHourlyForecastToolWithBaseURL("badkey", srv.URL, srv.Client())
	_, err := tool.Call(context.Background(), HourlyForecastToolInput{Lat: 48.8534, Lon: 2.3488})
	if err == nil {
		t.Error("expected error for non-200 API response")
	}
}

func TestHourlyForecastTool_Call_WithPrecipitation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload := `{
			"cod": "200",
			"cnt": 1,
			"list": [{
				"dt": 1711360800,
				"main": {"temp": 10.0, "feels_like": 8.5, "temp_min": 9.0, "temp_max": 11.0, "pressure": 1010, "humidity": 90},
				"weather": [{"id": 500, "main": "Rain", "description": "light rain", "icon": "10d"}],
				"clouds": {"all": 80},
				"wind": {"speed": 5.0, "deg": 180},
				"rain": {"1h": 1.2},
				"visibility": 8000,
				"pop": 0.85,
				"sys": {"pod": "d"},
				"dt_txt": "2024-03-25 09:00:00"
			}],
			"city": {"id": 2643743, "name": "London", "coord": {"lat": 51.5085, "lon": -0.1257}, "country": "GB", "timezone": 0, "sunrise": 1711341600, "sunset": 1711387200}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	tool := newHourlyForecastToolWithBaseURL("testkey", srv.URL, srv.Client())
	result, err := tool.Call(context.Background(), HourlyForecastToolInput{Lat: 51.5085, Lon: -0.1257})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Forecasts) != 1 {
		t.Fatalf("expected 1 forecast, got %d", len(result.Forecasts))
	}
	f := result.Forecasts[0]
	if f.Precipitation == nil {
		t.Fatal("expected Precipitation to be non-nil")
	}
	if *f.Precipitation != 1.2 {
		t.Errorf("expected Precipitation 1.2, got %f", *f.Precipitation)
	}
	if f.Pop != 0.85 {
		t.Errorf("expected Pop 0.85, got %f", f.Pop)
	}
	if f.Snow != nil {
		t.Errorf("expected Snow to be nil, got %v", f.Snow)
	}
}

func TestHourlyForecastTool_Integration(t *testing.T) {
	projectRoot := testutils.GetProjectRoot()
	_ = godotenv.Load(
		filepath.Join(projectRoot, ".env.test"),
	)

	apiKey := os.Getenv("OPENWEATHERMAP_API_KEY")
	if apiKey == "" {
		t.Skip("OPENWEATHERMAP_API_KEY not set in .env.test, skipping integration test")
	}

	if v := os.Getenv("OPENWEATHERMAP_FREE_PLAN"); v == "" || !strings.EqualFold(v, "false") {
		t.Skip("hourly forecast requires a paid OWM subscription (set OPENWEATHERMAP_FREE_PLAN=false to run)")
	}

	tool := NewHourlyForecastTool(apiKey)
	result, err := tool.Call(context.Background(), HourlyForecastToolInput{Lat: 48.8566, Lon: 2.3522})
	if err != nil {
		if strings.Contains(err.Error(), "status 401") {
			t.Skip("hourly forecast requires a paid OWM subscription, skipping")
		}
		t.Fatalf("integration call failed: %v", err)
	}
	if result.City.Name == "" {
		t.Error("expected non-empty city name")
	}
	if result.City.Country == "" {
		t.Error("expected non-empty country code")
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
		if f.Humidity < 0 || f.Humidity > 100 {
			t.Errorf("humidity out of range: %d", f.Humidity)
		}
		if len(f.Weather) == 0 {
			t.Error("expected at least one weather condition")
		}
	}
}
