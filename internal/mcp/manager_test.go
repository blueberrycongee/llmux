package mcp

import (
	"context"
	"testing"

	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestManagerFiltering(t *testing.T) {
	m := &MCPManager{
		clients: make(map[string]*Client),
	}

	t.Run("shouldIncludeClient", func(t *testing.T) {
		tests := []struct {
			name           string
			clientID       string
			includeClients []string
			want           bool
		}{
			{"nil includes all", "client1", nil, true},
			{"empty excludes all", "client1", []string{}, false},
			{"wildcard includes all", "client1", []string{"*"}, true},
			{"specific match", "client1", []string{"client1", "client2"}, true},
			{"specific no match", "client3", []string{"client1", "client2"}, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := m.shouldIncludeClient(tt.clientID, tt.includeClients)
				if got != tt.want {
					t.Errorf("shouldIncludeClient(%q, %v) = %v, want %v",
						tt.clientID, tt.includeClients, got, tt.want)
				}
			})
		}
	})

	t.Run("shouldSkipToolForConfig", func(t *testing.T) {
		tests := []struct {
			name     string
			toolName string
			cfg      ClientConfig
			want     bool
		}{
			{"nil skips all", "tool1", ClientConfig{ToolsToExecute: nil}, true},
			{"empty skips all", "tool1", ClientConfig{ToolsToExecute: []string{}}, true},
			{"wildcard includes all", "tool1", ClientConfig{ToolsToExecute: []string{"*"}}, false},
			{"specific match", "tool1", ClientConfig{ToolsToExecute: []string{"tool1", "tool2"}}, false},
			{"specific no match", "tool3", ClientConfig{ToolsToExecute: []string{"tool1", "tool2"}}, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := m.shouldSkipToolForConfig(tt.toolName, tt.cfg)
				if got != tt.want {
					t.Errorf("shouldSkipToolForConfig(%q, %v) = %v, want %v",
						tt.toolName, tt.cfg.ToolsToExecute, got, tt.want)
				}
			})
		}
	})

	t.Run("shouldSkipToolForRequest", func(t *testing.T) {
		tests := []struct {
			name         string
			clientID     string
			toolName     string
			includeTools []string
			want         bool
		}{
			{"nil includes all", "client1", "tool1", nil, false},
			{"empty excludes all", "client1", "tool1", []string{}, true},
			{"wildcard for client", "client1", "tool1", []string{"client1/*"}, false},
			{"specific match", "client1", "tool1", []string{"client1/tool1"}, false},
			{"specific no match", "client1", "tool2", []string{"client1/tool1"}, true},
			{"wrong client", "client2", "tool1", []string{"client1/tool1"}, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := m.shouldSkipToolForRequest(tt.clientID, tt.toolName, tt.includeTools)
				if got != tt.want {
					t.Errorf("shouldSkipToolForRequest(%q, %q, %v) = %v, want %v",
						tt.clientID, tt.toolName, tt.includeTools, got, tt.want)
				}
			})
		}
	})
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()

	t.Run("WithIncludeClients", func(t *testing.T) {
		clients := []string{"client1", "client2"}
		ctxWithClients := WithIncludeClients(ctx, clients)

		got := getIncludeClients(ctxWithClients)
		if len(got) != len(clients) {
			t.Errorf("getIncludeClients() = %v, want %v", got, clients)
		}
	})

	t.Run("WithIncludeTools", func(t *testing.T) {
		tools := []string{"client1/tool1", "client2/*"}
		ctxWithTools := WithIncludeTools(ctx, tools)

		got := getIncludeTools(ctxWithTools)
		if len(got) != len(tools) {
			t.Errorf("getIncludeTools() = %v, want %v", got, tools)
		}
	})

	t.Run("nil context values", func(t *testing.T) {
		if got := getIncludeClients(ctx); got != nil {
			t.Errorf("getIncludeClients() = %v, want nil", got)
		}
		if got := getIncludeTools(ctx); got != nil {
			t.Errorf("getIncludeTools() = %v, want nil", got)
		}
	})
}

func TestBuildClientInfo(t *testing.T) {
	m := &MCPManager{
		clients: make(map[string]*Client),
	}

	client := &Client{
		ID:   "test-id",
		Name: "Test Client",
		Config: ClientConfig{
			Type: ConnectionTypeHTTP,
		},
		Tools: map[string]types.Tool{
			"tool1": {Function: types.ToolFunction{Name: "tool1"}},
			"tool2": {Function: types.ToolFunction{Name: "tool2"}},
		},
		Conn: nil, // Disconnected
	}

	info := m.buildClientInfo(client)

	if info.ID != "test-id" {
		t.Errorf("ID = %q, want %q", info.ID, "test-id")
	}
	if info.Name != "Test Client" {
		t.Errorf("Name = %q, want %q", info.Name, "Test Client")
	}
	if info.Type != ConnectionTypeHTTP {
		t.Errorf("Type = %q, want %q", info.Type, ConnectionTypeHTTP)
	}
	if info.State != StateDisconnected {
		t.Errorf("State = %q, want %q", info.State, StateDisconnected)
	}
	if info.ToolCount != 2 {
		t.Errorf("ToolCount = %d, want %d", info.ToolCount, 2)
	}
}
