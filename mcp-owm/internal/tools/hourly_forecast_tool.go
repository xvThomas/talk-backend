package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/xvThomas/LLMClientWrapper/talk-libs/mcpserver"
)

const defaultProBaseURL = "https://pro.openweathermap.org/data/2.5"

// HourlyForecastToolInput is the typed input for HourlyForecastTool.
type HourlyForecastToolInput struct {
	Lat   float64 `json:"lat" description:"Latitude of the location"`
	Lon   float64 `json:"lon" description:"Longitude of the location"`
	Count int     `json:"count,omitempty" description:"Optional number of hourly timestamps to return (1-96). If omitted, returns all 96 timestamps (4 days)."`
}

// HourlyForecastToolOutput is the typed output for HourlyForecastTool.
type HourlyForecastToolOutput struct {
	City      ForecastCity    `json:"city" description:"City information"`
	Count     int             `json:"cnt" description:"Number of forecast entries"`
	Forecasts []ForecastEntry `json:"forecasts" description:"Hourly forecast entries (up to 96 timestamps for 4 days)"`
}

// HourlyForecastTool implements mcpserver.MCPTool for fetching 4-day hourly forecast via OpenWeatherMap Pro.
type HourlyForecastTool struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewHourlyForecastTool creates a HourlyForecastTool with the given API key.
func NewHourlyForecastTool(apiKey string) mcpserver.MCPTool[HourlyForecastToolInput, HourlyForecastToolOutput] {
	return &HourlyForecastTool{apiKey: apiKey, baseURL: defaultProBaseURL, http: &http.Client{}}
}

var _ mcpserver.MCPTool[HourlyForecastToolInput, HourlyForecastToolOutput] = (*HourlyForecastTool)(nil)

// newHourlyForecastToolWithBaseURL creates a HourlyForecastTool with a custom base URL (for testing).
func newHourlyForecastToolWithBaseURL(apiKey, baseURL string, client *http.Client) *HourlyForecastTool {
	return &HourlyForecastTool{apiKey: apiKey, baseURL: baseURL, http: client}
}

// Name returns the tool name as expected by the model.
func (t *HourlyForecastTool) Name() string { return "get_hourly_forecast" }

// Description describes what the tool does.
func (t *HourlyForecastTool) Description() string {
	return "Get the hourly weather forecast for the next 4 days (96 hours) for a given location specified by latitude and longitude. Use the geocode_city tool first to convert a city name to coordinates. Returns up to 96 data points including temperature, humidity, wind, precipitation probability, and weather conditions. Use the optional 'count' parameter to limit the number of hourly timestamps returned."
}

// Call calls the OpenWeatherMap Pro hourly forecast API and returns a typed output struct.
func (t *HourlyForecastTool) Call(ctx context.Context, input HourlyForecastToolInput) (HourlyForecastToolOutput, error) {
	if input.Lat == 0 && input.Lon == 0 {
		return HourlyForecastToolOutput{}, fmt.Errorf("parameters 'lat' and 'lon' must not both be zero")
	}

	response, err := t.fetchHourlyForecast(ctx, input.Lat, input.Lon, input.Count)
	if err != nil {
		return HourlyForecastToolOutput{}, err
	}

	out := HourlyForecastToolOutput{
		City: ForecastCity{
			Name:     response.City.Name,
			Coord:    Coordinates{Lon: response.City.Coord.Lon, Lat: response.City.Coord.Lat},
			Country:  response.City.Country,
			Timezone: response.City.Timezone,
			Sunrise:  response.City.Sunrise,
			Sunset:   response.City.Sunset,
		},
		Count:     response.Cnt,
		Forecasts: make([]ForecastEntry, 0, len(response.List)),
	}

	for _, item := range response.List {
		entry := ForecastEntry{
			DateTime:   time.Unix(item.Dt, 0).UTC().Format(time.RFC3339),
			Temp:       item.Main.Temp,
			FeelsLike:  item.Main.FeelsLike,
			TempMin:    item.Main.TempMin,
			TempMax:    item.Main.TempMax,
			Pressure:   item.Main.Pressure,
			Humidity:   item.Main.Humidity,
			SeaLevel:   item.Main.SeaLevel,
			GrndLevel:  item.Main.GrndLevel,
			Cloudiness: item.Clouds.All,
			WindSpeed:  item.Wind.Speed,
			WindDeg:    item.Wind.Deg,
			WindGust:   item.Wind.Gust,
			Visibility: item.Visibility,
			Pop:        item.Pop,
		}

		entry.Weather = make([]WeatherCondition, 0, len(item.Weather))
		for _, w := range item.Weather {
			entry.Weather = append(entry.Weather, WeatherCondition{
				Main:        w.Main,
				Description: w.Description,
			})
		}

		if item.Rain != nil {
			entry.Precipitation = &item.Rain.OneH
		}
		if item.Snow != nil {
			entry.Snow = &item.Snow.OneH
		}

		out.Forecasts = append(out.Forecasts, entry)
	}

	return out, nil
}

type hourlyForecastResponse struct {
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

func (t *HourlyForecastTool) fetchHourlyForecast(ctx context.Context, lat, lon float64, count int) (*hourlyForecastResponse, error) {
	endpoint := fmt.Sprintf("%s/forecast/hourly?lat=%f&lon=%f&appid=%s&units=metric",
		t.baseURL, lat, lon, t.apiKey)

	if count > 0 {
		endpoint += fmt.Sprintf("&cnt=%d", count)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building hourly forecast request: %w", err)
	}

	resp, err := t.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hourly forecast API request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hourly forecast API returned status %d", resp.StatusCode)
	}

	var data hourlyForecastResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding hourly forecast response: %w", err)
	}

	return &data, nil
}
