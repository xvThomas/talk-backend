# mcp-owm

MCP server exposing [OpenWeatherMap](https://openweathermap.org/) data (weather, forecasts, air quality, geocoding) as tools consumable by LLM agents.

## API Key

A free OpenWeatherMap API key is required.

1. Create an account at <https://home.openweathermap.org/users/sign_up>
2. Generate a key in **API keys** → <https://home.openweathermap.org/api_keys>
3. Set the key in your `.env` file (see below)

> The free plan gives access to most tools. Pro-only tools (hourly forecast, daily 16-day forecast) require a paid subscription.

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OPENWEATHERMAP_API_KEY` | **yes** | — | Your OpenWeatherMap API key |
| `OPENWEATHERMAP_FREE_PLAN` | no | `true` | Set to `false` to enable pro-only tools (hourly/daily forecasts) |

Example `.env`:

```env
OPENWEATHERMAP_API_KEY=your_key_here
OPENWEATHERMAP_FREE_PLAN=true
```

## Tools

### Free plan

| Tool | Description | Parameters |
|------|-------------|------------|
| `geocode` | Convert a city name to coordinates | `city` (string), `limit` (int, optional) |
| `reverse_geocode` | Convert coordinates to location names | `lat`, `lon` (float64), `limit` (int, optional) |
| `get_current_weather` | Current weather for a location | `lat`, `lon` (float64) |
| `get_weather_forecast` | 5-day / 3-hour forecast | `lat`, `lon` (float64), `count` (int, optional, 1–40) |
| `get_current_air_pollution` | Current Air Quality Index & pollutants | `lat`, `lon` (float64) |
| `get_air_pollution_forecast` | 4-day hourly air quality forecast | `lat`, `lon` (float64) |

### Pro plan only (`OPENWEATHERMAP_FREE_PLAN=false`)

| Tool | Description | Parameters |
|------|-------------|------------|
| `get_hourly_forecast` | Hourly forecast up to 96 hours | `lat`, `lon` (float64), `count` (int, optional, 1–96) |
| `get_daily_forecast` | Daily forecast up to 16 days | `lat`, `lon` (float64), `count` (int, optional, 1–16) |

## Prompts

| Prompt | Description | Arguments |
|--------|-------------|-----------|
| `current_weather` | Structured weather briefing for a city | `city` (required) |
| `current_air` | Structured air quality report for a city | `city` (required) |
| `forecast_weather` | Multi-day weather forecast summary | `city` (required) |
| `forecast_air` | Multi-day air quality forecast | `city` (required) |

## Run

```bash
make dev
```

## Authentication

This server supports **X-API-Key** and **OAuth 2.0** authentication.  
See [docs/mcp-server-authentication.md](../docs/mcp-server-authentication.md) for configuration details.
