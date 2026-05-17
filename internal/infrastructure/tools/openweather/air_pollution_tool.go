package openweather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"talks/internal/domain"
	"time"
)

// AirPollutionToolInput is the typed input for AirPollutionTool.
type AirPollutionToolInput struct {
	Lat float64 `json:"lat" description:"Latitude of the location"`
	Lon float64 `json:"lon" description:"Longitude of the location"`
}

// AirQualityComponents contains concentrations of polluting gases in μg/m3.
type AirQualityComponents struct {
	CO   float64 `json:"co" description:"Concentration of Carbon monoxide (CO) in μg/m3"`
	NO   float64 `json:"no" description:"Concentration of Nitrogen monoxide (NO) in μg/m3"`
	NO2  float64 `json:"no2" description:"Concentration of Nitrogen dioxide (NO2) in μg/m3"`
	O3   float64 `json:"o3" description:"Concentration of Ozone (O3) in μg/m3"`
	SO2  float64 `json:"so2" description:"Concentration of Sulphur dioxide (SO2) in μg/m3"`
	PM25 float64 `json:"pm2_5" description:"Concentration of fine particles PM2.5 in μg/m3"`
	PM10 float64 `json:"pm10" description:"Concentration of coarse particles PM10 in μg/m3"`
	NH3  float64 `json:"nh3" description:"Concentration of Ammonia (NH3) in μg/m3"`
}

// AirPollutionToolOutput is the typed output for AirPollutionTool.
type AirPollutionToolOutput struct {
	DateTime   string               `json:"dt" description:"Measurement time in ISO 8601 format"`
	AQI        int                  `json:"aqi" description:"Air Quality Index: 1=Good, 2=Fair, 3=Moderate, 4=Poor, 5=Very Poor"`
	Components AirQualityComponents `json:"components" description:"Concentrations of polluting gases in μg/m3"`
}

// AirPollutionTool implements domain.TypedTool for fetching current air pollution data via OpenWeatherMap.
type AirPollutionTool struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewAirPollutionTool creates an AirPollutionTool with the given API key.
func NewAirPollutionTool(apiKey string) domain.TypedTool[AirPollutionToolInput, AirPollutionToolOutput] {
	return &AirPollutionTool{apiKey: apiKey, baseURL: defaultBaseURL, http: &http.Client{}}
}

var _ domain.TypedTool[AirPollutionToolInput, AirPollutionToolOutput] = (*AirPollutionTool)(nil)

// newAirPollutionToolWithBaseURL creates an AirPollutionTool with a custom base URL (for testing).
func newAirPollutionToolWithBaseURL(apiKey, baseURL string, client *http.Client) *AirPollutionTool {
	return &AirPollutionTool{apiKey: apiKey, baseURL: baseURL, http: client}
}

// Name returns the tool name as expected by the model.
func (t *AirPollutionTool) Name() string { return "get_current_air_pollution" }

// Description describes what the tool does.
func (t *AirPollutionTool) Description() string {
	return "Get current air pollution data for a given location specified by latitude and longitude. Use the geocode_city tool first to convert a city name to coordinates. Returns the Air Quality Index (1=Good to 5=Very Poor) and concentrations of polluting gases: CO, NO, NO2, O3, SO2, PM2.5, PM10, and NH3."
}

// Call calls the OpenWeatherMap Air Pollution API and returns a typed output struct.
func (t *AirPollutionTool) Call(ctx context.Context, input AirPollutionToolInput) (AirPollutionToolOutput, error) {
	if input.Lat == 0 && input.Lon == 0 {
		return AirPollutionToolOutput{}, fmt.Errorf("parameters 'lat' and 'lon' must not both be zero")
	}

	response, err := t.fetchAirPollution(ctx, input.Lat, input.Lon)
	if err != nil {
		return AirPollutionToolOutput{}, err
	}

	if len(response.List) == 0 {
		return AirPollutionToolOutput{}, fmt.Errorf("air pollution API returned empty data")
	}

	item := response.List[0]
	return AirPollutionToolOutput{
		DateTime: time.Unix(item.Dt, 0).UTC().Format(time.RFC3339),
		AQI:      item.Main.AQI,
		Components: AirQualityComponents{
			CO:   item.Components.CO,
			NO:   item.Components.NO,
			NO2:  item.Components.NO2,
			O3:   item.Components.O3,
			SO2:  item.Components.SO2,
			PM25: item.Components.PM25,
			PM10: item.Components.PM10,
			NH3:  item.Components.NH3,
		},
	}, nil
}

type airPollutionResponse struct {
	List []struct {
		Dt   int64 `json:"dt"`
		Main struct {
			AQI int `json:"aqi"`
		} `json:"main"`
		Components struct {
			CO   float64 `json:"co"`
			NO   float64 `json:"no"`
			NO2  float64 `json:"no2"`
			O3   float64 `json:"o3"`
			SO2  float64 `json:"so2"`
			PM25 float64 `json:"pm2_5"`
			PM10 float64 `json:"pm10"`
			NH3  float64 `json:"nh3"`
		} `json:"components"`
	} `json:"list"`
}

func (t *AirPollutionTool) fetchAirPollution(ctx context.Context, lat, lon float64) (*airPollutionResponse, error) {
	endpoint := fmt.Sprintf("%s/air_pollution?lat=%f&lon=%f&appid=%s",
		t.baseURL, lat, lon, t.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building air pollution request: %w", err)
	}

	resp, err := t.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("air pollution API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("air pollution API returned status %d", resp.StatusCode)
	}

	var data airPollutionResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding air pollution response: %w", err)
	}

	return &data, nil
}
