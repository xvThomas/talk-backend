package testutils

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/joho/godotenv"
)

// getProjectRoot returns the absolute path to the project root directory
func GetProjectRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to get current file path")
	}
	// From talk/internal/helpers/testutils/ go up 4 levels to talk/ module root
	dir := filepath.Dir(filename)
	return filepath.Join(dir, "..", "..", "..", "..")
}

// SetupTestEnv loads environment variables from .env.test file in priority order
// godotenv.Load() does NOT overwrite existing environment variables,
// so CI/CD variables (e.g., GitHub Secrets) take precedence automatically
//
// Priority order:
// 1. Environment variables (CI/CD secrets) - never overwritten
// 2. .env.test - for test-specific variables
func SetupTestEnv(t testing.TB) {
	t.Helper()

	projectRoot := GetProjectRoot()

	// Load env files in priority order
	// Only loads variables that are not already set in environment
	_ = godotenv.Load(
		filepath.Join(projectRoot, ".env.test"),
	)
}

// SetupTestEnvWithRequiredVarsOrSkipTest loads environment variables and
// verifies required ones are set.
// Skips the test if any required variable is missing
func SetupTestEnvWithRequiredVarsOrSkipTest(t testing.TB, requiredVars ...string) {
	t.Helper()

	SetupTestEnv(t)

	// Verify that all required variables are set, otherwise skip the test
	for _, varName := range requiredVars {
		if os.Getenv(varName) == "" {
			t.Skipf("%s not set in environment or .env files - skipping test", varName)
		}
	}
}

// GetAnthropicTestModel returns the Anthropic model name used for testing
func GetAnthropicTestModel() string {
	return "claude-haiku-4-5-20251001"
}
