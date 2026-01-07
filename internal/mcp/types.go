// Package mcp provides Model Context Protocol (MCP) integration for LLMux.
// It enables connecting to external MCP servers and executing tools via the MCP protocol.
package mcp

import (
	"context"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// ============================================================================
// CONSTANTS
// ============================================================================

const (
	// MCPVersion is the version identifier for LLMux MCP client.
	MCPVersion = "1.0.0"

	// MCPClientName is the name used when connecting to MCP servers.
	MCPClientName = "LLMux-MCP-Client"

	// MCPLogPrefix is the consistent logging prefix for MCP operations.
	MCPLogPrefix = "[MCP]"

	// DefaultConnectionTimeout is the default timeout for establishing MCP connections.
	DefaultConnectionTimeout = 30 * time.Second

	// DefaultExecutionTimeout is the default timeout for tool execution.
	DefaultExecutionTimeout = 60 * time.Second

	// MaxToolIterations is the maximum number of tool execution iterations in agentic loop.
	MaxToolIterations = 10
)

// ============================================================================
// CONTEXT KEYS
// ============================================================================

// ContextKey is the type for MCP context keys.
type ContextKey string

const (
	// ContextKeyIncludeClients filters which MCP clients to include.
	// Value: []string - nil=all, []=none, ["*"]=all, ["id1","id2"]=specific
	ContextKeyIncludeClients ContextKey = "mcp-include-clients"

	// ContextKeyIncludeTools filters which tools to include.
	// Value: []string - format: "clientID/toolName" or "clientID/*"
	ContextKeyIncludeTools ContextKey = "mcp-include-tools"

	// ContextKeyManager stores the MCP manager in request context.
	ContextKeyManager ContextKey = "mcp-manager"
)

// ============================================================================
// CONNECTION TYPES
// ============================================================================

// ConnectionType defines the type of MCP connection.
type ConnectionType string

const (
	// ConnectionTypeHTTP uses HTTP/Streamable HTTP transport.
	ConnectionTypeHTTP ConnectionType = "http"

	// ConnectionTypeSTDIO uses standard input/output transport.
	ConnectionTypeSTDIO ConnectionType = "stdio"

	// ConnectionTypeSSE uses Server-Sent Events transport.
	ConnectionTypeSSE ConnectionType = "sse"

	// ConnectionTypeInProcess uses in-process transport for local tools.
	ConnectionTypeInProcess ConnectionType = "inprocess"
)

// ConnectionState represents the state of an MCP client connection.
type ConnectionState string

const (
	// StateConnected indicates the client is connected.
	StateConnected ConnectionState = "connected"

	// StateDisconnected indicates the client is disconnected.
	StateDisconnected ConnectionState = "disconnected"

	// StateError indicates the client encountered an error.
	StateError ConnectionState = "error"
)

// ============================================================================
// CLIENT TYPES
// ============================================================================

// Client represents a connected MCP client with its configuration and tools.
type Client struct {
	ID             string                // Unique identifier
	Name           string                // Human-readable name
	Config         ClientConfig          // Client configuration
	Conn           *client.Client        // Active MCP client connection
	Tools          map[string]types.Tool // Available tools mapped by name
	ConnectionInfo ClientConnectionInfo  // Connection metadata
	cancelFunc     context.CancelFunc    // Cancel function for SSE connections
	mu             sync.RWMutex          // Mutex for thread-safe tool access
}

// ClientConnectionInfo stores metadata about how a client is connected.
type ClientConnectionInfo struct {
	Type          ConnectionType `json:"type"`
	URL           string         `json:"url,omitempty"`
	CommandString string         `json:"command_string,omitempty"`
	ConnectedAt   time.Time      `json:"connected_at,omitempty"`
}

// ClientInfo is the public representation of a client for API responses.
type ClientInfo struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Type        ConnectionType  `json:"type"`
	State       ConnectionState `json:"state"`
	Tools       []string        `json:"tools"`
	ToolCount   int             `json:"tool_count"`
	ConnectedAt *time.Time      `json:"connected_at,omitempty"`
}

// ============================================================================
// TOOL EXECUTION TYPES
// ============================================================================

// ToolExecutionResult represents the result of executing an MCP tool.
type ToolExecutionResult struct {
	ToolCallID string        `json:"tool_call_id"`
	ToolName   string        `json:"tool_name"`
	Content    string        `json:"content"`
	IsError    bool          `json:"is_error,omitempty"`
	Duration   time.Duration `json:"duration_ms,omitempty"`
}

// Reset clears the ToolExecutionResult for reuse.
func (r *ToolExecutionResult) Reset() {
	r.ToolCallID = ""
	r.ToolName = ""
	r.Content = ""
	r.IsError = false
	r.Duration = 0
}
