package mcp

import "encoding/json"

// JSONRPCRequest represents a JSON-RPC request object, containing the JSON-RPC version, an optional ID, the method to be called, and optional parameters.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC response object,
// containing the JSON-RPC version, an optional ID,
// the result of the method call, or an error if the call failed.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents the error object in a JSON-RPC response,
// containing an error code and a message describing the error.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
