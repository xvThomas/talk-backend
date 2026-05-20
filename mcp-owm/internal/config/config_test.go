package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadServerEnv_Success(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPENWEATHERMAP_API_KEY", "my-api-key")

	env, err := LoadServerEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.OpenWeatherMapAPIKey != "my-api-key" {
		t.Errorf("expected OpenWeatherMapAPIKey %q, got %q", "my-api-key", env.OpenWeatherMapAPIKey)
	}
	if !env.FreePlan {
		t.Error("expected FreePlan to default to true")
	}
	if env.RateLimitPerMinute != 60 {
		t.Errorf("expected default RateLimitPerMinute 60, got %d", env.RateLimitPerMinute)
	}
}

func TestLoadServerEnv_RateLimitCustom(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPENWEATHERMAP_API_KEY", "key")
	t.Setenv("OPENWEATHERMAP_RATE_LIMIT_PER_MINUTE", "600")

	env, err := LoadServerEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.RateLimitPerMinute != 600 {
		t.Errorf("expected RateLimitPerMinute 600, got %d", env.RateLimitPerMinute)
	}
}

func TestLoadServerEnv_RateLimitInvalid(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPENWEATHERMAP_API_KEY", "key")
	t.Setenv("OPENWEATHERMAP_RATE_LIMIT_PER_MINUTE", "not-a-number")

	env, err := LoadServerEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.RateLimitPerMinute != 60 {
		t.Errorf("expected default RateLimitPerMinute 60 for invalid value, got %d", env.RateLimitPerMinute)
	}
}

func TestLoadServerEnv_MissingAPIKey(t *testing.T) {
	clearEnv(t)

	_, err := LoadServerEnv()
	if err == nil {
		t.Fatal("expected error for missing OPENWEATHERMAP_API_KEY")
	}
}

func TestLoadServerEnv_FreePlanFalse(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPENWEATHERMAP_API_KEY", "key")
	t.Setenv("OPENWEATHERMAP_FREE_PLAN", "false")

	env, err := LoadServerEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.FreePlan {
		t.Error("expected FreePlan to be false")
	}
}

func TestLoadServerEnv_FreePlanCaseInsensitive(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPENWEATHERMAP_API_KEY", "key")
	t.Setenv("OPENWEATHERMAP_FREE_PLAN", "FALSE")

	env, err := LoadServerEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.FreePlan {
		t.Error("expected FreePlan to be false with uppercase FALSE")
	}
}

func TestLoadServerEnv_FreePlanOtherValues(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPENWEATHERMAP_API_KEY", "key")
	t.Setenv("OPENWEATHERMAP_FREE_PLAN", "true")

	env, err := LoadServerEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !env.FreePlan {
		t.Error("expected FreePlan to be true for value 'true'")
	}
}

func TestLoadServerEnv_WithEnvFile(t *testing.T) {
	clearEnv(t)

	envFile := writeEnvFile(t, `OPENWEATHERMAP_API_KEY=file-key
OPENWEATHERMAP_FREE_PLAN=false
X_API_KEY=auth-key
BASE_URL=http://localhost:8080
`)

	env, err := LoadServerEnv(envFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.OpenWeatherMapAPIKey != "file-key" {
		t.Errorf("expected OpenWeatherMapAPIKey %q, got %q", "file-key", env.OpenWeatherMapAPIKey)
	}
	if env.FreePlan {
		t.Error("expected FreePlan to be false from env file")
	}
	if env.APIKey != "auth-key" {
		t.Errorf("expected APIKey %q, got %q", "auth-key", env.APIKey)
	}
	if env.BaseURL != "http://localhost:8080" {
		t.Errorf("expected BaseURL %q, got %q", "http://localhost:8080", env.BaseURL)
	}
}

func TestLoadServerEnv_EnvVarsNotOverridden(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPENWEATHERMAP_API_KEY", "pre-existing")

	envFile := writeEnvFile(t, `OPENWEATHERMAP_API_KEY=from-file
`)

	env, err := LoadServerEnv(envFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.OpenWeatherMapAPIKey != "pre-existing" {
		t.Errorf("expected OpenWeatherMapAPIKey %q, got %q", "pre-existing", env.OpenWeatherMapAPIKey)
	}
}

func TestLoadServerEnv_MissingFileDoesNotError(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPENWEATHERMAP_API_KEY", "key")

	env, err := LoadServerEnv("/nonexistent/path/.env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.OpenWeatherMapAPIKey != "key" {
		t.Errorf("expected OpenWeatherMapAPIKey %q, got %q", "key", env.OpenWeatherMapAPIKey)
	}
}

func TestLoadServerEnv_BaseEnvOAuthFields(t *testing.T) {
	clearEnv(t)

	envFile := writeEnvFile(t, `OPENWEATHERMAP_API_KEY=key
OAUTH_AUTHORIZATION_SERVER=https://auth.example.com
OAUTH_AUDIENCE=my-api
OAUTH_SCOPES=read,write
`)

	env, err := LoadServerEnv(envFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.OAuthAuthorizationServer != "https://auth.example.com" {
		t.Errorf("expected OAuthAuthorizationServer %q, got %q", "https://auth.example.com", env.OAuthAuthorizationServer)
	}
	if env.OAuthAudience != "my-api" {
		t.Errorf("expected OAuthAudience %q, got %q", "my-api", env.OAuthAudience)
	}
	if env.OAuthScopes != "read,write" {
		t.Errorf("expected OAuthScopes %q, got %q", "read,write", env.OAuthScopes)
	}
}

// clearEnv unsets all environment variables read by LoadServerEnv.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"OPENWEATHERMAP_API_KEY", "OPENWEATHERMAP_FREE_PLAN",
		"OPENWEATHERMAP_RATE_LIMIT_PER_MINUTE",
		"X_API_KEY", "BASE_URL",
		"OAUTH_AUTHORIZATION_SERVER", "OAUTH_AUDIENCE",
		"OAUTH_SCOPES", "OAUTH_CLIENT_SECRET",
	} {
		t.Setenv(key, "")
		_ = os.Unsetenv(key)
	}
}

// writeEnvFile creates a temporary .env file and returns its path.
func writeEnvFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}
	return path
}
