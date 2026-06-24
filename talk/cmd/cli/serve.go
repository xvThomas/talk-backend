package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/xvThomas/talk-backend/talk-libs/logger"
	"github.com/xvThomas/talk-backend/talk/internal/agui"
	"github.com/xvThomas/talk-backend/talk/internal/config"
)

const defaultServePort = "8090"

func newServeCmd() *cobra.Command {
	var portFlag string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the AG-UI protocol HTTP server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			port := resolvePort(portFlag)
			return runServe(cmd.Context(), port)
		},
	}

	cmd.Flags().StringVar(&portFlag, "port", "", "HTTP server port (default: 8090, env: SERVE_PORT)")

	return cmd
}

func resolvePort(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := os.Getenv("SERVE_PORT"); env != "" {
		return env
	}
	return defaultServePort
}

func runServe(ctx context.Context, port string) error {
	log := logger.Logger
	if log == nil {
		log = slog.Default()
	}

	cfg, err := config.Load(".env")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()

	aguiHandler := agui.NewHandler(log, nil)
	mux.Handle("POST /agent", aguiHandler)
	mux.HandleFunc("GET /agent", methodNotAllowed)
	mux.HandleFunc("PUT /agent", methodNotAllowed)
	mux.HandleFunc("DELETE /agent", methodNotAllowed)

	handler := corsMiddleware(mux, cfg)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      0, // SSE requires no write timeout
	}

	// Graceful shutdown on SIGTERM/SIGINT.
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		log.Info("shutting down server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("server shutdown error", slog.String("error", err.Error()))
		}
	}()

	log.Info("starting AG-UI server", slog.String("port", port), slog.String("addr", ":"+port))

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	wg.Wait()
	log.Info("server stopped")
	return nil
}

func corsMiddleware(next http.Handler, cfg *config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", cfg.CORSAllowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", cfg.CORSAllowHeaders)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func methodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
}
