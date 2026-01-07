package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// connectClient establishes a connection to an MCP server.
func (m *MCPManager) connectClient(c *Client) error {
	var mcpClient *client.Client
	var connInfo ClientConnectionInfo
	var err error

	cfg := c.Config

	switch cfg.Type {
	case ConnectionTypeHTTP:
		mcpClient, connInfo, err = m.createHTTPConnection(cfg)
	case ConnectionTypeSTDIO:
		mcpClient, connInfo, err = m.createSTDIOConnection(cfg)
	case ConnectionTypeSSE:
		mcpClient, connInfo, c.cancelFunc, err = m.createSSEConnection(cfg)
	default:
		return fmt.Errorf("unsupported connection type: %s", cfg.Type)
	}

	if err != nil {
		return err
	}

	// Initialize connection with timeout
	timeout := cfg.GetConnectionTimeout(m.config.DefaultConnectionTimeout)
	ctx, cancel := context.WithTimeout(m.ctx, timeout)
	defer cancel()

	// Start the transport
	if err := mcpClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start transport: %w", err)
	}

	// Initialize MCP protocol
	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    MCPClientName,
				Version: MCPVersion,
			},
		},
	}

	if _, err := mcpClient.Initialize(ctx, initReq); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// Update client with connection
	c.Conn = mcpClient
	connInfo.ConnectedAt = time.Now()
	c.ConnectionInfo = connInfo

	// Retrieve tools
	if err := m.refreshClientTools(ctx, c); err != nil {
		m.logger.Warn(MCPLogPrefix+" failed to retrieve tools",
			"client", c.Name,
			"error", err,
		)
		// Continue even if tool retrieval fails
	}

	return nil
}

func (m *MCPManager) createHTTPConnection(cfg ClientConfig) (*client.Client, ClientConnectionInfo, error) {
	connInfo := ClientConnectionInfo{
		Type: ConnectionTypeHTTP,
		URL:  cfg.URL,
	}

	httpTransport, err := transport.NewStreamableHTTP(
		cfg.URL,
		transport.WithHTTPHeaders(cfg.Headers),
	)
	if err != nil {
		return nil, connInfo, fmt.Errorf("failed to create HTTP transport: %w", err)
	}

	return client.NewClient(httpTransport), connInfo, nil
}

func (m *MCPManager) createSTDIOConnection(cfg ClientConfig) (*client.Client, ClientConnectionInfo, error) {
	cmdString := fmt.Sprintf("%s %s", cfg.Command, strings.Join(cfg.Args, " "))
	connInfo := ClientConnectionInfo{
		Type:          ConnectionTypeSTDIO,
		CommandString: cmdString,
	}

	// Verify required environment variables
	for _, env := range cfg.Envs {
		if os.Getenv(env) == "" {
			return nil, connInfo, fmt.Errorf("environment variable %s not set", env)
		}
	}

	stdioTransport := transport.NewStdio(
		cfg.Command,
		cfg.Envs,
		cfg.Args...,
	)

	return client.NewClient(stdioTransport), connInfo, nil
}

func (m *MCPManager) createSSEConnection(cfg ClientConfig) (*client.Client, ClientConnectionInfo, context.CancelFunc, error) {
	connInfo := ClientConnectionInfo{
		Type: ConnectionTypeSSE,
		URL:  cfg.URL,
	}

	sseTransport, err := transport.NewSSE(
		cfg.URL,
		transport.WithHeaders(cfg.Headers),
	)
	if err != nil {
		return nil, connInfo, nil, fmt.Errorf("failed to create SSE transport: %w", err)
	}

	// SSE needs a long-lived context
	ctx, cancel := context.WithCancel(m.ctx)
	_ = ctx // SSE transport manages its own context

	return client.NewClient(sseTransport), connInfo, cancel, nil
}

// refreshClientTools retrieves and stores tools from an MCP client.
func (m *MCPManager) refreshClientTools(ctx context.Context, c *Client) error {
	if c.Conn == nil {
		return fmt.Errorf("client not connected")
	}

	listReq := mcp.ListToolsRequest{
		PaginatedRequest: mcp.PaginatedRequest{
			Request: mcp.Request{
				Method: string(mcp.MethodToolsList),
			},
		},
	}

	resp, err := c.Conn.ListTools(ctx, listReq)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.Tools = make(map[string]types.Tool)

	for i := range resp.Tools {
		tool := convertMCPTool(&resp.Tools[i])
		c.Tools[resp.Tools[i].Name] = tool
	}

	m.logger.Debug(MCPLogPrefix+" tools refreshed",
		"client", c.Name,
		"count", len(c.Tools),
	)

	return nil
}
