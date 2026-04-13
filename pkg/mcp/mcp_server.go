package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"talks/internal/domain"
	"time"

	"github.com/gorilla/mux"
)

type McpServer struct {
	port       int
	tools      []domain.Tool
	mcpRouter  McpRpcRouter
	mcpHandler McpServerHandler
}

func NewMcpServer(name string, version string, port int, tools []domain.Tool, allowedOrigins []string) *McpServer {
	mcpRouter := NewMcpRpcRouter(name, version, tools)
	mcpHandler := NewMcpServerHandler(mcpRouter, port, allowedOrigins)

	return &McpServer{port: port, tools: tools, mcpRouter: mcpRouter, mcpHandler: mcpHandler}
}

func (s *McpServer) Start() {
	go s.startHTTP()

	fi, _ := os.Stdin.Stat()
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		// stdin is a pipe → read JSON-RPC from stdin
		s.readStdio()
	} else {
		// no pipe → wait for interrupt signal
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		log.Println("Shutting down...")
	}
}

func (s *McpServer) readStdio() {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Printf("stdio read error: %v", err)
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.writeError(nil, -32700, fmt.Sprintf("parse error: %v", err))
			continue
		}

		// Notifications have no ID; route them for side-effects but do not write a response.
		if req.ID == nil {
			s.mcpRouter.RouteRequest(context.Background(), req)
			continue
		}

		resp := s.mcpRouter.RouteRequest(context.Background(), req)
		b, _ := json.Marshal(resp)
		fmt.Println(string(b))
	}
}

func (s *McpServer) startHTTP() {
	router := mux.NewRouter().PathPrefix("").Subrouter()

	s.mcpHandler.RegisterRoutes(router)

	srv := &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler:      router,
		ReadTimeout:  15 * time.Second, // prevent slowloris attacks
		WriteTimeout: 15 * time.Second, // prevent slowloris attacks
		IdleTimeout:  60 * time.Second, // allow time for clients to finish requests
	}

	log.Printf("HTTP server listening on 127.0.0.1:%d (endpoints: POST /rpc, GET /sse)\n", s.port)
	if err := srv.ListenAndServe(); err != nil {
		log.Printf("HTTP server error: %v", err)
	}
}

// Legacy stdio writers retained for compatibility
func (s *McpServer) writeResponse(id json.RawMessage, result any) {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: result}
	b, _ := json.Marshal(resp)
	fmt.Println(string(b))
}

func (s *McpServer) writeError(id json.RawMessage, code int, message string) {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: id, Error: &JSONRPCError{Code: code, Message: message}}
	b, _ := json.Marshal(resp)
	fmt.Println(string(b))
}
