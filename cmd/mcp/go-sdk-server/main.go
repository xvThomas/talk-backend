// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"talks/internal/infrastructure/config"
	"talks/internal/infrastructure/tools/openweather"
)

var (
	transport = flag.String("transport", "stdio", "transport to use: stdio | sse | streamable")
	addr      = flag.String("addr", "localhost:8080", "address to listen on (HTTP transports)")
)

type serverConfig struct {
	serviceToken string // token du service tiers, fourni par le client
}

type contextKey string

const serviceTokenKey contextKey = "serviceToken"

// serviceTokenMiddleware extrait le token du service tiers depuis le header
// X-Service-Token et l'injecte dans le contexte de la requête.
// Ce header est distinct de toute authentification MCP.
func serviceTokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Service-Token")
		if token == "" {
			http.Error(w, "X-Service-Token header is required", http.StatusBadRequest)
			return
		}
		ctx := context.WithValue(r.Context(), serviceTokenKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newServer(cfg serverConfig) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "weather-mcp", Version: "1.0.0"}, nil)
	weatherTool := openweather.NewCurrentWeatherTool(cfg.serviceToken)
	mcp.AddTool(s, &mcp.Tool{
		Name:        weatherTool.Name(),
		Description: weatherTool.Description(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args openweather.CurrentWeatherToolInput) (*mcp.CallToolResult, openweather.CurrentWeatherToolOutput, error) {
		out, err := weatherTool.Call(ctx, args)
		return nil, out, err
	})
	return s
}

func main() {

	fmt.Printf("Go MCP SDK Server\n")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s --transport stdio\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --transport sse --addr localhost:8080\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --transport streamable --addr localhost:8080\n", os.Args[0])
	}
	flag.Parse()

	switch *transport {
	case "stdio":
		// Priorité : variable d'environnement (VS Code / Claude Desktop / OS)
		// > .env (développement local uniquement).
		// godotenv.Load (appelé dans config.Load) ne remplace jamais une variable
		// déjà présente dans l'environnement.
		cfg, err := config.Load(".env")
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}
		if cfg.OpenWeatherMapAPIKey == "" {
			log.Fatal("OPENWEATHERMAP_API_KEY is required for stdio mode")
		}
		runStdio(serverConfig{serviceToken: cfg.OpenWeatherMapAPIKey})
	case "sse":
		// En HTTP : le client envoie son token via le header X-Service-Token.
		runSSE(*addr)
	case "streamable":
		runStreamable(*addr)
	default:
		fmt.Fprintf(os.Stderr, "unknown transport %q\n", *transport)
		flag.Usage()
		os.Exit(1)
	}
}

func runStdio(cfg serverConfig) {
	// Pour Claude Desktop, VS Code (mode local subprocess) :
	//   configure le serveur avec "command": "/path/to/binary"
	s := newServer(cfg)
	fmt.Println("Stdio server running")
	if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("stdio server failed: %v", err)
	}
}

func runSSE(addr string) {
	// Transport SSE legacy (spec 2024-11-05).
	// Le client fournit son token de service tiers via X-Service-Token.
	handler := mcp.NewSSEHandler(func(r *http.Request) *mcp.Server {
		serviceToken, _ := r.Context().Value(serviceTokenKey).(string)
		return newServer(serverConfig{serviceToken: serviceToken})
	}, nil)

	log.Printf("SSE server listening on http://%s/sse", addr)
	if err := http.ListenAndServe(addr, serviceTokenMiddleware(handler)); err != nil {
		log.Fatalf("SSE server failed: %v", err)
	}
}

func runStreamable(addr string) {
	// Transport Streamable HTTP (spec 2025-06-18).
	// Le client fournit son token de service tiers via X-Service-Token.
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		serviceToken, _ := r.Context().Value(serviceTokenKey).(string)
		return newServer(serverConfig{serviceToken: serviceToken})
	}, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", serviceTokenMiddleware(handler))

	log.Printf("Streamable HTTP server listening on http://%s/mcp", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Streamable server failed: %v", err)
	}
}
