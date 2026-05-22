package main

import (
	"testing"

	"github.com/xvThomas/LLMClientWrapper/mcp-owm/internal/config"
)

func TestBuildApp_FreePlan(t *testing.T) {
	env := &config.ServerEnv{
		OpenWeatherMapAPIKey: "test-key",
		FreePlan:             true,
		RateLimitPerMinute:   60,
	}
	app := buildApp(env)
	if app == nil {
		t.Fatal("expected non-nil app")
	}
}

func TestBuildApp_ProPlan(t *testing.T) {
	env := &config.ServerEnv{
		OpenWeatherMapAPIKey: "test-key",
		FreePlan:             false,
		RateLimitPerMinute:   60,
	}
	app := buildApp(env)
	if app == nil {
		t.Fatal("expected non-nil app")
	}
}

func TestBuildApp_WithAPIKey(t *testing.T) {
	env := &config.ServerEnv{
		OpenWeatherMapAPIKey: "test-key",
		FreePlan:             true,
		RateLimitPerMinute:   60,
	}
	env.APIKey = "mcp-api-key"
	app := buildApp(env)
	if app == nil {
		t.Fatal("expected non-nil app")
	}
}

func TestBuildApp_WithOAuth(t *testing.T) {
	env := &config.ServerEnv{
		OpenWeatherMapAPIKey: "test-key",
		FreePlan:             true,
		RateLimitPerMinute:   60,
	}
	env.OAuthAuthorizationServer = "https://auth.example.com"
	env.OAuthAudience = "my-api"
	env.OAuthScopes = "read,write"
	env.BaseURL = "https://mcp.example.com"
	app := buildApp(env)
	if app == nil {
		t.Fatal("expected non-nil app")
	}
}

func TestBuildApp_ReturnsCorrectType(t *testing.T) {
	env := &config.ServerEnv{
		OpenWeatherMapAPIKey: "test-key",
		FreePlan:             true,
		RateLimitPerMinute:   60,
	}
	app := buildApp(env)
	var _ = app
}
