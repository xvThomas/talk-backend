package testutils

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/joho/godotenv"
)

// findModuleRoot walks up from dir until it finds a go.mod file.
func findModuleRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("could not find go.mod in any parent directory")
		}
		dir = parent
	}
}

// GetProjectRoot returns the module root directory of the caller.
// It walks up from the caller's file location until it finds a go.mod file.
func GetProjectRoot() string {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("failed to get caller file path")
	}
	return findModuleRoot(filepath.Dir(filename))
}

// SetupTestEnv loads environment variables from .env.test file at the caller's module root.
// godotenv.Load() does NOT overwrite existing environment variables,
// so CI/CD variables (e.g., GitHub Secrets) take precedence automatically.
func SetupTestEnv(t testing.TB) {
	t.Helper()

	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("failed to get caller file path")
	}
	projectRoot := findModuleRoot(filepath.Dir(filename))

	_ = godotenv.Load(
		filepath.Join(projectRoot, ".env.test"),
	)
}

// SetupTestEnvWithRequiredVarsOrSkipTest loads environment variables and
// verifies required ones are set. Skips the test if any required variable is missing.
func SetupTestEnvWithRequiredVarsOrSkipTest(t testing.TB, requiredVars ...string) {
	t.Helper()

	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("failed to get caller file path")
	}
	projectRoot := findModuleRoot(filepath.Dir(filename))

	_ = godotenv.Load(
		filepath.Join(projectRoot, ".env.test"),
	)

	for _, varName := range requiredVars {
		if os.Getenv(varName) == "" {
			t.Skipf("%s not set in environment or .env files - skipping test", varName)
		}
	}
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
