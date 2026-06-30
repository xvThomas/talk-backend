package config

import (
	"errors"
	"testing"
)

func TestParseToolsMaxConcurrent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "empty defaults", input: "", want: 4},
		{name: "valid positive", input: "8", want: 8},
		{name: "zero fallback", input: "0", want: 4},
		{name: "negative fallback", input: "-2", want: 4},
		{name: "invalid fallback", input: "abc", want: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseToolsMaxConcurrent(tt.input)
			if got != tt.want {
				t.Fatalf("parseToolsMaxConcurrent(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseContextFullTurns(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "empty defaults lean", input: "", want: 0},
		{name: "full mode", input: "-1", want: -1},
		{name: "lean mode", input: "0", want: 0},
		{name: "hybrid mode", input: "3", want: 3},
		{name: "invalid fallback full", input: "abc", want: -1},
		{name: "lower than -1 fallback full", input: "-3", want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseContextFullTurns(tt.input)
			if got != tt.want {
				t.Fatalf("parseContextFullTurns(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseLangfuseBaseURL(t *testing.T) {
	if got := parseLangfuseBaseURL(""); got != "https://cloud.langfuse.com" {
		t.Fatalf("default base URL = %q, want %q", got, "https://cloud.langfuse.com")
	}
	if got := parseLangfuseBaseURL("https://example.com"); got != "https://example.com" {
		t.Fatalf("custom base URL = %q, want %q", got, "https://example.com")
	}
}

func TestParseConsoleUsageReporter(t *testing.T) {
	trueValues := []string{"", "true", "1", "yes", "True", "TRUE", "Yes", "YES"}
	for _, input := range trueValues {
		if got := parseConsoleUsageReporter(input); !got {
			t.Fatalf("parseConsoleUsageReporter(%q) = false, want true", input)
		}
	}

	falseValues := []string{"false", "0", "no", "False", "FALSE", "No", "NO"}
	for _, input := range falseValues {
		if got := parseConsoleUsageReporter(input); got {
			t.Fatalf("parseConsoleUsageReporter(%q) = true, want false", input)
		}
	}

	if got := parseConsoleUsageReporter("invalid"); !got {
		t.Fatalf("parseConsoleUsageReporter invalid = false, want true")
	}
}

func TestParseCORSValue(t *testing.T) {
	if got := parseCORSValue("", "*"); got != "*" {
		t.Fatalf("parseCORSValue empty = %q, want %q", got, "*")
	}
	if got := parseCORSValue("https://site.example", "*"); got != "https://site.example" {
		t.Fatalf("parseCORSValue set = %q, want %q", got, "https://site.example")
	}
}

func TestGetRequiredKeyValue(t *testing.T) {
	const key = "TEST_REQUIRED_ENV"
	t.Setenv(key, "value-1")

	got, err := GetRequiredKeyValue(key)
	if err != nil {
		t.Fatalf("GetRequiredKeyValue unexpected error: %v", err)
	}
	if got != "value-1" {
		t.Fatalf("GetRequiredKeyValue = %q, want %q", got, "value-1")
	}

	const missingKey = "TEST_REQUIRED_ENV_MISSING"
	_, err = GetRequiredKeyValue(missingKey)
	if err == nil {
		t.Fatal("GetRequiredKeyValue missing expected error, got nil")
	}
	if !errors.Is(err, ErrMissingEnvVar) {
		t.Fatalf("GetRequiredKeyValue error = %v, want ErrMissingEnvVar", err)
	}
}

func TestGetKeyValue(t *testing.T) {
	const key = "TEST_OPTIONAL_ENV"
	t.Setenv(key, "present")

	if got := GetKeyValue(key); got != "present" {
		t.Fatalf("GetKeyValue(%q) = %q, want %q", key, got, "present")
	}

	const missingKey = "TEST_OPTIONAL_ENV_MISSING"
	if got := GetKeyValue(missingKey); got != "" {
		t.Fatalf("GetKeyValue(%q) = %q, want empty", missingKey, got)
	}
}

func TestLoadReadsAndParsesEnvironment(t *testing.T) {
	t.Setenv("TOOLS_MAX_CONCURRENT", "7")
	t.Setenv("CONTEXT_FULL_TURNS", "2")
	t.Setenv("LANGFUSE_SECRET_KEY", "sk-test")
	t.Setenv("LANGFUSE_PUBLIC_KEY", "pk-test")
	t.Setenv("LANGFUSE_BASE_URL", "https://lf.example")
	t.Setenv("CONSOLE_USAGE_REPORTER", "false")
	t.Setenv("CORS_ALLOW_ORIGIN", "https://app.example")
	t.Setenv("CORS_ALLOW_HEADERS", "X-Test, Content-Type")

	cfg, err := Load("/non/existent/.env")
	if err != nil {
		t.Fatalf("Load unexpected error: %v", err)
	}

	if cfg.ToolsMaxConcurrent != 7 {
		t.Fatalf("ToolsMaxConcurrent = %d, want 7", cfg.ToolsMaxConcurrent)
	}
	if cfg.ContextFullTurns != 2 {
		t.Fatalf("ContextFullTurns = %d, want 2", cfg.ContextFullTurns)
	}
	if cfg.LangfuseSecretKey != "sk-test" || cfg.LangfusePublicKey != "pk-test" {
		t.Fatalf("Langfuse keys mismatch: got secret=%q public=%q", cfg.LangfuseSecretKey, cfg.LangfusePublicKey)
	}
	if cfg.LangfuseBaseURL != "https://lf.example" {
		t.Fatalf("LangfuseBaseURL = %q, want %q", cfg.LangfuseBaseURL, "https://lf.example")
	}
	if cfg.ConsoleUsageReporter {
		t.Fatal("ConsoleUsageReporter = true, want false")
	}
	if cfg.CORSAllowOrigin != "https://app.example" {
		t.Fatalf("CORSAllowOrigin = %q, want %q", cfg.CORSAllowOrigin, "https://app.example")
	}
	if cfg.CORSAllowHeaders != "X-Test, Content-Type" {
		t.Fatalf("CORSAllowHeaders = %q, want %q", cfg.CORSAllowHeaders, "X-Test, Content-Type")
	}
}
