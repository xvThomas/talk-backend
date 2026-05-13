package mcpserver

import (
	"context"
	"crypto/subtle"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"talks/internal/domain"
	"talks/pkg/logger"
)

// ToolRegistrar registers a tool on an mcp.Server.
// Use RegisterTool to create one from a domain.TypedTool.
type ToolRegistrar func(s *mcp.Server)

// RegisterTool returns a ToolRegistrar that adds the given TypedTool to an mcp.Server.
func RegisterTool[TInput, TOutput any](tool domain.TypedTool[TInput, TOutput]) ToolRegistrar {
	return func(s *mcp.Server) {
		mcp.AddTool(s, &mcp.Tool{
			Name:        tool.Name(),
			Description: tool.Description(),
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args TInput) (*mcp.CallToolResult, TOutput, error) {
			out, err := tool.Call(ctx, args)
			return nil, out, err
		})
	}
}

// App is a reusable MCP server runner that handles CLI flags, transport
// routing (stdio / HTTP), and server creation.
type App struct {
	Name    string
	Version string
	Tools   []ToolRegistrar
	APIKey  string // required for HTTP transport
}

// Run parses CLI flags and starts the server using the selected transport.
func (a *App) Run() {
	log := logger.GetLogger()

	transport := flag.String("transport", "stdio", "transport to use: stdio | http")
	addr := flag.String("addr", "localhost:8080", "address to listen on (HTTP transport)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s --transport stdio\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --transport http --addr localhost:8080\n", os.Args[0])
	}
	flag.Parse()

	log.Info("MCP Server", "name", a.Name, "version", a.Version)

	switch *transport {
	case "stdio":
		a.runStdio()
	case "http":
		if a.APIKey == "" {
			log.Error("X_API_KEY is required for HTTP transport")
			os.Exit(1)
		}
		a.runHTTP(*addr)
	default:
		log.Error("unknown transport", "transport", *transport)
		flag.Usage()
		os.Exit(1)
	}
}

func (a *App) newServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: a.Name, Version: a.Version}, nil)
	for _, register := range a.Tools {
		register(s)
	}
	return s
}

func (a *App) runStdio() {
	log := logger.GetLogger()
	s := a.newServer()
	log.Info("Stdio server running")
	if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Error("stdio server failed", "error", err)
		os.Exit(1)
	}
}

func (a *App) runHTTP(addr string) {
	log := logger.GetLogger()

	serverFactory := func(_ *http.Request) *mcp.Server {
		return a.newServer()
	}

	authMiddleware := apiKeyAuthMiddleware(a.APIKey)

	sseHandler := mcp.NewSSEHandler(serverFactory, nil)
	streamableHandler := mcp.NewStreamableHTTPHandler(serverFactory, nil)

	mux := http.NewServeMux()
	mux.Handle("/sse", authMiddleware(sseHandler))
	mux.Handle("/mcp", authMiddleware(streamableHandler))

	log.Info("HTTP server listening", "addr", addr, "sse", "/sse", "streamable", "/mcp")
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}

// apiKeyAuthMiddleware checks that the X-API-Key header matches the expected key.
func apiKeyAuthMiddleware(expectedKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := r.Header.Get("X-API-Key")
			if subtle.ConstantTimeCompare([]byte(provided), []byte(expectedKey)) != 1 {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
