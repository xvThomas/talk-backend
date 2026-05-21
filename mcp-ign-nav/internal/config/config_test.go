package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadServerEnv_NoFileNoVars(t *testing.T) {
	clearEnv(t)

	env, err := LoadServerEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.APIKey != "" {
		t.Errorf("expected empty APIKey, got %q", env.APIKey)
	}
	if env.BaseURL != "" {
		t.Errorf("expected empty BaseURL, got %q", env.BaseURL)
	}
	if env.OAuthAuthorizationServer != "" {
		t.Errorf("expected empty OAuthAuthorizationServer, got %q", env.OAuthAuthorizationServer)
	}
	if env.GetGeoJSONGeometry {
		t.Error("expected GetGeoJSONGeometry to be false by default")
	}
}

func TestLoadServerEnv_GetGeoJSONGeometry(t *testing.T) {
	clearEnv(t)
	t.Setenv("GET_GEOJSON_GEOMETRY", "true")

	env, err := LoadServerEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !env.GetGeoJSONGeometry {
		t.Error("expected GetGeoJSONGeometry to be true")
	}
}

func TestLoadServerEnv_WithEnvFile(t *testing.T) {
	clearEnv(t)

	envFile := writeEnvFile(t, `X_API_KEY=test-key
BASE_URL=http://localhost:8080
OAUTH_AUTHORIZATION_SERVER=https://auth.example.com
OAUTH_AUDIENCE=my-api
OAUTH_SCOPES=read,write
`)

	env, err := LoadServerEnv(envFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.APIKey != "test-key" {
		t.Errorf("expected APIKey %q, got %q", "test-key", env.APIKey)
	}
	if env.BaseURL != "http://localhost:8080" {
		t.Errorf("expected BaseURL %q, got %q", "http://localhost:8080", env.BaseURL)
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

func TestLoadServerEnv_MissingFileDoesNotError(t *testing.T) {
	clearEnv(t)

	env, err := LoadServerEnv("/nonexistent/path/.env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.APIKey != "" {
		t.Errorf("expected empty APIKey, got %q", env.APIKey)
	}
}

func TestLoadServerEnv_EnvVarsNotOverridden(t *testing.T) {
	clearEnv(t)
	t.Setenv("X_API_KEY", "pre-existing")

	envFile := writeEnvFile(t, `X_API_KEY=from-file
`)

	env, err := LoadServerEnv(envFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.APIKey != "pre-existing" {
		t.Errorf("expected APIKey %q, got %q", "pre-existing", env.APIKey)
	}
}

func TestLoadServerEnv_MultipleFiles(t *testing.T) {
	clearEnv(t)

	file1 := writeEnvFile(t, `X_API_KEY=key1
`)
	file2 := writeEnvFile(t, `BASE_URL=http://localhost:9090
`)

	env, err := LoadServerEnv(file1, file2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.APIKey != "key1" {
		t.Errorf("expected APIKey %q, got %q", "key1", env.APIKey)
	}
	if env.BaseURL != "http://localhost:9090" {
		t.Errorf("expected BaseURL %q, got %q", "http://localhost:9090", env.BaseURL)
	}
}

func TestLoadServerEnv_OAuthScopesList(t *testing.T) {
	clearEnv(t)

	envFile := writeEnvFile(t, `OAUTH_SCOPES=read, write, admin
`)

	env, err := LoadServerEnv(envFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	scopes := env.OAuthScopesList()
	expected := []string{"read", "write", "admin"}
	if len(scopes) != len(expected) {
		t.Fatalf("expected %d scopes, got %d", len(expected), len(scopes))
	}
	for i, s := range scopes {
		if s != expected[i] {
			t.Errorf("scope[%d]: expected %q, got %q", i, expected[i], s)
		}
	}
}

// clearEnv unsets all environment variables read by LoadServerEnv.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"X_API_KEY", "BASE_URL",
		"OAUTH_AUTHORIZATION_SERVER", "OAUTH_AUDIENCE",
		"OAUTH_SCOPES", "OAUTH_CLIENT_SECRET",
		"GET_GEOJSON_GEOMETRY",
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
