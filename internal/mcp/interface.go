package mcp

import (
	"context"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// Manager defines the interface for MCP management operations.
// This interface allows for easy mocking in tests and decouples
// the MCP implementation from the rest of the codebase.
type Manager interface {
	// ========== Tool Operations ==========

	// GetAvailableTools returns all tools from connected MCP clients.
	// It applies filtering based on client configuration and request context.
	GetAvailableTools(ctx context.Context) []types.Tool

	// ExecuteToolCall executes a single tool call and returns the result.
	ExecuteToolCall(ctx context.Context, toolCall types.ToolCall) (*ToolExecutionResult, error)

	// ExecuteToolCalls executes multiple tool calls concurrently.
	ExecuteToolCalls(ctx context.Context, toolCalls []types.ToolCall) []ToolExecutionResult

	// ========== Client Management ==========

	// AddClient adds a new MCP client with the given configuration.
	AddClient(cfg ClientConfig) error

	// RemoveClient removes an MCP client by ID.
	RemoveClient(id string) error

	// ReconnectClient attempts to reconnect a disconnected client.
	ReconnectClient(id string) error

	// GetClients returns information about all managed clients.
	GetClients() []ClientInfo

	// GetClient returns information about a specific client.
	GetClient(id string) (*ClientInfo, error)

	// ========== Lifecycle ==========

	// Close shuts down the manager and all client connections.
	Close() error
}

// ToolInjector is an optional interface for injecting MCP tools into requests.
type ToolInjector interface {
	// InjectTools adds MCP tools to a chat request.
	InjectTools(ctx context.Context, req *types.ChatRequest)
}

// Ensure MCPManager implements both interfaces.
var (
	_ Manager      = (*MCPManager)(nil)
	_ ToolInjector = (*MCPManager)(nil)
)
