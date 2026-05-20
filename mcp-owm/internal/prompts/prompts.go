package prompts

import "github.com/xvThomas/LLMClientWrapper/talk-libs/mcpserver"

// CurrentWeather instructs the LLM to provide a structured weather briefing.
var CurrentWeather = mcpserver.Prompt{
	Name:        "current_weather",
	Description: "Get a structured weather briefing for a city using real-time data",
	Arguments: []mcpserver.PromptArgument{
		{Name: "city", Description: "The city name to get weather for", Required: true},
	},
	Messages: []mcpserver.PromptMessage{
		{
			Role: "user",
			Text: "Give me the current weather for {{city}}. " +
				"First use geocode_city to get the coordinates, then call get_current_weather. " +
				"Present the result as: " +
				"1) Current conditions (temperature, feels like, humidity) " +
				"2) Wind (speed, direction, gusts) " +
				"3) Visibility and cloud cover " +
				"4) A short human-friendly summary.",
		},
	},
}

// CurrentAir instructs the LLM to provide a structured air quality report.
var CurrentAir = mcpserver.Prompt{
	Name:        "current_air",
	Description: "Get a structured air quality report for a city using real-time data",
	Arguments: []mcpserver.PromptArgument{
		{Name: "city", Description: "The city name to get air quality for", Required: true},
	},
	Messages: []mcpserver.PromptMessage{
		{
			Role: "user",
			Text: "Give me the current air quality for {{city}}. " +
				"First use geocode_city to get the coordinates, then call get_current_air_pollution. " +
				"Present the result as: " +
				"1) Overall Air Quality Index with human-readable label (Good/Fair/Moderate/Poor/Very Poor) " +
				"2) Key pollutants (PM2.5, PM10, O3, NO2) with their concentrations " +
				"3) Health recommendations based on the AQI level.",
		},
	},
}

// ForecastWeather instructs the LLM to provide a multi-day weather forecast.
var ForecastWeather = mcpserver.Prompt{
	Name:        "forecast_weather",
	Description: "Get a weather forecast summary for a city over the next days",
	Arguments: []mcpserver.PromptArgument{
		{Name: "city", Description: "The city name to get the forecast for", Required: true},
	},
	Messages: []mcpserver.PromptMessage{
		{
			Role: "user",
			Text: "Give me the weather forecast for {{city}} over the next few days. " +
				"First use geocode_city to get the coordinates, then call get_weather_forecast. " +
				"Present the result as: " +
				"1) A day-by-day summary with high/low temperatures and general conditions " +
				"2) Notable weather events (rain, storms, snow) with timing " +
				"3) Wind trends " +
				"4) A brief overall outlook.",
		},
	},
}

// ForecastAir instructs the LLM to provide a multi-day air quality forecast.
var ForecastAir = mcpserver.Prompt{
	Name:        "forecast_air",
	Description: "Get an air quality forecast for a city over the next days",
	Arguments: []mcpserver.PromptArgument{
		{Name: "city", Description: "The city name to get the air quality forecast for", Required: true},
	},
	Messages: []mcpserver.PromptMessage{
		{
			Role: "user",
			Text: "Give me the air quality forecast for {{city}} over the next few days. " +
				"First use geocode_city to get the coordinates, then call get_air_pollution_forecast. " +
				"Present the result as: " +
				"1) A day-by-day AQI trend with human-readable labels (Good/Fair/Moderate/Poor/Very Poor) " +
				"2) Key pollutants evolution (PM2.5, O3) over the period " +
				"3) Best and worst periods for outdoor activities " +
				"4) Health recommendations for sensitive groups.",
		},
	},
}
