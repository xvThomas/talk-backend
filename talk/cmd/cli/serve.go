package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	"github.com/spf13/cobra"
	"github.com/xvThomas/talk-backend/talk-libs/logger"
	"github.com/xvThomas/talk-backend/talk/internal/agui"
	"github.com/xvThomas/talk-backend/talk/internal/config"
	"github.com/xvThomas/talk-backend/talk/internal/domain"
	"github.com/xvThomas/talk-backend/talk/internal/llm/router"
	"github.com/xvThomas/talk-backend/talk/internal/mcp"
	sqlitestore "github.com/xvThomas/talk-backend/talk/internal/memory/sqlite"
)

const defaultServePort = "8090"

func newServeCmd() *cobra.Command {
	var (
		portFlag  string
		modelFlag string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the AG-UI protocol HTTP server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			port := resolvePort(portFlag)
			return runServe(cmd.Context(), port, modelFlag)
		},
	}

	cmd.Flags().StringVar(&portFlag, "port", "", "HTTP server port (default: 8090, env: SERVE_PORT)")
	cmd.Flags().StringVar(&modelFlag, "model", "", "Model alias to use (e.g. sonnet-4.6, devstral)")
	_ = cmd.MarkFlagRequired("model")

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

func runServe(ctx context.Context, port, modelAlias string) error {
	log := logger.Logger
	if log == nil {
		log = slog.Default()
	}

	cfg, err := config.Load(".env")
	if err != nil {
		return err
	}

	// Resolve LLM client from model alias.
	r := router.NewLLMRouter(cfg)
	client, err := r.Get(modelAlias)
	if err != nil {
		return fmt.Errorf("resolving model %q: %w", modelAlias, err)
	}

	modelDescriptor, err := domain.Lookup(modelAlias)
	if err != nil {
		return fmt.Errorf("looking up model %q: %w", modelAlias, err)
	}

	pp := buildPromptProvider(defaultSystemPromptPath())

	// Open shared SQLite store.
	dbPath := storeDBPath()
	messages, browser, err := sqlitestore.New(dbPath)
	if err != nil {
		return fmt.Errorf("opening session store: %w", err)
	}
	defer func() { _ = messages.Close() }()

	// MCP server registry and manager.
	mcpRegistry, err := mcp.NewSQLiteRegistry(messages.DB())
	if err != nil {
		return fmt.Errorf("initializing mcp registry: %w", err)
	}
	mcpManager := mcp.NewManager(mcpRegistry)
	mcpManager.ConnectAll(ctx)
	defer mcpManager.Close()

	// ChatFunc creates a per-request ConversationManager and calls Chat.
	chatFn := func(reqCtx context.Context, threadID string, aguiMessages []types.Message) (string, error) {
		scope := domain.NewSessionScope(threadID, "anonymous")
		manager := domain.NewConversationManager(domain.ConversationManagerConfig{
			Client:             client,
			ModelID:            modelAlias,
			Scope:              scope,
			Provider:           modelDescriptor.OLTPProvider,
			Store:              messages,
			SessionBrowser:     browser,
			PromptProvider:     pp,
			Tools:              mcpManager.Tools,
			EventHandlers:      messages,
			MaxConcurrentTools: cfg.ToolsMaxConcurrent,
			ContextFullTurns:   cfg.ContextFullTurns,
		})

		userInput := extractUserInput(aguiMessages)
		if userInput == "" {
			return "", fmt.Errorf("no user message found in request")
		}

		response, chatErr := manager.Chat(reqCtx, userInput)
		if chatErr != nil {
			log.Error("chat error",
				slog.String("threadId", threadID),
				slog.String("error", chatErr.Error()),
			)
			return "", userFacingError(chatErr)
		}
		return response, nil
	}

	mux := http.NewServeMux()

	aguiHandler := agui.NewHandler(log, chatFn)
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

	log.Info("starting AG-UI server",
		slog.String("port", port),
		slog.String("model", modelAlias),
		slog.String("db", dbPath),
	)

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	wg.Wait()
	log.Info("server stopped")
	return nil
}

// extractUserInput returns the content of the last user message from the AG-UI message list.
func extractUserInput(messages []types.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == types.RoleUser {
			if s, ok := messages[i].Content.(string); ok {
				return s
			}
			return fmt.Sprintf("%v", messages[i].Content)
		}
	}
	return ""
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

// userFacingError returns a sanitized error message suitable for end users.
// Technical details are logged separately at ERROR level.
func userFacingError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("request was cancelled")
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("request timed out, please try again")
	case errors.Is(err, config.ErrMissingEnvVar) ||
		errors.Is(err, domain.ErrSystemPrompt):
		return fmt.Errorf("the assistant is not configured correctly, please contact the administrator")
	case errors.As(err, new(*sqlitestore.ErrStore)):
		return fmt.Errorf("service temporarily unavailable, please try again")
	default:
		return fmt.Errorf("an unexpected error occurred, please try again")
	}
}
