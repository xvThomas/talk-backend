package openweather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"talks/internal/domain"
	"time"
)

// AirPollutionForecastToolInput is the typed input for AirPollutionForecastTool.
type AirPollutionForecastToolInput struct {
	Lat float64 `json:"lat" description:"Latitude of the location"`
	Lon float64 `json:"lon" description:"Longitude of the location"`
}

// AirPollutionForecastItem represents a single forecast data point.
type AirPollutionForecastItem struct {
	DateTime   string               `json:"dt" description:"Forecast time in ISO 8601 format"`
	AQI        int                  `json:"aqi" description:"Air Quality Index: 1=Good, 2=Fair, 3=Moderate, 4=Poor, 5=Very Poor"`
	Components AirQualityComponents `json:"components" description:"Concentrations of polluting gases in μg/m3"`
}

// AirPollutionForecastToolOutput is the typed output for AirPollutionForecastTool.
type AirPollutionForecastToolOutput struct {
	Items []AirPollutionForecastItem `json:"items" description:"Hourly air pollution forecast for the next 4 days"`
}

// AirPollutionForecastTool implements domain.TypedTool for fetching air pollution forecast data via OpenWeatherMap.
type AirPollutionForecastTool struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewAirPollutionForecastTool creates an AirPollutionForecastTool with the given API key.
func NewAirPollutionForecastTool(apiKey string) domain.TypedTool[AirPollutionForecastToolInput, AirPollutionForecastToolOutput] {
	return &AirPollutionForecastTool{apiKey: apiKey, baseURL: defaultBaseURL, http: &http.Client{}}
}

var _ domain.TypedTool[AirPollutionForecastToolInput, AirPollutionForecastToolOutput] = (*AirPollutionForecastTool)(nil)

// newAirPollutionForecastToolWithBaseURL creates an AirPollutionForecastTool with a custom base URL (for testing).
func newAirPollutionForecastToolWithBaseURL(apiKey, baseURL string, client *http.Client) *AirPollutionForecastTool {
	return &AirPollutionForecastTool{apiKey: apiKey, baseURL: baseURL, http: client}
}

// Name returns the tool name as expected by the model.
func (t *AirPollutionForecastTool) Name() string { return "get_air_pollution_forecast" }

// Description describes what the tool does.
func (t *AirPollutionForecastTool) Description() string {
	return "Get air pollution forecast for the next 4 days with hourly granularity for a given location specified by latitude and longitude. Use the geocode_city tool first to convert a city name to coordinates. Returns a list of hourly forecasts with Air Quality Index (1=Good to 5=Very Poor) and concentrations of polluting gases: CO, NO, NO2, O3, SO2, PM2.5, PM10, and NH3."
}

// Call calls the OpenWeatherMap Air Pollution Forecast API and returns a typed output struct.
func (t *AirPollutionForecastTool) Call(ctx context.Context, input AirPollutionForecastToolInput) (AirPollutionForecastToolOutput, error) {
	if input.Lat == 0 && input.Lon == 0 {
		return AirPollutionForecastToolOutput{}, fmt.Errorf("parameters 'lat' and 'lon' must not both be zero")
	}

	response, err := t.fetchForecast(ctx, input.Lat, input.Lon)
	if err != nil {
		return AirPollutionForecastToolOutput{}, err
	}

	if len(response.List) == 0 {
		return AirPollutionForecastToolOutput{}, fmt.Errorf("air pollution forecast API returned empty data")
	}

	items := make([]AirPollutionForecastItem, len(response.List))
	for i, entry := range response.List {
		items[i] = AirPollutionForecastItem{
			DateTime: time.Unix(entry.Dt, 0).UTC().Format(time.RFC3339),
			AQI:      entry.Main.AQI,
			Components: AirQualityComponents{
				CO:   entry.Components.CO,
				NO:   entry.Components.NO,
				NO2:  entry.Components.NO2,
				O3:   entry.Components.O3,
				SO2:  entry.Components.SO2,
				PM25: entry.Components.PM25,
				PM10: entry.Components.PM10,
				NH3:  entry.Components.NH3,
			},
		}
	}

	return AirPollutionForecastToolOutput{Items: items}, nil
}

func (t *AirPollutionForecastTool) fetchForecast(ctx context.Context, lat, lon float64) (*airPollutionResponse, error) {
	endpoint := fmt.Sprintf("%s/air_pollution/forecast?lat=%f&lon=%f&appid=%s",
		t.baseURL, lat, lon, t.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building air pollution forecast request: %w", err)
	}

	resp, err := t.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("air pollution forecast API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("air pollution forecast API returned status %d", resp.StatusCode)
	}

	var data airPollutionResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding air pollution forecast response: %w", err)
	}

	return &data, nil
}
