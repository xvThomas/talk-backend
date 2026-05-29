package mcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/xvThomas/LLMClientWrapper/talk-libs/version"

	"github.com/xvThomas/LLMClientWrapper/talk/internal/domain"
)

const connectTimeout = 15 * time.Second

// ServerStatus holds the runtime state of a connected MCP server.
type ServerStatus struct {
	Config        ServerConfig
	Connected     bool
	ServerName    string // from Initialize response
	ServerVersion string // from Initialize response
	Tools         []string
	Error         string // non-empty if connection failed
}

// Manager manages connections to registered MCP servers.
type Manager struct {
	registry Registry
	sessions map[string]*mcp.ClientSession // keyed by ServerConfig.ID
	statuses []ServerStatus
	tools    []domain.Tool
}

// NewManager creates a new Manager using the given Registry.
func NewManager(registry Registry) *Manager {
	return &Manager{
		registry: registry,
		sessions: make(map[string]*mcp.ClientSession),
	}
}

// ConnectAll connects to all registered MCP servers.
// Connection errors are recorded per server but do not abort the process.
func (m *Manager) ConnectAll(ctx context.Context) {
	configs, err := m.registry.List(ctx)
	if err != nil {
		m.statuses = []ServerStatus{{Error: fmt.Sprintf("failed to list servers: %v", err)}}
		return
	}

	m.tools = nil
	m.statuses = nil

	for _, cfg := range configs {
		status := ServerStatus{Config: cfg}
		session, err := m.connect(ctx, cfg)
		if err != nil {
			status.Error = err.Error()
			m.statuses = append(m.statuses, status)
			continue
		}
		m.sessions[cfg.ID] = session
		status.Connected = true

		// Retrieve server info from the initialization result.
		if res := session.InitializeResult(); res != nil && res.ServerInfo != nil {
			status.ServerName = res.ServerInfo.Name
			status.ServerVersion = res.ServerInfo.Version
		}

		// List available tools.
		toolsResult, err := session.ListTools(ctx, &mcp.ListToolsParams{})
		if err != nil {
			status.Error = fmt.Sprintf("connected but failed to list tools: %v", err)
			m.statuses = append(m.statuses, status)
			continue
		}

		for _, t := range toolsResult.Tools {
			status.Tools = append(status.Tools, t.Name)
			m.tools = append(m.tools, &mcpToolAdapter{
				serverName: cfg.Name,
				tool:       *t,
				session:    session,
			})
		}
		m.statuses = append(m.statuses, status)
	}
}

// Connect connects to a single MCP server by config. Used for testing a new server on add.
func (m *Manager) Connect(ctx context.Context, cfg ServerConfig) (*ServerStatus, error) {
	status := ServerStatus{Config: cfg}
	session, err := m.connect(ctx, cfg)
	if err != nil {
		status.Error = err.Error()
		return &status, err
	}

	status.Connected = true
	if res := session.InitializeResult(); res != nil && res.ServerInfo != nil {
		status.ServerName = res.ServerInfo.Name
		status.ServerVersion = res.ServerInfo.Version
	}

	toolsResult, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		status.Error = fmt.Sprintf("connected but failed to list tools: %v", err)
		_ = session.Close()
		return &status, nil
	}

	for _, t := range toolsResult.Tools {
		status.Tools = append(status.Tools, t.Name)
		m.tools = append(m.tools, &mcpToolAdapter{
			serverName: cfg.Name,
			tool:       *t,
			session:    session,
		})
	}

	m.sessions[cfg.ID] = session
	m.statuses = append(m.statuses, status)
	return &status, nil
}

// Disconnect closes and removes a specific server connection and its tools.
func (m *Manager) Disconnect(id string) {
	if session, ok := m.sessions[id]; ok {
		_ = session.Close()
		delete(m.sessions, id)
	}
	m.rebuildToolsExcluding(id)
}

// Refresh re-queries the tool list for all connected servers and rebuilds the
// internal tools slice. Returns the number of tools discovered.
func (m *Manager) Refresh(ctx context.Context) int {
	m.tools = nil
	for i := range m.statuses {
		st := &m.statuses[i]
		st.Tools = nil

		session, ok := m.sessions[st.Config.ID]
		if !ok || !st.Connected {
			continue
		}

		toolsResult, err := session.ListTools(ctx, &mcp.ListToolsParams{})
		if err != nil {
			st.Error = fmt.Sprintf("failed to refresh tools: %v", err)
			continue
		}

		st.Error = ""
		for _, t := range toolsResult.Tools {
			st.Tools = append(st.Tools, t.Name)
			m.tools = append(m.tools, &mcpToolAdapter{
				serverName: st.Config.Name,
				tool:       *t,
				session:    session,
			})
		}
	}
	return len(m.tools)
}

func (m *Manager) rebuildToolsExcluding(excludeID string) {
	var filtered []domain.Tool
	for _, t := range m.tools {
		if adapter, ok := t.(*mcpToolAdapter); ok {
			for _, st := range m.statuses {
				if st.Config.ID == excludeID && adapter.serverName == st.Config.Name {
					goto skip
				}
			}
		}
		filtered = append(filtered, t)
	skip:
	}
	m.tools = filtered

	var filteredStatus []ServerStatus
	for _, st := range m.statuses {
		if st.Config.ID != excludeID {
			filteredStatus = append(filteredStatus, st)
		}
	}
	m.statuses = filteredStatus
}

// Tools returns all tools from all connected MCP servers.
func (m *Manager) Tools() []domain.Tool {
	return m.tools
}

// Statuses returns the connection status for all servers.
func (m *Manager) Statuses() []ServerStatus {
	return m.statuses
}

// Close closes all active MCP sessions.
func (m *Manager) Close() {
	for id, session := range m.sessions {
		_ = session.Close()
		delete(m.sessions, id)
	}
}

func (m *Manager) connect(ctx context.Context, cfg ServerConfig) (*mcp.ClientSession, error) {
	connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	httpClient := buildHTTPClient(cfg)
	transport := &mcp.StreamableClientTransport{
		Endpoint:   cfg.URL,
		HTTPClient: httpClient,
	}
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "talk-cli",
		Version: version.Version,
	}, nil)

	session, err := client.Connect(connectCtx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connecting to %q (%s): %w", cfg.Name, cfg.URL, err)
	}
	return session, nil
}

// buildHTTPClient returns an *http.Client configured for the server's auth type.
func buildHTTPClient(cfg ServerConfig) *http.Client {
	switch cfg.AuthType {
	case AuthTypeAPIKey:
		if cfg.APIKey == "" {
			return http.DefaultClient
		}
		return &http.Client{
			Transport: &apiKeyTransport{
				key:  cfg.APIKey,
				base: http.DefaultTransport,
			},
		}
	default:
		return http.DefaultClient
	}
}

// apiKeyTransport injects an X-API-Key header into every request.
type apiKeyTransport struct {
	key  string
	base http.RoundTripper
}

func (t *apiKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("X-API-Key", t.key)
	return t.base.RoundTrip(r)
}
