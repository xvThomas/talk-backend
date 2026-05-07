package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/mux"
)

type McpServerHandler interface {
	RegisterRoutes(router *mux.Router)
	PostRPC(w http.ResponseWriter, r *http.Request)
	GetSSE(w http.ResponseWriter, r *http.Request)
}

type sseClient struct {
	ch   chan []byte
	quit chan struct{}
}

type mcpServerHandler struct {
	rpcRouter      McpRpcRouter
	port           int
	sseClients     sync.Map // map[string]*sseClient  (HTTP+SSE 2024-11-05)
	sessions       sessionStore                       // Streamable HTTP 2025-03-26
	allowedOrigins []string
}

// NewMcpServerHandler creates an HTTP handler supporting:
//   - HTTP+SSE (MCP 2024-11-05): POST /rpc, GET /sse
//   - Streamable HTTP (MCP 2025-03-26): POST/GET/DELETE /mcp
//
// allowedOrigins is the list of permitted Origin header values for browser
// clients. Non-browser clients (no Origin header) are always allowed.
// Pass nil or an empty slice to allow all browser origins (not recommended
// for production).
func NewMcpServerHandler(rpcRouter McpRpcRouter, port int, allowedOrigins []string) McpServerHandler {
	if rpcRouter == nil {
		panic("rpcRouter cannot be nil")
	}
	return &mcpServerHandler{
		rpcRouter:      rpcRouter,
		port:           port,
		allowedOrigins: allowedOrigins,
	}
}

func (h *mcpServerHandler) RegisterRoutes(router *mux.Router) {
	// HTTP+SSE (2024-11-05)
	router.HandleFunc("/rpc", h.PostRPC).Methods(http.MethodPost)
	router.HandleFunc("/sse", h.GetSSE).Methods(http.MethodGet)

	// Streamable HTTP (2025-03-26)
	router.HandleFunc("/mcp", h.checkOrigin(h.PostMCP)).Methods(http.MethodPost)
	router.HandleFunc("/mcp", h.checkOrigin(h.GetMCP)).Methods(http.MethodGet)
	router.HandleFunc("/mcp", h.checkOrigin(h.DeleteMCP)).Methods(http.MethodDelete)
}

// ── HTTP+SSE (2024-11-05) ────────────────────────────────────────────────────

// PostRPC handles JSON-RPC requests.
// If the request carries a ?session=<id> query param AND that session has an
// active SSE stream, the response is pushed on the SSE channel and 202 is
// returned. Otherwise the response is returned as JSON directly.
func (h *mcpServerHandler) PostRPC(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32700, Message: fmt.Sprintf("parse error: %v", err)},
		})
		return
	}

	resp := h.rpcRouter.RouteRequest(r.Context(), req)

	sessionID := r.URL.Query().Get("session")
	if sessionID != "" {
		if v, ok := h.sseClients.Load(sessionID); ok {
			client := v.(*sseClient)
			b, _ := json.Marshal(resp)
			select {
			case client.ch <- b:
			default:
			}
			w.WriteHeader(http.StatusAccepted)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetSSE opens a persistent SSE stream conforming to MCP 2024-11-05.
// Immediately sends:
//
//	event: endpoint
//	data: http://127.0.0.1:<port>/rpc?session=<uuid>
func (h *mcpServerHandler) GetSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	sessionID := generateID()
	client := &sseClient{ch: make(chan []byte, 16), quit: make(chan struct{})}
	h.sseClients.Store(sessionID, client)
	defer h.sseClients.Delete(sessionID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	endpointURL := fmt.Sprintf("http://127.0.0.1:%d/rpc?session=%s", h.port, sessionID)
	_, _ = fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()

	for {
		select {
		case msg := <-client.ch:
			_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		case <-client.quit:
			return
		}
	}
}

// ── Streamable HTTP (2025-03-26) ─────────────────────────────────────────────

// PostMCP handles POST /mcp per the MCP 2025-03-26 Streamable HTTP spec.
func (h *mcpServerHandler) PostMCP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32700, Message: fmt.Sprintf("parse error: %v", err)},
		})
		return
	}

	// initialize: skip session validation, create new session.
	if req.Method == "initialize" {
		sess := h.sessions.create()

		resp := h.rpcRouter.RouteRequest(r.Context(), req)
		b, _ := json.Marshal(resp)

		w.Header().Set("Mcp-Session-Id", sess.id)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
		return
	}

	// All other methods require a valid MCP-Protocol-Version header.
	protoHeader := r.Header.Get("MCP-Protocol-Version")
	if !isSupportedVersion(protoHeader) {
		http.Error(w, "MCP-Protocol-Version header missing or unsupported", http.StatusBadRequest)
		return
	}

	// Validate session.
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		http.Error(w, "Mcp-Session-Id header required", http.StatusBadRequest)
		return
	}
	sess, ok := h.sessions.get(sessionID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// Notifications (no id) -> 202, no response body.
	if req.ID == nil {
		h.rpcRouter.RouteRequest(withProtoVersion(r.Context(), protoHeader), req)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	resp := h.rpcRouter.RouteRequest(withProtoVersion(r.Context(), protoHeader), req)
	b, _ := json.Marshal(resp)

	// If the client accepts SSE, push via any registered GET /mcp stream.
	// Otherwise fall back to direct JSON response.
	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") && sess.hasConn() {
		sess.push(b)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

// GetMCP opens a persistent SSE stream for server-push notifications
// per MCP 2025-03-26.
func (h *mcpServerHandler) GetMCP(w http.ResponseWriter, r *http.Request) {
	protoHeader := r.Header.Get("MCP-Protocol-Version")
	if !isSupportedVersion(protoHeader) {
		http.Error(w, "MCP-Protocol-Version header missing or unsupported", http.StatusBadRequest)
		return
	}
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		http.Error(w, "Mcp-Session-Id header required", http.StatusBadRequest)
		return
	}
	sess, ok := h.sessions.get(sessionID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ch := make(chan []byte, 16)
	sess.addConn(ch)
	defer sess.removeConn(ch)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return // session was deleted
			}
			_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// DeleteMCP terminates a Streamable HTTP session.
func (h *mcpServerHandler) DeleteMCP(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		http.Error(w, "Mcp-Session-Id header required", http.StatusBadRequest)
		return
	}
	if _, ok := h.sessions.get(sessionID); !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	h.sessions.delete(sessionID)
	w.WriteHeader(http.StatusOK)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// checkOrigin is a middleware that rejects browser requests from non-allowed origins.
// Requests without an Origin header (non-browser clients) are always allowed.
func (h *mcpServerHandler) checkOrigin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next(w, r)
			return
		}
		if len(h.allowedOrigins) == 0 || h.isAllowedOrigin(origin) {
			next(w, r)
			return
		}
		http.Error(w, "Forbidden", http.StatusForbidden)
	}
}

func (h *mcpServerHandler) isAllowedOrigin(origin string) bool {
	for _, o := range h.allowedOrigins {
		if o == origin {
			return true
		}
	}
	return false
}

func isSupportedVersion(v string) bool {
	for _, sv := range supportedVersions {
		if sv == v {
			return true
		}
	}
	return false
}

// generateID returns a cryptographically random 16-byte hex string.
func generateID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
