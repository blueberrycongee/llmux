package mcp

import (
	"time"

	"github.com/blueberrycongee/llmux/internal/config"
)

// FromConfig converts a config.MCPConfig to mcp.Config.
func FromConfig(cfg config.MCPConfig) Config {
	clients := make([]ClientConfig, len(cfg.Clients))

	for i := range cfg.Clients {
		c := &cfg.Clients[i]
		clients[i] = ClientConfig{
			ID:                c.ID,
			Name:              c.Name,
			Type:              ConnectionType(c.Type),
			URL:               c.URL,
			Command:           c.Command,
			Args:              c.Args,
			Envs:              c.Envs,
			Headers:           c.Headers,
			ToolsToExecute:    c.ToolsToExecute,
			ConnectionTimeout: c.ConnectionTimeout,
			ExecutionTimeout:  c.ExecutionTimeout,
		}
	}

	connTimeout := cfg.DefaultConnectionTimeout
	if connTimeout == 0 {
		connTimeout = 30 * time.Second
	}

	execTimeout := cfg.DefaultExecutionTimeout
	if execTimeout == 0 {
		execTimeout = 60 * time.Second
	}

	return Config{
		Enabled:                  cfg.Enabled,
		Clients:                  clients,
		DefaultConnectionTimeout: connTimeout,
		DefaultExecutionTimeout:  execTimeout,
	}
}
