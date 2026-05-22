package mcpserver

import (
	"os"
	"testing"
	"time"
)

func TestLoadBaseEnv_Defaults(t *testing.T) {
	// Clear all relevant env vars to test defaults.
	keys := []string{
		"X_API_KEY", "BASE_URL", "OAUTH_AUTHORIZATION_SERVER",
		"OAUTH_AUDIENCE", "OAUTH_SCOPES", "OAUTH_CLIENT_SECRET",
		"HTTP_RATE_LIMIT", "HTTP_RATE_BURST",
		"HTTP_READ_TIMEOUT", "HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT",
		"HTTP_TRUSTED_PROXIES",
	}
	for _, k := range keys {
		t.Setenv(k, "")
	}

	env := LoadBaseEnv()

	if env.APIKey != "" {
		t.Errorf("expected empty APIKey, got %q", env.APIKey)
	}
	if env.HTTPRateLimit != 50 {
		t.Errorf("expected HTTPRateLimit 50, got %d", env.HTTPRateLimit)
	}
	if env.HTTPRateBurst != 50 {
		t.Errorf("expected HTTPRateBurst 50, got %d", env.HTTPRateBurst)
	}
	if env.HTTPReadTimeout != 10*time.Second {
		t.Errorf("expected HTTPReadTimeout 10s, got %v", env.HTTPReadTimeout)
	}
	if env.HTTPWriteTimeout != 30*time.Second {
		t.Errorf("expected HTTPWriteTimeout 30s, got %v", env.HTTPWriteTimeout)
	}
	if env.HTTPIdleTimeout != 60*time.Second {
		t.Errorf("expected HTTPIdleTimeout 60s, got %v", env.HTTPIdleTimeout)
	}
	if len(env.TrustedProxies) != 0 {
		t.Errorf("expected no TrustedProxies, got %d", len(env.TrustedProxies))
	}
}

func TestLoadBaseEnv_AllSet(t *testing.T) {
	t.Setenv("X_API_KEY", "test-key")
	t.Setenv("BASE_URL", "https://example.com")
	t.Setenv("OAUTH_AUTHORIZATION_SERVER", "https://auth.example.com")
	t.Setenv("OAUTH_AUDIENCE", "my-api")
	t.Setenv("OAUTH_SCOPES", "read,write")
	t.Setenv("OAUTH_CLIENT_SECRET", "s3cret")
	t.Setenv("HTTP_RATE_LIMIT", "100")
	t.Setenv("HTTP_RATE_BURST", "200")
	t.Setenv("HTTP_READ_TIMEOUT", "5")
	t.Setenv("HTTP_WRITE_TIMEOUT", "15")
	t.Setenv("HTTP_IDLE_TIMEOUT", "30")
	t.Setenv("HTTP_TRUSTED_PROXIES", "10.0.0.1,172.16.0.0/12")

	env := LoadBaseEnv()

	if env.APIKey != "test-key" {
		t.Errorf("expected APIKey %q, got %q", "test-key", env.APIKey)
	}
	if env.BaseURL != "https://example.com" {
		t.Errorf("expected BaseURL %q, got %q", "https://example.com", env.BaseURL)
	}
	if env.OAuthAuthorizationServer != "https://auth.example.com" {
		t.Errorf("expected OAuthAuthorizationServer %q, got %q", "https://auth.example.com", env.OAuthAuthorizationServer)
	}
	if env.OAuthAudience != "my-api" {
		t.Errorf("expected OAuthAudience %q, got %q", "my-api", env.OAuthAudience)
	}
	if env.OAuthClientSecret != "s3cret" {
		t.Errorf("expected OAuthClientSecret %q, got %q", "s3cret", env.OAuthClientSecret)
	}
	if env.HTTPRateLimit != 100 {
		t.Errorf("expected HTTPRateLimit 100, got %d", env.HTTPRateLimit)
	}
	if env.HTTPRateBurst != 200 {
		t.Errorf("expected HTTPRateBurst 200, got %d", env.HTTPRateBurst)
	}
	if env.HTTPReadTimeout != 5*time.Second {
		t.Errorf("expected HTTPReadTimeout 5s, got %v", env.HTTPReadTimeout)
	}
	if env.HTTPWriteTimeout != 15*time.Second {
		t.Errorf("expected HTTPWriteTimeout 15s, got %v", env.HTTPWriteTimeout)
	}
	if env.HTTPIdleTimeout != 30*time.Second {
		t.Errorf("expected HTTPIdleTimeout 30s, got %v", env.HTTPIdleTimeout)
	}
	if len(env.TrustedProxies) != 2 {
		t.Fatalf("expected 2 TrustedProxies, got %d", len(env.TrustedProxies))
	}
}

func TestEnvInt_Default(t *testing.T) {
	t.Setenv("TEST_ENV_INT", "")
	got := envInt("TEST_ENV_INT", 42)
	if got != 42 {
		t.Errorf("expected default 42, got %d", got)
	}
}

func TestEnvInt_Valid(t *testing.T) {
	t.Setenv("TEST_ENV_INT", "99")
	got := envInt("TEST_ENV_INT", 42)
	if got != 99 {
		t.Errorf("expected 99, got %d", got)
	}
}

func TestEnvInt_Invalid(t *testing.T) {
	t.Setenv("TEST_ENV_INT", "not-a-number")
	got := envInt("TEST_ENV_INT", 42)
	if got != 42 {
		t.Errorf("expected default 42 for invalid input, got %d", got)
	}
}

func TestOAuthScopesList_Empty(t *testing.T) {
	env := BaseEnv{OAuthScopes: ""}
	got := env.OAuthScopesList()
	if got != nil {
		t.Errorf("expected nil for empty scopes, got %v", got)
	}
}

func TestOAuthScopesList_Single(t *testing.T) {
	env := BaseEnv{OAuthScopes: "read"}
	got := env.OAuthScopesList()
	if len(got) != 1 || got[0] != "read" {
		t.Errorf("expected [read], got %v", got)
	}
}

func TestOAuthScopesList_Multiple(t *testing.T) {
	env := BaseEnv{OAuthScopes: "read, write , admin"}
	got := env.OAuthScopesList()
	expected := []string{"read", "write", "admin"}
	if len(got) != len(expected) {
		t.Fatalf("expected %d scopes, got %d: %v", len(expected), len(got), got)
	}
	for i, want := range expected {
		if got[i] != want {
			t.Errorf("scope[%d]: expected %q, got %q", i, want, got[i])
		}
	}
}

// Suppress unused import warning for os (used by t.Setenv internally).
var _ = os.Getenv
