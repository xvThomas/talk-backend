package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/xvThomas/LLMClientWrapper/talk-libs/mcpserver"
)

// DailyForecastToolInput is the typed input for DailyForecastTool.
type DailyForecastToolInput struct {
	Lat   float64 `json:"lat" description:"Latitude of the location"`
	Lon   float64 `json:"lon" description:"Longitude of the location"`
	Count int     `json:"count,omitempty" description:"Optional number of days to return (1-16). If omitted, returns 16 days."`
}

// DailyTemperature contains the temperature breakdown for a day.
type DailyTemperature struct {
	Day   float64 `json:"day" description:"Temperature at 12:00 local time in Celsius"`
	Min   float64 `json:"min" description:"Min daily temperature in Celsius"`
	Max   float64 `json:"max" description:"Max daily temperature in Celsius"`
	Night float64 `json:"night" description:"Temperature at 00:00 local time in Celsius"`
	Eve   float64 `json:"eve" description:"Temperature at 18:00 local time in Celsius"`
	Morn  float64 `json:"morn" description:"Temperature at 06:00 local time in Celsius"`
}

// DailyFeelsLike contains the feels-like temperature breakdown for a day.
type DailyFeelsLike struct {
	Day   float64 `json:"day" description:"Feels-like temperature at 12:00 local time in Celsius"`
	Night float64 `json:"night" description:"Feels-like temperature at 00:00 local time in Celsius"`
	Eve   float64 `json:"eve" description:"Feels-like temperature at 18:00 local time in Celsius"`
	Morn  float64 `json:"morn" description:"Feels-like temperature at 06:00 local time in Celsius"`
}

// DailyForecastEntry represents a single daily forecast data point.
type DailyForecastEntry struct {
	DateTime   string             `json:"dt" description:"Forecast date in ISO 8601 format (e.g. 2026-03-30T12:00:00Z)"`
	Temp       DailyTemperature   `json:"temp" description:"Temperature breakdown for the day"`
	FeelsLike  DailyFeelsLike     `json:"feels_like" description:"Feels-like temperature breakdown for the day"`
	Pressure   int                `json:"pressure" description:"Atmospheric pressure in hPa"`
	Humidity   int                `json:"humidity" description:"Humidity percentage"`
	Weather    []WeatherCondition `json:"weather" description:"Weather conditions"`
	WindSpeed  float64            `json:"wind_speed" description:"Maximum wind speed in meter/sec"`
	WindDeg    int                `json:"wind_deg" description:"Wind direction in degrees"`
	WindGust   float64            `json:"wind_gust,omitempty" description:"Wind gust in meter/sec"`
	Cloudiness int                `json:"cloudiness" description:"Cloudiness percentage"`
	Pop        float64            `json:"pop" description:"Probability of precipitation (0 to 1)"`
	Rain       *float64           `json:"rain,omitempty" description:"Precipitation volume in mm"`
	Snow       *float64           `json:"snow,omitempty" description:"Snow volume in mm"`
}

// DailyForecastToolOutput is the typed output for DailyForecastTool.
type DailyForecastToolOutput struct {
	City      ForecastCity         `json:"city" description:"City information"`
	Count     int                  `json:"cnt" description:"Number of forecast days"`
	Forecasts []DailyForecastEntry `json:"forecasts" description:"Daily forecast entries (up to 16 days)"`
}

// DailyForecastTool implements mcpserver.MCPTool for fetching the 16-day daily forecast via OpenWeatherMap.
type DailyForecastTool struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewDailyForecastTool creates a DailyForecastTool with the given API key.
func NewDailyForecastTool(apiKey string) mcpserver.MCPTool[DailyForecastToolInput, DailyForecastToolOutput] {
	return &DailyForecastTool{apiKey: apiKey, baseURL: defaultBaseURL, http: &http.Client{}}
}

var _ mcpserver.MCPTool[DailyForecastToolInput, DailyForecastToolOutput] = (*DailyForecastTool)(nil)

