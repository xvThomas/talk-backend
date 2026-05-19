package testutils

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/joho/godotenv"
)

// GetProjectRoot returns the absolute path to the mcp-owm module root directory.
func GetProjectRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to get current file path")
	}
	// From mcp-owm/internal/testutils/ go up 3 levels to mcp-owm/
	dir := filepath.Dir(filename)
	return filepath.Join(dir, "..", "..", "..")
}

// SetupTestEnv loads environment variables from .env.test file in priority order.
// godotenv.Load() does NOT overwrite existing environment variables,
// so CI/CD variables (e.g., GitHub Secrets) take precedence automatically.
func SetupTestEnv(t testing.TB) {
	t.Helper()

	projectRoot := GetProjectRoot()

	_ = godotenv.Load(
		filepath.Join(projectRoot, ".env.test"),
	)
}

// RequireEnv skips the test if the given environment variable is not set.
func RequireEnv(t testing.TB, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("skipping: %s not set", key)
	}
	return v
}
