package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
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
				os.Unsetenv("SERVE_PORT")
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
	handler := corsMiddleware(inner)

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
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- runServe(ctx, "0") // port 0 = random available port
	}()

	// Give the server time to start.
	time.Sleep(100 * time.Millisecond)

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
