package mcp

import (
	"context"
	"fmt"
	"sync"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// MockManager is a mock implementation of Manager for testing.
type MockManager struct {
	mu      sync.RWMutex
	clients map[string]*MockClient
	tools   []types.Tool

	// Hooks for customizing behavior
	ExecuteFunc func(ctx context.Context, toolCall types.ToolCall) (*ToolExecutionResult, error)
}

// MockClient represents a mock MCP client.
type MockClient struct {
	ID    string
	Name  string
	Type  ConnectionType
	Tools map[string]types.Tool
	State ConnectionState
}

// NewMockManager creates a new mock manager for testing.
func NewMockManager() *MockManager {
	return &MockManager{
		clients: make(map[string]*MockClient),
		tools:   []types.Tool{},
	}
}

// AddMockClient adds a mock client with the given tools.
func (m *MockManager) AddMockClient(id, name string, connType ConnectionType, tools []types.Tool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	toolMap := make(map[string]types.Tool)
	for _, t := range tools {
		toolMap[t.Function.Name] = t
		m.tools = append(m.tools, t)
	}

	m.clients[id] = &MockClient{
		ID:    id,
		Name:  name,
		Type:  connType,
		Tools: toolMap,
		State: StateConnected,
	}
}

// SetExecuteFunc sets a custom function for tool execution.
func (m *MockManager) SetExecuteFunc(fn func(ctx context.Context, toolCall types.ToolCall) (*ToolExecutionResult, error)) {
	m.ExecuteFunc = fn
}

// ========== Manager Interface Implementation ==========

func (m *MockManager) GetAvailableTools(ctx context.Context) []types.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tools
}

func (m *MockManager) ExecuteToolCall(ctx context.Context, toolCall types.ToolCall) (*ToolExecutionResult, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, toolCall)
	}

	// Default behavior: return success
	return &ToolExecutionResult{
		ToolCallID: toolCall.ID,
		ToolName:   toolCall.Function.Name,
		Content:    fmt.Sprintf("Mock result for %s", toolCall.Function.Name),
		IsError:    false,
	}, nil
}

func (m *MockManager) ExecuteToolCalls(ctx context.Context, toolCalls []types.ToolCall) []ToolExecutionResult {
	results := make([]ToolExecutionResult, len(toolCalls))

	for i, tc := range toolCalls {
		result, err := m.ExecuteToolCall(ctx, tc)
		if err != nil {
			results[i] = ToolExecutionResult{
				ToolCallID: tc.ID,
				ToolName:   tc.Function.Name,
				Content:    fmt.Sprintf("Error: %s", err.Error()),
				IsError:    true,
			}
		} else {
			results[i] = *result
		}
	}

	return results
}

func (m *MockManager) AddClient(cfg ClientConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[cfg.ID]; exists {
		return fmt.Errorf("client %q already exists", cfg.ID)
	}

	m.clients[cfg.ID] = &MockClient{
		ID:    cfg.ID,
		Name:  cfg.Name,
		Type:  cfg.Type,
		Tools: make(map[string]types.Tool),
		State: StateConnected,
	}

	return nil
}

func (m *MockManager) RemoveClient(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[id]; !exists {
		return fmt.Errorf("client %q not found", id)
	}

	delete(m.clients, id)
	return nil
}

func (m *MockManager) ReconnectClient(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[id]
	if !exists {
		return fmt.Errorf("client %q not found", id)
	}

	client.State = StateConnected
	return nil
}

func (m *MockManager) GetClients() []ClientInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ClientInfo, 0, len(m.clients))
	for _, c := range m.clients {
		toolNames := make([]string, 0, len(c.Tools))
		for name := range c.Tools {
			toolNames = append(toolNames, name)
		}

		result = append(result, ClientInfo{
			ID:        c.ID,
			Name:      c.Name,
			Type:      c.Type,
			State:     c.State,
			Tools:     toolNames,
			ToolCount: len(c.Tools),
		})
	}

	return result
}

func (m *MockManager) GetClient(id string) (*ClientInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, exists := m.clients[id]
	if !exists {
		return nil, fmt.Errorf("client %q not found", id)
	}

	toolNames := make([]string, 0, len(c.Tools))
	for name := range c.Tools {
		toolNames = append(toolNames, name)
	}

	return &ClientInfo{
		ID:        c.ID,
		Name:      c.Name,
		Type:      c.Type,
		State:     c.State,
		Tools:     toolNames,
		ToolCount: len(c.Tools),
	}, nil
}

func (m *MockManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.clients = make(map[string]*MockClient)
	m.tools = []types.Tool{}
	return nil
}

// InjectTools implements ToolInjector for MockManager.
func (m *MockManager) InjectTools(ctx context.Context, req *types.ChatRequest) {
	tools := m.GetAvailableTools(ctx)
	if len(tools) == 0 {
		return
	}

	existing := make(map[string]bool)
	for _, t := range req.Tools {
		existing[t.Function.Name] = true
	}

	for _, tool := range tools {
		if !existing[tool.Function.Name] {
			req.Tools = append(req.Tools, tool)
		}
	}
}

// Ensure MockManager implements Manager interface.
var _ Manager = (*MockManager)(nil)
