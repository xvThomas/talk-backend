package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/xvThomas/LLMClientWrapper/talk-libs/mcpserver"
)

const defaultBaseURL = "https://api.openweathermap.org/data/2.5"

// CurrentWeatherToolInput is the typed input for CurrentWeatherTool.
type CurrentWeatherToolInput struct {
	Lat float64 `json:"lat" description:"Latitude of the location"`
	Lon float64 `json:"lon" description:"Longitude of the location"`
}

// WeatherCondition contains weather condition details.
type WeatherCondition struct {
	Main        string `json:"main" description:"Group of weather parameters (Rain, Snow, Extreme etc.)"`
	Description string `json:"description" description:"Weather condition within the group"`
}

// Coordinates contains geographic coordinates.
type Coordinates struct {
	Lon float64 `json:"lon" description:"Longitude of the location"`
	Lat float64 `json:"lat" description:"Latitude of the location"`
}

// WindData contains wind measurements.
type WindData struct {
	Speed float64 `json:"speed" description:"Wind speed in meter/sec"`
	Deg   int     `json:"deg" description:"Wind direction in degrees"`
	Gust  float64 `json:"gust,omitempty" description:"Wind gust in meter/sec"`
}

// SysData contains country and sun timing data.
type SysData struct {
	Country string `json:"country" description:"Country code"`
	Sunrise int64  `json:"sunrise" description:"Sunrise time, unix, UTC"`
	Sunset  int64  `json:"sunset" description:"Sunset time, unix, UTC"`
}

// CurrentWeatherToolOutput is the typed output for CurrentWeatherTool.
type CurrentWeatherToolOutput struct {
	Coord         Coordinates        `json:"coord" description:"Geographic coordinates of the location"`
	Weather       []WeatherCondition `json:"weather" description:"Weather conditions"`
	Temp          float64            `json:"temp" description:"Current temperature in Celsius"`
	FeelsLike     float64            `json:"feels_like" description:"Perceived temperature in Celsius"`
	TempMin       float64            `json:"temp_min" description:"Minimum temperature in Celsius"`
	TempMax       float64            `json:"temp_max" description:"Maximum temperature in Celsius"`
	Pressure      int                `json:"pressure" description:"Atmospheric pressure in hPa"`
	Humidity      int                `json:"humidity" description:"Humidity percentage"`
	SeaLevel      int                `json:"sea_level,omitempty" description:"Sea level atmospheric pressure in hPa"`
	GrndLevel     int                `json:"grnd_level,omitempty" description:"Ground level atmospheric pressure in hPa"`
	Visibility    int                `json:"visibility" description:"Visibility in meters"`
	WindSpeed     float64            `json:"wind_speed" description:"Wind speed in meter/sec"`
	WindDeg       int                `json:"wind_deg" description:"Wind direction in degrees"`
	WindGust      float64            `json:"wind_gust,omitempty" description:"Wind gust in meter/sec"`
	Cloudiness    int                `json:"cloudiness" description:"Cloudiness percentage"`
	Precipitation *float64           `json:"precipitation,omitempty" description:"Precipitation volume for the last 1 hour in mm"`
	Snow          *float64           `json:"snow,omitempty" description:"Snow volume for the last 1 hour in mm"`
	DateTime      string             `json:"dt" description:"Data calculation time in ISO 8601 format (e.g. 2026-03-30T15:22:26Z)"`
	Sys           SysData            `json:"sys" description:"System data"`
	Timezone      int                `json:"timezone" description:"Shift in seconds from UTC"`
	Name          string             `json:"name" description:"City name"`
}

// CurrentWeatherTool implements mcpserver.MCPTool for fetching current weather via OpenWeatherMap.
type CurrentWeatherTool struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewCurrentWeatherTool creates a CurrentWeatherTool with the given API key.
func NewCurrentWeatherTool(apiKey string) mcpserver.MCPTool[CurrentWeatherToolInput, CurrentWeatherToolOutput] {
	return &CurrentWeatherTool{apiKey: apiKey, baseURL: defaultBaseURL, http: &http.Client{}}
}

var _ mcpserver.MCPTool[CurrentWeatherToolInput, CurrentWeatherToolOutput] = (*CurrentWeatherTool)(nil)

// newCurrentWeatherToolWithBaseURL creates a CurrentWeatherTool with a custom base URL (for testing).
func newCurrentWeatherToolWithBaseURL(apiKey, baseURL string, client *http.Client) *CurrentWeatherTool {
	return &CurrentWeatherTool{apiKey: apiKey, baseURL: baseURL, http: client}
}

