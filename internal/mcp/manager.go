package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// MCPManager manages all MCP client connections and tool operations.
type MCPManager struct {
	ctx     context.Context
	cancel  context.CancelFunc
	clients map[string]*Client // id -> Client
	mu      sync.RWMutex
	config  Config
	logger  *slog.Logger
}

// NewManager creates a new MCP manager instance.
func NewManager(ctx context.Context, cfg Config, logger *slog.Logger) (*MCPManager, error) {
	if logger == nil {
		logger = slog.Default()
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	mgrCtx, cancel := context.WithCancel(ctx)

	m := &MCPManager{
		ctx:     mgrCtx,
		cancel:  cancel,
		clients: make(map[string]*Client),
		config:  cfg,
		logger:  logger,
	}

	// Initialize configured clients
	for i := range cfg.Clients {
		if err := m.AddClient(cfg.Clients[i]); err != nil {
			m.logger.Warn(MCPLogPrefix+" failed to add client",
				"name", cfg.Clients[i].Name,
				"error", err,
			)
			// Continue with other clients
		}
	}

	m.logger.Info(MCPLogPrefix+" manager initialized",
		"clients", len(m.clients),
	)

	return m, nil
}

// ============================================================================
// TOOL OPERATIONS
// ============================================================================

// GetAvailableTools returns all tools from connected MCP clients.
func (m *MCPManager) GetAvailableTools(ctx context.Context) []types.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Extract filtering from context
	includeClients := getIncludeClients(ctx)
	includeTools := getIncludeTools(ctx)

	var tools []types.Tool

	for id, client := range m.clients {
		// Skip disconnected clients
		if client.Conn == nil {
			continue
		}

		// Apply client filtering
		if !m.shouldIncludeClient(id, includeClients) {
			continue
		}

		client.mu.RLock()
		for toolName, tool := range client.Tools {
			// Apply config-level tool filtering
			if m.shouldSkipToolForConfig(toolName, client.Config) {
				continue
			}

			// Apply request-level tool filtering
			if m.shouldSkipToolForRequest(id, toolName, includeTools) {
				continue
			}

			tools = append(tools, tool)
		}
		client.mu.RUnlock()
	}

	return tools
}

// ExecuteToolCall executes a single tool call and returns the result.
func (m *MCPManager) ExecuteToolCall(ctx context.Context, toolCall types.ToolCall) (*ToolExecutionResult, error) {
	if toolCall.Function.Name == "" {
		return nil, fmt.Errorf("tool call missing function name")
	}

	start := time.Now()
	toolName := toolCall.Function.Name

	// Find the client that has this tool
	client := m.findClientForTool(toolName)
	if client == nil {
		return nil, fmt.Errorf("tool %q not found in any MCP client", toolName)
	}

	if client.Conn == nil {
		return nil, fmt.Errorf("client %q not connected", client.Name)
	}

	// Get execution timeout
	timeout := client.Config.GetExecutionTimeout(m.config.DefaultExecutionTimeout)
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute the tool
	result, err := executeToolOnClient(execCtx, client, toolCall)
	if err != nil {
		m.logger.Error(MCPLogPrefix+" tool execution failed",
			"tool", toolName,
			"client", client.Name,
			"error", err,
		)
		return &ToolExecutionResult{
			ToolCallID: toolCall.ID,
			ToolName:   toolName,
			Content:    fmt.Sprintf("Error: %s", err.Error()),
			IsError:    true,
			Duration:   time.Since(start),
		}, nil
	}

	result.Duration = time.Since(start)

	m.logger.Debug(MCPLogPrefix+" tool executed",
		"tool", toolName,
		"client", client.Name,
		"duration_ms", result.Duration.Milliseconds(),
	)

	return result, nil
}

// ExecuteToolCalls executes multiple tool calls concurrently.
func (m *MCPManager) ExecuteToolCalls(ctx context.Context, toolCalls []types.ToolCall) []ToolExecutionResult {
	results := make([]ToolExecutionResult, len(toolCalls))
	var wg sync.WaitGroup

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, call types.ToolCall) {
			defer wg.Done()

			result, err := m.ExecuteToolCall(ctx, call)
			if err != nil {
				results[idx] = ToolExecutionResult{
					ToolCallID: call.ID,
					ToolName:   call.Function.Name,
					Content:    fmt.Sprintf("Error: %s", err.Error()),
					IsError:    true,
				}
			} else if result != nil {
				results[idx] = *result
			}
		}(i, tc)
	}

	wg.Wait()
	return results
}

