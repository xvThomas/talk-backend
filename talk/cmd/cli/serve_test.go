package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	"github.com/xvThomas/talk-backend/talk/internal/config"
	"github.com/xvThomas/talk-backend/talk/internal/domain"
	sqlitestore "github.com/xvThomas/talk-backend/talk/internal/memory/sqlite"
)

func TestResolvePort(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
		envValue  string
		want      string
	}{
		{
			name:      "flag takes priority",
			flagValue: "9090",
			envValue:  "7070",
			want:      "9090",
		},
		{
			name:      "env used when no flag",
			flagValue: "",
			envValue:  "7070",
			want:      "7070",
		},
		{
			name:      "default when nothing set",
			flagValue: "",
			envValue:  "",
			want:      defaultServePort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("SERVE_PORT", tt.envValue)
			} else {
				t.Setenv("SERVE_PORT", "")
			}

			got := resolvePort(tt.flagValue)
			if got != tt.want {
				t.Errorf("resolvePort(%q) = %q, want %q", tt.flagValue, got, tt.want)
			}
		})
	}
}

func TestCorsMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &config.Config{
		CORSAllowOrigin:  "*",
		CORSAllowHeaders: "Content-Type, Authorization",
	}
	handler := corsMiddleware(inner, cfg)

	t.Run("OPTIONS returns 204 with CORS headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/agent", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("OPTIONS status = %d, want %d", rec.Code, http.StatusNoContent)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "*")
		}
		if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
			t.Error("Access-Control-Allow-Methods header is empty")
		}
		if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
			t.Error("Access-Control-Allow-Headers header is empty")
		}
	})

	t.Run("non-OPTIONS passes through with CORS headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("GET status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "*")
		}
	})
}

func TestRunServeGracefulShutdown(t *testing.T) {
	// Set minimal config for the server to start.
	t.Setenv("ANTHROPIC_API_KEY", "test-key-not-used")

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir) // storeDBPath() uses HOME

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- runServe(ctx, "0") // port 0 = random available port
	}()

	// Give the server time to start.
	time.Sleep(200 * time.Millisecond)

	// Cancel context to trigger graceful shutdown.
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("runServe returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down within timeout")
	}
}

func TestServeCommandRegistered(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"serve"})
	if err != nil {
		t.Fatalf("finding serve command: %v", err)
	}
	if cmd.Use != "serve" {
		t.Errorf("command Use = %q, want %q", cmd.Use, "serve")
	}
}

func TestExtractUserInput(t *testing.T) {
	tests := []struct {
		name     string
		messages []types.Message
		want     string
	}{
		{
			name:     "single user message",
			messages: []types.Message{{Role: types.RoleUser, Content: "hello"}},
			want:     "hello",
		},
		{
			name: "last user message wins",
			messages: []types.Message{
				{Role: types.RoleUser, Content: "first"},
				{Role: types.RoleAssistant, Content: "response"},
				{Role: types.RoleUser, Content: "second"},
			},
			want: "second",
		},
		{
			name:     "no user messages",
			messages: []types.Message{{Role: types.RoleAssistant, Content: "hi"}},
			want:     "",
		},
		{
			name:     "empty list",
			messages: nil,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractUserInput(tt.messages)
			if got != tt.want {
				t.Errorf("extractUserInput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestServeCommandStartsWithoutModelFlag(t *testing.T) {
	cmd := newServeCmd()
	// --model is no longer required; command should parse args without error.
	cmd.SetArgs([]string{"--port", "0"})
	// We only test flag parsing, not execution (which needs config).
	if err := cmd.ParseFlags([]string{"--port", "0"}); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
}

func TestUserFacingError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "context canceled",
			err:  context.Canceled,
			want: "request was cancelled",
		},
		{
			name: "deadline exceeded",
			err:  context.DeadlineExceeded,
			want: "request timed out, please try again",
		},
		{
			name: "missing env var",
			err:  fmt.Errorf("%w %q", config.ErrMissingEnvVar, "ANTHROPIC_API_KEY"),
			want: "the assistant is not configured correctly, please contact the administrator",
		},
		{
			name: "system prompt error",
			err:  fmt.Errorf("%w: file not found", domain.ErrSystemPrompt),
			want: "the assistant is not configured correctly, please contact the administrator",
		},
		{
			name: "store error",
			err:  &sqlitestore.ErrStore{Err: fmt.Errorf("database is locked")},
			want: "service temporarily unavailable, please try again",
		},
		{
			name: "unknown error",
			err:  fmt.Errorf("something unexpected happened"),
			want: "an unexpected error occurred, please try again",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := userFacingError(tt.err)
			if got.Error() != tt.want {
				t.Errorf("userFacingError() = %q, want %q", got.Error(), tt.want)
			}
		})
	}
}
