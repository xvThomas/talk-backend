package mcplegacy

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
	// Protocol: stdin → requests, stdout → responses.
	// Logs and errors must never go to stdout.
	log.SetOutput(os.Stderr)

	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	negotiatedVersion := protoV1 // updated after the first initialize response
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
			s.writeErrorTo(writer, nil, -32700, fmt.Sprintf("parse error: %v", err))
			continue
		}

		// Notifications have no ID; route them for side-effects but do not write a response.
		if req.ID == nil {
			s.mcpRouter.RouteRequest(withProtoVersion(context.Background(), negotiatedVersion), req)
			continue
		}

		ctx := withProtoVersion(context.Background(), negotiatedVersion)
		resp := s.mcpRouter.RouteRequest(ctx, req)
		if req.Method == "initialize" && resp.Error == nil {
			negotiatedVersion = extractProtoVersion(resp.Result)
		}
		b, err := json.Marshal(resp)
		if err != nil {
			log.Printf("stdio marshal error: %v", err)
			continue
		}
		_, _ = fmt.Fprintf(writer, "%s\n", b)
		_ = writer.Flush()
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

// writeErrorTo writes a JSON-RPC error response to the given writer (stdout).
func (s *McpServer) writeErrorTo(w *bufio.Writer, id json.RawMessage, code int, message string) {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: id, Error: &JSONRPCError{Code: code, Message: message}}
	b, _ := json.Marshal(resp)
	_, _ = fmt.Fprintf(w, "%s\n", b)
	_ = w.Flush()
}

// extractProtoVersion parses the protocolVersion field from an initialize result.
// Falls back to protoV1 if absent or unparseable.
func extractProtoVersion(result any) string {
	b, err := json.Marshal(result)
	if err != nil {
		return protoV1
	}
	var r struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(b, &r); err != nil || r.ProtocolVersion == "" {
		return protoV1
	}
	return r.ProtocolVersion
}