// InjectTools adds MCP tools to a chat request.
func (m *MCPManager) InjectTools(ctx context.Context, req *types.ChatRequest) {
	mcpTools := m.GetAvailableTools(ctx)
	if len(mcpTools) == 0 {
		return
	}

	// Build map of existing tool names for deduplication
	existing := make(map[string]bool)
	for _, t := range req.Tools {
		existing[t.Function.Name] = true
	}

	// Add MCP tools that don't already exist
	for _, tool := range mcpTools {
		if !existing[tool.Function.Name] {
			req.Tools = append(req.Tools, tool)
			existing[tool.Function.Name] = true
		}
	}
}

// ============================================================================
// CLIENT MANAGEMENT
// ============================================================================

// AddClient adds a new MCP client with the given configuration.
func (m *MCPManager) AddClient(cfg ClientConfig) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	m.mu.Lock()
	if _, exists := m.clients[cfg.ID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("client %q already exists", cfg.ID)
	}

	// Create placeholder entry
	client := &Client{
		ID:     cfg.ID,
		Name:   cfg.Name,
		Config: cfg,
		Tools:  make(map[string]types.Tool),
	}
	m.clients[cfg.ID] = client
	m.mu.Unlock()

	// Connect (without holding lock for network operations)
	if err := m.connectClient(client); err != nil {
		m.mu.Lock()
		delete(m.clients, cfg.ID)
		m.mu.Unlock()
		return fmt.Errorf("failed to connect: %w", err)
	}

	m.logger.Info(MCPLogPrefix+" client added",
		"id", cfg.ID,
		"name", cfg.Name,
		"type", cfg.Type,
	)

	return nil
}

// RemoveClient removes an MCP client by ID.
func (m *MCPManager) RemoveClient(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[id]
	if !exists {
		return fmt.Errorf("client %q not found", id)
	}

	m.cleanupClient(client)
	delete(m.clients, id)

	m.logger.Info(MCPLogPrefix+" client removed", "id", id)
	return nil
}

// ReconnectClient attempts to reconnect a disconnected client.
func (m *MCPManager) ReconnectClient(id string) error {
	m.mu.RLock()
	client, exists := m.clients[id]
	if !exists {
		m.mu.RUnlock()
		return fmt.Errorf("client %q not found", id)
	}
	cfg := client.Config
	m.mu.RUnlock()

	// Cleanup existing connection
	m.mu.Lock()
	m.cleanupClient(client)
	m.mu.Unlock()

	// Reconnect
	if err := m.connectClient(client); err != nil {
		return fmt.Errorf("failed to reconnect: %w", err)
	}

	m.logger.Info(MCPLogPrefix+" client reconnected",
		"id", id,
		"name", cfg.Name,
	)

	return nil
}

// GetClients returns information about all managed clients.
func (m *MCPManager) GetClients() []ClientInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ClientInfo, 0, len(m.clients))

	for _, client := range m.clients {
		info := m.buildClientInfo(client)
		result = append(result, info)
	}

	return result
}

// GetClient returns information about a specific client.
func (m *MCPManager) GetClient(id string) (*ClientInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[id]
	if !exists {
		return nil, fmt.Errorf("client %q not found", id)
	}

	info := m.buildClientInfo(client)
	return &info, nil
}

// Close shuts down the manager and all client connections.
func (m *MCPManager) Close() error {
	m.cancel()

	m.mu.Lock()
	defer m.mu.Unlock()

	for id, client := range m.clients {
		m.cleanupClient(client)
		delete(m.clients, id)
	}

	m.logger.Info(MCPLogPrefix + " manager closed")
	return nil
}

// ============================================================================
// PRIVATE HELPERS
// ============================================================================