// Name returns the tool name as expected by the model.
func (t *CurrentWeatherTool) Name() string { return "get_current_weather" }

// Description describes what the tool does.
func (t *CurrentWeatherTool) Description() string {
	return "Get the current weather for a given location specified by latitude and longitude. Use the geocode_city tool first to convert a city name to coordinates. Returns live data: coordinates, temperature, feels-like, min/max temp, humidity, pressure, wind speed and direction, cloudiness, visibility, precipitation, and sunrise/sunset times."
}

// Call calls the OpenWeatherMap API and returns a typed output struct.
func (t *CurrentWeatherTool) Call(ctx context.Context, input CurrentWeatherToolInput) (CurrentWeatherToolOutput, error) {
	if input.Lat == 0 && input.Lon == 0 {
		return CurrentWeatherToolOutput{}, fmt.Errorf("parameters 'lat' and 'lon' must not both be zero")
	}

	response, err := t.fetchWeather(ctx, input.Lat, input.Lon)
	if err != nil {
		return CurrentWeatherToolOutput{}, err
	}

	out := CurrentWeatherToolOutput{
		Coord:      Coordinates{Lon: response.Coord.Lon, Lat: response.Coord.Lat},
		Temp:       response.Main.Temp,
		FeelsLike:  response.Main.FeelsLike,
		TempMin:    response.Main.TempMin,
		TempMax:    response.Main.TempMax,
		Pressure:   response.Main.Pressure,
		Humidity:   response.Main.Humidity,
		SeaLevel:   response.Main.SeaLevel,
		GrndLevel:  response.Main.GrndLevel,
		Visibility: response.Visibility,
		WindSpeed:  response.Wind.Speed,
		WindDeg:    response.Wind.Deg,
		WindGust:   response.Wind.Gust,
		Cloudiness: response.Clouds.All,
		DateTime:   time.Unix(response.Dt, 0).UTC().Format(time.RFC3339),
		Sys: SysData{
			Country: response.Sys.Country,
			Sunrise: response.Sys.Sunrise,
			Sunset:  response.Sys.Sunset,
		},
		Timezone: response.Timezone,
		Name:     response.Name,
	}

	out.Weather = make([]WeatherCondition, 0, len(response.Weather))
	for _, w := range response.Weather {
		out.Weather = append(out.Weather, WeatherCondition{
			Main:        w.Main,
			Description: w.Description,
		})
	}

	if response.Rain != nil {
		out.Precipitation = &response.Rain.OneH
	}
	if response.Snow != nil {
		out.Snow = &response.Snow.OneH
	}

	return out, nil
}

type weatherResponse struct {
	Coord struct {
		Lon float64 `json:"lon"`
		Lat float64 `json:"lat"`
	} `json:"coord"`
	Weather []struct {
		ID          int    `json:"id"`
		Main        string `json:"main"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	} `json:"weather"`
	Base string `json:"base"`
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		Pressure  int     `json:"pressure"`
		Humidity  int     `json:"humidity"`
		TempMin   float64 `json:"temp_min"`
		TempMax   float64 `json:"temp_max"`
		SeaLevel  int     `json:"sea_level"`
		GrndLevel int     `json:"grnd_level"`
	} `json:"main"`
	Visibility int `json:"visibility"`
	Wind       struct {
		Speed float64 `json:"speed"`
		Deg   int     `json:"deg"`
		Gust  float64 `json:"gust"`
	} `json:"wind"`
	Clouds struct {
		All int `json:"all"`
	} `json:"clouds"`
	Rain *struct {
		OneH float64 `json:"1h"`
	} `json:"rain"`
	Snow *struct {
		OneH float64 `json:"1h"`
	} `json:"snow"`
	Dt  int64 `json:"dt"`
	Sys struct {
		Type    int    `json:"type"`
		ID      int    `json:"id"`
		Country string `json:"country"`
		Sunrise int64  `json:"sunrise"`
		Sunset  int64  `json:"sunset"`
	} `json:"sys"`
	Timezone int    `json:"timezone"`
	ID       int    `json:"id"`
	Name     string `json:"name"`
}

func (t *CurrentWeatherTool) fetchWeather(ctx context.Context, lat, lon float64) (*weatherResponse, error) {
	endpoint := fmt.Sprintf("%s/weather?lat=%f&lon=%f&appid=%s&units=metric",
		t.baseURL, lat, lon, t.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building weather request: %w", err)
	}

	resp, err := t.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weather API request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API returned status %d", resp.StatusCode)
	}

	var data weatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding weather response: %w", err)
	}
	return &data, nil
}
