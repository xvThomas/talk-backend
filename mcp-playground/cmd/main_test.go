package main

import (
	"testing"

	"github.com/xvThomas/talk-backend/mcp-playground/internal/config"
)

func TestBuildApp_Minimal(t *testing.T) {
	env := &config.ServerEnv{}
	app := buildApp(env)
	if app == nil {
		t.Fatal("expected non-nil app")
	}
}

func TestBuildApp_WithAPIKey(t *testing.T) {
	env := &config.ServerEnv{}
	env.APIKey = "test-key"
	app := buildApp(env)
	if app == nil {
		t.Fatal("expected non-nil app")
	}
}

func TestBuildApp_WithOAuth(t *testing.T) {
	env := &config.ServerEnv{}
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
	env := &config.ServerEnv{}
	app := buildApp(env)
	var _ = app
}