func (m *MCPManager) findClientForTool(toolName string) *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, client := range m.clients {
		client.mu.RLock()
		_, exists := client.Tools[toolName]
		client.mu.RUnlock()
		if exists {
			return client
		}
	}
	return nil
}

func (m *MCPManager) shouldIncludeClient(clientID string, includeClients []string) bool {
	// nil = include all (default)
	if includeClients == nil {
		return true
	}
	// [] = include none
	if len(includeClients) == 0 {
		return false
	}
	// ["*"] = include all
	if slices.Contains(includeClients, "*") {
		return true
	}
	// Check specific client
	return slices.Contains(includeClients, clientID)
}

func (m *MCPManager) shouldSkipToolForConfig(toolName string, cfg ClientConfig) bool {
	// nil or empty = no tools exposed (safe default)
	if len(cfg.ToolsToExecute) == 0 {
		return true
	}
	// ["*"] = all tools exposed
	if slices.Contains(cfg.ToolsToExecute, "*") {
		return false
	}
	// Check specific tool
	return !slices.Contains(cfg.ToolsToExecute, toolName)
}

func (m *MCPManager) shouldSkipToolForRequest(clientID, toolName string, includeTools []string) bool {
	// nil = include all (default)
	if includeTools == nil {
		return false
	}
	// [] = include none
	if len(includeTools) == 0 {
		return true
	}

	// Check for wildcard "clientID/*"
	wildcard := fmt.Sprintf("%s/*", clientID)
	if slices.Contains(includeTools, wildcard) {
		return false
	}

	// Check for specific "clientID/toolName"
	fullName := fmt.Sprintf("%s/%s", clientID, toolName)
	return !slices.Contains(includeTools, fullName)
}

func (m *MCPManager) buildClientInfo(client *Client) ClientInfo {
	state := StateDisconnected
	if client.Conn != nil {
		state = StateConnected
	}

	client.mu.RLock()
	toolNames := make([]string, 0, len(client.Tools))
	for name := range client.Tools {
		toolNames = append(toolNames, name)
	}
	toolCount := len(client.Tools)
	client.mu.RUnlock()

	info := ClientInfo{
		ID:        client.ID,
		Name:      client.Name,
		Type:      client.Config.Type,
		State:     state,
		Tools:     toolNames,
		ToolCount: toolCount,
	}

	if !client.ConnectionInfo.ConnectedAt.IsZero() {
		info.ConnectedAt = &client.ConnectionInfo.ConnectedAt
	}

	return info
}

func (m *MCPManager) cleanupClient(client *Client) {
	if client.cancelFunc != nil {
		client.cancelFunc()
		client.cancelFunc = nil
	}

	if client.Conn != nil {
		if err := client.Conn.Close(); err != nil {
			m.logger.Warn(MCPLogPrefix+" failed to close client connection",
				"name", client.Name,
				"error", err,
			)
		}
		client.Conn = nil
	}

	client.mu.Lock()
	client.Tools = make(map[string]types.Tool)
	client.mu.Unlock()
}

// ============================================================================
// CONTEXT HELPERS
// ============================================================================

func getIncludeClients(ctx context.Context) []string {
	if v, ok := ctx.Value(ContextKeyIncludeClients).([]string); ok {
		return v
	}
	return nil
}

func getIncludeTools(ctx context.Context) []string {
	if v, ok := ctx.Value(ContextKeyIncludeTools).([]string); ok {
		return v
	}
	return nil
}

// WithIncludeClients returns a context with client filtering.
func WithIncludeClients(ctx context.Context, clients []string) context.Context {
	return context.WithValue(ctx, ContextKeyIncludeClients, clients)
}

// WithIncludeTools returns a context with tool filtering.
func WithIncludeTools(ctx context.Context, tools []string) context.Context {
	return context.WithValue(ctx, ContextKeyIncludeTools, tools)
}

// WithManager returns a context with the MCP manager.
func WithManager(ctx context.Context, m Manager) context.Context {
	return context.WithValue(ctx, ContextKeyManager, m)
}

// GetManager retrieves the MCP manager from context.
func GetManager(ctx context.Context) Manager {
	if m, ok := ctx.Value(ContextKeyManager).(Manager); ok {
		return m
	}
	return nil
}