// newDailyForecastToolWithBaseURL creates a DailyForecastTool with a custom base URL (for testing).
func newDailyForecastToolWithBaseURL(apiKey, baseURL string, client *http.Client) *DailyForecastTool {
	return &DailyForecastTool{apiKey: apiKey, baseURL: baseURL, http: client}
}

// Name returns the tool name as expected by the model.
func (t *DailyForecastTool) Name() string { return "get_daily_forecast" }

// Description describes what the tool does.
func (t *DailyForecastTool) Description() string {
	return "Get the daily weather forecast for up to 16 days for a given location specified by latitude and longitude. Use the geocode_city tool first to convert a city name to coordinates. Returns daily temperature (day/min/max/night/eve/morn), feels-like, humidity, wind, precipitation probability, and weather conditions. Use the optional 'count' parameter to limit the number of days returned (1-16)."
}

// Call calls the OpenWeatherMap 16-day daily forecast API and returns a typed output struct.
func (t *DailyForecastTool) Call(ctx context.Context, input DailyForecastToolInput) (DailyForecastToolOutput, error) {
	if input.Lat == 0 && input.Lon == 0 {
		return DailyForecastToolOutput{}, fmt.Errorf("parameters 'lat' and 'lon' must not both be zero")
	}

	response, err := t.fetchDailyForecast(ctx, input.Lat, input.Lon, input.Count)
	if err != nil {
		return DailyForecastToolOutput{}, err
	}

	out := DailyForecastToolOutput{
		City: ForecastCity{
			Name:     response.City.Name,
			Coord:    Coordinates{Lon: response.City.Coord.Lon, Lat: response.City.Coord.Lat},
			Country:  response.City.Country,
			Timezone: response.City.Timezone,
			Sunrise:  response.City.Sunrise,
			Sunset:   response.City.Sunset,
		},
		Count:     response.Cnt,
		Forecasts: make([]DailyForecastEntry, 0, len(response.List)),
	}

	for _, item := range response.List {
		entry := DailyForecastEntry{
			DateTime: time.Unix(item.Dt, 0).UTC().Format(time.RFC3339),
			Temp: DailyTemperature{
				Day:   item.Temp.Day,
				Min:   item.Temp.Min,
				Max:   item.Temp.Max,
				Night: item.Temp.Night,
				Eve:   item.Temp.Eve,
				Morn:  item.Temp.Morn,
			},
			FeelsLike: DailyFeelsLike{
				Day:   item.FeelsLike.Day,
				Night: item.FeelsLike.Night,
				Eve:   item.FeelsLike.Eve,
				Morn:  item.FeelsLike.Morn,
			},
			Pressure:   item.Pressure,
			Humidity:   item.Humidity,
			Cloudiness: item.Clouds,
			WindSpeed:  item.Speed,
			WindDeg:    item.Deg,
			WindGust:   item.Gust,
			Pop:        item.Pop,
		}

		entry.Weather = make([]WeatherCondition, 0, len(item.Weather))
		for _, w := range item.Weather {
			entry.Weather = append(entry.Weather, WeatherCondition{
				Main:        w.Main,
				Description: w.Description,
			})
		}

		if item.Rain > 0 {
			rain := item.Rain
			entry.Rain = &rain
		}
		if item.Snow > 0 {
			snow := item.Snow
			entry.Snow = &snow
		}

		out.Forecasts = append(out.Forecasts, entry)
	}

	return out, nil
}

type dailyForecastResponse struct {
	Cod  interface{} `json:"cod"`
	Cnt  int         `json:"cnt"`
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

func (t *DailyForecastTool) fetchDailyForecast(ctx context.Context, lat, lon float64, count int) (*dailyForecastResponse, error) {
	endpoint := fmt.Sprintf("%s/forecast/daily?lat=%f&lon=%f&appid=%s&units=metric",
		t.baseURL, lat, lon, t.apiKey)

	if count > 0 {
		endpoint += fmt.Sprintf("&cnt=%d", count)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building daily forecast request: %w", err)
	}

	resp, err := t.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("daily forecast API request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daily forecast API returned status %d", resp.StatusCode)
	}

	var data dailyForecastResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding daily forecast response: %w", err)
	}

	return &data, nil
}
