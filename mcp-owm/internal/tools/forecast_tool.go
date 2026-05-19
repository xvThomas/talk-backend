package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"github.com/xvThomas/LLMClientWrapper/talk-libs/domain"
	"time"
)

// ForecastToolInput is the typed input for Forecast5Days3HoursWeatherTool.
type ForecastToolInput struct {
	Lat   float64 `json:"lat" description:"Latitude of the location"`
	Lon   float64 `json:"lon" description:"Longitude of the location"`
	Count int     `json:"count,omitempty" description:"Optional number of 3-hour timestamps to return (1-40). If omitted, returns all 40 timestamps (5 days)."`
}

// ForecastEntry represents a single 3-hour forecast data point.
type ForecastEntry struct {
	DateTime      string             `json:"dt" description:"Forecast time in ISO 8601 format (e.g. 2026-03-30T15:00:00Z)"`
	Temp          float64            `json:"temp" description:"Temperature in Celsius"`
	FeelsLike     float64            `json:"feels_like" description:"Perceived temperature in Celsius"`
	TempMin       float64            `json:"temp_min" description:"Minimum temperature in Celsius"`
	TempMax       float64            `json:"temp_max" description:"Maximum temperature in Celsius"`
	Pressure      int                `json:"pressure" description:"Atmospheric pressure in hPa"`
	Humidity      int                `json:"humidity" description:"Humidity percentage"`
	SeaLevel      int                `json:"sea_level,omitempty" description:"Sea level atmospheric pressure in hPa"`
	GrndLevel     int                `json:"grnd_level,omitempty" description:"Ground level atmospheric pressure in hPa"`
	Weather       []WeatherCondition `json:"weather" description:"Weather conditions"`
	Cloudiness    int                `json:"cloudiness" description:"Cloudiness percentage"`
	WindSpeed     float64            `json:"wind_speed" description:"Wind speed in meter/sec"`
	WindDeg       int                `json:"wind_deg" description:"Wind direction in degrees"`
	WindGust      float64            `json:"wind_gust,omitempty" description:"Wind gust in meter/sec"`
	Visibility    int                `json:"visibility" description:"Visibility in meters"`
	Pop           float64            `json:"pop" description:"Probability of precipitation (0 to 1)"`
	Precipitation *float64           `json:"precipitation,omitempty" description:"Rain volume for the last 3 hours in mm"`
	Snow          *float64           `json:"snow,omitempty" description:"Snow volume for the last 3 hours in mm"`
}

// ForecastCity contains city information from the forecast response.
type ForecastCity struct {
	Name     string      `json:"name" description:"City name"`
	Coord    Coordinates `json:"coord" description:"Geographic coordinates of the location"`
	Country  string      `json:"country" description:"Country code"`
	Timezone int         `json:"timezone" description:"Shift in seconds from UTC"`
	Sunrise  int64       `json:"sunrise" description:"Sunrise time, unix, UTC"`
	Sunset   int64       `json:"sunset" description:"Sunset time, unix, UTC"`
}

// ForecastToolOutput is the typed output for Forecast5Days3HoursWeatherTool.
type ForecastToolOutput struct {
	City      ForecastCity    `json:"city" description:"City information"`
	Count     int             `json:"cnt" description:"Number of forecast entries"`
	Forecasts []ForecastEntry `json:"forecasts" description:"3-hour forecast entries (up to 40 timestamps for 5 days)"`
}

// Forecast5Days3HoursWeatherTool implements domain.TypedTool for fetching 5-day/3-hour forecast via OpenWeatherMap.
type Forecast5Days3HoursWeatherTool struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewForecast5Days3HoursWeatherTool creates a Forecast5Days3HoursWeatherTool with the given API key.
func NewForecast5Days3HoursWeatherTool(apiKey string) domain.TypedTool[ForecastToolInput, ForecastToolOutput] {
	return &Forecast5Days3HoursWeatherTool{apiKey: apiKey, baseURL: defaultBaseURL, http: &http.Client{}}
}

var _ domain.TypedTool[ForecastToolInput, ForecastToolOutput] = (*Forecast5Days3HoursWeatherTool)(nil)

// newForecast5Days3HoursWeatherToolWithBaseURL creates a Forecast5Days3HoursWeatherTool with a custom base URL (for testing).
func newForecast5Days3HoursWeatherToolWithBaseURL(apiKey, baseURL string, client *http.Client) *Forecast5Days3HoursWeatherTool {
	return &Forecast5Days3HoursWeatherTool{apiKey: apiKey, baseURL: baseURL, http: client}
}

// Name returns the tool name as expected by the model.
func (t *Forecast5Days3HoursWeatherTool) Name() string { return "get_weather_forecast" }

// Description describes what the tool does.
func (t *Forecast5Days3HoursWeatherTool) Description() string {
	return "Get the weather forecast for the next 5 days with 3-hour intervals for a given location specified by latitude and longitude. Use the geocode_city tool first to convert a city name to coordinates. Returns up to 40 data points including temperature, humidity, wind, precipitation probability, and weather conditions. Use the optional 'count' parameter to limit the number of 3-hour timestamps returned."
}

// Call calls the OpenWeatherMap 5-day/3-hour forecast API and returns a typed output struct.
func (t *Forecast5Days3HoursWeatherTool) Call(ctx context.Context, input ForecastToolInput) (ForecastToolOutput, error) {
	if input.Lat == 0 && input.Lon == 0 {
		return ForecastToolOutput{}, fmt.Errorf("parameters 'lat' and 'lon' must not both be zero")
	}

	response, err := t.fetchForecast(ctx, input.Lat, input.Lon, input.Count)
	if err != nil {
		return ForecastToolOutput{}, err
	}

	out := ForecastToolOutput{
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
			entry.Precipitation = &item.Rain.ThreeH
		}
		if item.Snow != nil {
			entry.Snow = &item.Snow.ThreeH
		}

		out.Forecasts = append(out.Forecasts, entry)
	}

	return out, nil
}

type forecastResponse struct {
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
			ThreeH float64 `json:"3h"`
		} `json:"rain"`
		Snow *struct {
			ThreeH float64 `json:"3h"`
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

func (t *Forecast5Days3HoursWeatherTool) fetchForecast(ctx context.Context, lat, lon float64, count int) (*forecastResponse, error) {
	endpoint := fmt.Sprintf("%s/forecast?lat=%f&lon=%f&appid=%s&units=metric",
		t.baseURL, lat, lon, t.apiKey)

	if count > 0 {
		endpoint += fmt.Sprintf("&cnt=%d", count)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building forecast request: %w", err)
	}

	resp, err := t.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("forecast API request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("forecast API returned status %d", resp.StatusCode)
	}

	var data forecastResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding forecast response: %w", err)
	}
	return &data, nil
}
