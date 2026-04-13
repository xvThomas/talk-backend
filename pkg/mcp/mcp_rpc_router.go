package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"talks/internal/domain"
)

// Supported MCP protocol versions, in ascending order of preference.
const (
	protoV1 = "2024-11-05"
	protoV2 = "2025-03-26"
)

var supportedVersions = []string{protoV1, protoV2}

type McpRpcRouter interface {
	RouteRequest(ctx context.Context, req JSONRPCRequest) JSONRPCResponse
}

type mcpRpcRouter struct {
	Name    string
	Version string
	tools   []domain.Tool
}

var _ McpRpcRouter = (*mcpRpcRouter)(nil)

func NewMcpRpcRouter(name, version string, tools []domain.Tool) McpRpcRouter {
	return &mcpRpcRouter{Name: name, Version: version, tools: tools}
}

func (r *mcpRpcRouter) RouteRequest(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = r.initialize(req.Params)
	case "tools/list":
		res, err := r.toolsList()
		if err != nil {
			resp.Error = &JSONRPCError{Code: -32603, Message: fmt.Sprintf("failed to list tools: %v", err)}
		} else {
			resp.Result = res
		}
	case "tools/call":
		res, err := r.toolsCall(ctx, req.Params)
		if err != nil {
			resp.Error = err
		} else {
			resp.Result = res
		}
	case "notifications/initialized", "notifications/cancelled":
		// Notifications are one-way; no response needed.
	default:
		resp.Error = &JSONRPCError{Code: -32601, Message: "method not found"}
	}
	return resp
}

type initializeResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
	Capabilities struct {
		Tools map[string]any `json:"tools"`
	} `json:"capabilities"`
}

type initializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
}

// negotiate returns the highest version the server supports that is <= the client's version.
// Falls back to protoV1 if the client version is unknown.
func negotiate(clientVersion string) string {
	for i := len(supportedVersions) - 1; i >= 0; i-- {
		if supportedVersions[i] <= clientVersion {
			return supportedVersions[i]
		}
	}
	return protoV1
}

func (r *mcpRpcRouter) initialize(raw json.RawMessage) initializeResult {
	var params initializeParams
	_ = json.Unmarshal(raw, &params) // best-effort; missing fields use zero values

	var res initializeResult
	res.ProtocolVersion = negotiate(params.ProtocolVersion)
	res.ServerInfo.Name = r.Name
	res.ServerInfo.Version = r.Version
	res.Capabilities.Tools = map[string]any{}
	return res
}

type toolDescription struct {
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
}

type toolsListResult struct {
	Tools []toolDescription `json:"tools"`
}

func (r *mcpRpcRouter) toolsList() (toolsListResult, error) {
	tools := make([]toolDescription, 0, len(r.tools))
	for _, t := range r.tools {
		inputSchema, err := t.InputSchema()
		if err != nil {
			return toolsListResult{}, fmt.Errorf("Unable to get InputSchema for tool %s: %w", t.Name(), err)
		}
		outputSchema, err := t.OutputSchema()
		if err != nil {
			return toolsListResult{}, fmt.Errorf("Unable to get OutputSchema for tool %s: %w", t.Name(), err)
		}
		rpcTool := toolDescription{
			Name:         t.Name(),
			Description:  t.Description(),
			InputSchema:  inputSchema,
			OutputSchema: outputSchema,
		}
		tools = append(tools, rpcTool)
	}
	return toolsListResult{Tools: tools}, nil
}

type toolsCallParams struct {
	Name string         `json:"name"`
	Args map[string]any `json:"arguments"`
}

type toolsCallResult struct {
	Content any `json:"content"`
}

func (r *mcpRpcRouter) toolsCall(ctx context.Context, raw json.RawMessage) (toolsCallResult, *JSONRPCError) {
	var params toolsCallParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return toolsCallResult{}, &JSONRPCError{Code: -32602, Message: fmt.Sprintf("invalid params: %v", err)}
	}
	var tool domain.Tool
	for _, t := range r.tools {
		if t.Name() == params.Name {
			tool = t
			break
		}
	}
	if tool == nil {
		return toolsCallResult{}, &JSONRPCError{Code: -32601, Message: "tool not found"}
	}

	output, err := tool.Execute(ctx, params.Args)
	if err != nil {
		return toolsCallResult{}, &JSONRPCError{Code: -32603, Message: fmt.Sprintf("tool execution failed: %v", err)}
	}
	return toolsCallResult{Content: output}, nil
}
